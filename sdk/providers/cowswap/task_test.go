package cowswap

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func TestTaskRequiresExactConfirmedAuthorization(t *testing.T) {
	for _, mode := range []string{"confirmed", "missing execution", "missing sign", "changed sign", "duplicate sign", "mutable output copy"} {
		t.Run(mode, func(t *testing.T) {
			reader := &cowBatchReader{t: t, filled: big.NewInt(1000)}
			p, err := fixtureProvider(negativeConfig("http://127.0.0.1:1"), provider.Deps{Chain: reader})
			if err != nil {
				t.Fatal(err)
			}
			snapshot := negativeSnapshot()
			snapshot.BlockNumber = big.NewInt(13)
			snapshot.BlockHash = testHeader(snapshot.BlockNumber).Hash()
			order := sampleOrder()
			order.SellAmount = "1000"
			buffer, err := EncodeOrder(order)
			if err != nil {
				t.Fatal(err)
			}
			domain := protocolio.Domain("Gnosis Protocol", "v2", snapshot.ChainID, p.settlement)
			digest := protocolio.Digest(domain, crypto.Keccak256Hash(buffer))
			plan := []account.IMerkleAccountHash{{Offset: new(big.Int), Length: big.NewInt(int64(len(buffer))), Destination: new(big.Int)}}
			binding := account.NewMerkleAccount()
			payload, err := binding.TryPackSign(domain, buffer, plan)
			if err != nil {
				t.Fatal(err)
			}
			event := binding.GetABI().Events["Sign"]
			data, err := event.Inputs.NonIndexed().Pack(buffer, plan)
			if err != nil {
				t.Fatal(err)
			}
			txHash := common.HexToHash("0xaa")
			receiptBlock := big.NewInt(12)
			blockHash := testHeader(receiptBlock).Hash()
			log := types.Log{Address: snapshot.Account, Topics: []common.Hash{event.ID, digest, domain}, Data: data, TxHash: txHash, BlockNumber: 12, BlockHash: blockHash}
			snapshot.Execution = &provider.Execution{TxHash: txHash, BlockNumber: receiptBlock, BlockHash: blockHash, Logs: []types.Log{log}}
			prepared := prepared{ChainID: "1", Account: snapshot.Account, PreparedBlock: "12", Order: order, Digest: digest, UID: UID(digest, snapshot.Account, uint32(order.ValidTo))}
			out := provider.Output{Actions: map[string]hexutil.Bytes{p.cfg.Action: payload}}
			task := p.confirmedTask(out, prepared)
			switch mode {
			case "missing execution":
				snapshot.Execution = nil
			case "missing sign":
				snapshot.Execution.Logs = nil
			case "changed sign":
				snapshot.Execution.Logs[0].Data = []byte{1}
			case "duplicate sign":
				log.Index = 1
				snapshot.Execution.Logs = append(snapshot.Execution.Logs, log)
			case "mutable output copy":
				clear(out.Actions[p.cfg.Action])
			}
			progress, err := task.Advance(t.Context(), snapshot)
			if mode == "confirmed" || mode == "mutable output copy" {
				if err != nil || !progress.Done || progress.Outcome != "settled" {
					t.Fatal("confirmed task did not settle", progress, err)
				}
			} else if err == nil || reader.calls != 0 {
				t.Fatal("invalid authorization reached settlement reads", err, reader.calls)
			}
		})
	}
}
