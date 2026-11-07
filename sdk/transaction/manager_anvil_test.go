//go:build integration

package transaction

import (
	"bytes"
	"context"
	"math/big"
	"net"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	"github.com/symbioticfi/merkle-executor/sdk/chain"
	"github.com/symbioticfi/merkle-executor/sdk/signer"
)

func TestAnvilTransactionLifecycle(t *testing.T) {
	for _, revert := range []bool{false, true} {
		name := "confirmed logs"
		if revert {
			name = "canonical revert"
		}
		t.Run(name, func(t *testing.T) {
			rpcClient, client, endpoint := startAnvil(t)
			backend := anvilBackend(t, endpoint)
			s := anvilSigner(t)
			m, err := New(backend, s, big.NewInt(31337), Config{MaxFeeGwei: 30, Confirmations: 1}, logr.Discard())
			if err != nil {
				t.Fatal(err)
			}
			to := common.HexToAddress("0xdead")
			if !revert {
				setAnvilCode(t, rpcClient, to, "0x600160005260206000a000")
			}
			if err := m.Submit(t.Context(), Request{To: to}); err != nil {
				t.Fatal(err)
			}
			// Changing state after estimation makes the actual transaction revert.
			if revert {
				setAnvilCode(t, rpcClient, to, "0x60006000fd")
			}
			mineAnvilBlock(t, rpcClient)
			if result, err := m.Poll(t.Context()); result != nil || err != nil || !m.Busy() {
				t.Fatalf("inclusion passed confirmation policy: %+v %v", result, err)
			}
			mineAnvilBlock(t, rpcClient)
			result, err := m.Poll(t.Context())
			if err != nil || result == nil || result.Execution == nil || result.Reverted != revert || m.Busy() {
				t.Fatalf("canonical result: %+v %v", result, err)
			}
			receipt, err := client.TransactionReceipt(t.Context(), result.Hash)
			if err != nil {
				t.Fatal(err)
			}
			if result.Execution.BlockHash != receipt.BlockHash {
				t.Fatal("execution has wrong canonical block")
			}
			if !revert && (len(result.Execution.Logs) != 1 || len(result.Execution.Logs[0].Data) != 32 || result.Execution.Logs[0].Data[31] != 1) {
				t.Fatalf("execution lost receipt logs: %+v", result.Execution)
			}
		})
	}
}

type anvilRelay struct {
	*chain.Client
	forward bool
	sends   []*types.Transaction
}

func (r *anvilRelay) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	r.sends = append(r.sends, tx)
	if r.forward {
		if err := r.Client.SendTransaction(ctx, tx); err != nil {
			return err
		}
		return context.DeadlineExceeded
	}
	return nil
}

func TestAnvilAmbiguousBroadcastKeepsExactTransaction(t *testing.T) {
	rpcClient, client, endpoint := startAnvil(t)
	relay := &anvilRelay{Client: anvilBackend(t, endpoint), forward: true}
	m, err := New(relay, anvilSigner(t), big.NewInt(31337), Config{MaxFeeGwei: 30}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Submit(t.Context(), testRequest()); !errors.Is(err, context.DeadlineExceeded) || !m.Busy() {
		t.Fatalf("ambiguous submit lost work: %v", err)
	}
	_, _ = m.Poll(t.Context())
	if len(relay.sends) != 2 || relay.sends[0].Hash() != relay.sends[1].Hash() {
		t.Fatal("ambiguous retry changed signed transaction")
	}
	mineAnvilBlock(t, rpcClient)
	result, err := m.Poll(t.Context())
	if err != nil || result == nil || result.Hash != relay.sends[0].Hash() {
		t.Fatalf("ambiguous transaction not reconciled: %+v %v", result, err)
	}
	if _, err := client.TransactionReceipt(t.Context(), result.Hash); err != nil {
		t.Fatal(err)
	}
}

func TestAnvilUnknownNonceConsumptionRemainsBlocked(t *testing.T) {
	rpcClient, client, endpoint := startAnvil(t)
	s := anvilSigner(t)
	relay := &anvilRelay{Client: anvilBackend(t, endpoint)}
	m, err := New(relay, s, big.NewInt(31337), Config{MaxFeeGwei: 30}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Submit(t.Context(), testRequest()); err != nil {
		t.Fatal(err)
	}
	to := common.HexToAddress("0xbeef")
	external, err := s.SignTx(t.Context(), types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(31337), Nonce: 0, To: &to, Gas: 21_000, GasFeeCap: big.NewInt(10_000_000_000), GasTipCap: big.NewInt(1_000_000_000)}), big.NewInt(31337))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendTransaction(t.Context(), external); err != nil {
		t.Fatal(err)
	}
	mineAnvilBlock(t, rpcClient)
	for range 2 {
		if result, err := m.Poll(t.Context()); result != nil || !errors.Is(err, ErrNonceConsumed) || !m.Busy() {
			t.Fatalf("foreign nonce treated as completion: %+v %v", result, err)
		}
	}
	if len(relay.sends) != 1 {
		t.Fatal("rebroadcast consumed nonce")
	}
	if err := m.Submit(t.Context(), testRequest()); !errors.Is(err, ErrBusy) {
		t.Fatalf("lost uncertain ownership: %v", err)
	}
}

func TestAnvilFreshProcessRefusesPriorPendingTransaction(t *testing.T) {
	_, _, endpoint := startAnvil(t)
	backend := anvilBackend(t, endpoint)
	s := anvilSigner(t)
	first, err := New(backend, s, big.NewInt(31337), Config{MaxFeeGwei: 30}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Submit(t.Context(), testRequest()); err != nil {
		t.Fatal(err)
	}
	fresh, err := New(backend, s, big.NewInt(31337), Config{MaxFeeGwei: 30}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.Submit(t.Context(), Request{To: common.HexToAddress("0xbeef")}); err == nil || fresh.Busy() {
		t.Fatalf("fresh process overwrote pending work: %v", err)
	}
}

func anvilBackend(t *testing.T, endpoint string) *chain.Client {
	t.Helper()
	client, err := chain.Dial(t.Context(), endpoint, "", big.NewInt(31337))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}
func anvilSigner(t *testing.T) signer.Signer {
	t.Helper()
	s, err := signer.NewFromHexKey(testKey)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func setAnvilCode(t *testing.T, client *rpc.Client, to common.Address, code string) {
	t.Helper()
	if err := client.CallContext(t.Context(), nil, "anvil_setCode", to, code); err != nil {
		t.Fatal(err)
	}
}
func mineAnvilBlock(t *testing.T, client *rpc.Client) {
	t.Helper()
	if err := client.CallContext(t.Context(), nil, "anvil_mine", 1); err != nil {
		t.Fatal(err)
	}
}
func startAnvil(t *testing.T) (*rpc.Client, *ethclient.Client, string) {
	t.Helper()
	anvil, err := exec.LookPath("anvil")
	if err != nil {
		t.Skip("anvil is not installed")
	}
	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	cmd := exec.CommandContext(t.Context(), anvil, "--no-mining", "--silent", "--chain-id", "31337", "--port", strconv.Itoa(port))
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	})
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		client, err := rpc.DialContext(t.Context(), endpoint)
		if err == nil {
			var chainID string
			if err := client.CallContext(t.Context(), &chainID, "eth_chainId"); err == nil {
				eth := ethclient.NewClient(client)
				t.Cleanup(eth.Close)
				mineAnvilBlock(t, client)
				return client, eth, endpoint
			}
			client.Close()
		}
		select {
		case err := <-done:
			t.Fatalf("anvil exited: %v\n%s", err, output.String())
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("anvil did not start:\n%s", output.String())
	return nil, nil, ""
}
