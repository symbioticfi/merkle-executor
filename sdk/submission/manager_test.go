package submission

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

type testTransport struct {
	restores        int
	mutateInput     bool
	key             *ecdsa.PrivateKey
	store           Store
	owner           string
	sends, variants int
	changed         bool
	receipt         *types.Receipt
	header          *types.Header
}

func (t *testTransport) Hold(id string) error {
	if t.owner != "" && t.owner != id {
		return ErrLaneBusy
	}
	t.owner = id
	return nil
}
func (t *testTransport) Release(id string) error {
	if t.owner != id {
		return ErrLaneBusy
	}
	t.owner = ""
	return nil
}
func (t *testTransport) RestoreConfirmedNonce(uint64) { t.restores++ }
func (t *testTransport) SendAsync(_ context.Context, req Request, persist Persist) (<-chan Outcome, bool) {
	out := Outcome{}
	if t.mutateInput {
		req.Data[0] = 99
	}
	for i := 0; i < t.variants; i++ {
		to := req.To
		if t.changed && i > 0 {
			to = common.HexToAddress("0x9999")
		}
		unsigned := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(1), To: &to, Data: req.Data, Gas: 100000, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(int64(10 + i))})
		signed, err := types.SignTx(unsigned, types.LatestSignerForChainID(big.NewInt(1)), t.key)
		if err == nil {
			err = persist(unsigned, signed)
		}
		if err != nil {
			out.Err = err
			break
		}
		var saved record
		found, err := t.store.Load(journalKey, &saved)
		if err != nil || !found {
			out.Err = errors.New("broadcast has no durable journal")
			break
		}
		t.sends++
		out.Hash = signed.Hash()
		out.Attempts = append(out.Attempts, signed.Hash())
	}
	ch := make(chan Outcome, 1)
	ch <- out
	return ch, true
}
func (t *testTransport) TransactionReceipt(_ context.Context, hash common.Hash) (*types.Receipt, error) {
	if t.receipt == nil || t.receipt.TxHash != hash {
		return nil, ethereum.NotFound
	}
	return t.receipt, nil
}
func (t *testTransport) HeaderByNumber(context.Context, *big.Int) (*types.Header, error) {
	return t.header, nil
}
func (t *testTransport) HeaderByHash(context.Context, common.Hash) (*types.Header, error) {
	return t.header, nil
}
func fixture(t *testing.T) (*testTransport, *state.Store) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &testTransport{key: key, store: store, variants: 2, header: &types.Header{Number: big.NewInt(10)}}, store
}
func manager(t *testing.T, transport *testTransport, store Store) *Manager {
	t.Helper()
	m, err := New(transport, crypto.PubkeyToAddress(transport.key.PublicKey), big.NewInt(1), store, Config{PollInterval: time.Millisecond}, transport, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func TestSignedVariantsPersistAndRestartNeverReplays(t *testing.T) {
	transport, store := fixture(t)
	m := manager(t, transport, store)
	req := Request{ID: "stable-order", To: common.HexToAddress("0x1234"), Data: []byte{1, 2}}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	got, err := m.Send(ctx, req)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || !got.Pending || len(got.Attempts) != 2 || transport.sends != 2 || transport.owner != req.ID {
		t.Fatalf("lost ownership or attempts: %+v %v", got, err)
	}
	if err = m.Acknowledge(t.Context(), req.ID); err == nil {
		t.Fatal("uncertain work acknowledged")
	}
	recovered := manager(t, transport, store)
	ctx, cancel = context.WithTimeout(t.Context(), 10*time.Millisecond)
	_, _ = recovered.Send(ctx, req)
	cancel()
	if transport.sends != 2 {
		t.Fatal("recovery replayed non-idempotent recipe")
	}
	transport.receipt = &types.Receipt{TxHash: got.Attempts[0], BlockNumber: big.NewInt(10), BlockHash: transport.header.Hash(), Status: types.ReceiptStatusSuccessful}
	final, err := recovered.Send(t.Context(), req)
	if err != nil || !final.Confirmed || final.Execution == nil {
		t.Fatalf("missing canonical evidence: %+v %v", final, err)
	}
	if transport.owner == "" {
		t.Fatal("lane released before durable host acknowledgement")
	}
	if err = recovered.Acknowledge(t.Context(), req.ID); err != nil {
		t.Fatal(err)
	}
	if transport.owner != "" {
		t.Fatal("acknowledged lane remains reserved")
	}
}
func TestChangedReplacementNeverBroadcasts(t *testing.T) {
	transport, store := fixture(t)
	transport.changed = true
	m := manager(t, transport, store)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	got, _ := m.Send(ctx, Request{ID: "order", To: common.HexToAddress("0x1234"), Data: []byte{1}})
	if transport.sends != 1 || !got.Pending || len(got.Attempts) != 1 {
		t.Fatalf("changed replacement escaped: %+v", got)
	}
}

type brokenStore struct{ Store }

func (brokenStore) Save(string, any) error { return errors.New("disk full") }
func TestPersistenceFailureBlocksTransportAndRetainsOwner(t *testing.T) {
	transport, store := fixture(t)
	m := manager(t, transport, brokenStore{store})
	got, err := m.Send(t.Context(), Request{ID: "order", To: common.HexToAddress("0x1234"), Data: []byte{1}})
	if err == nil || !got.Pending || transport.sends != 0 || transport.owner != "order" {
		t.Fatalf("storage failure released work: %+v %v", got, err)
	}
}

func TestTransportCannotMutateAdmittedCalldata(t *testing.T) {
	transport, store := fixture(t)
	transport.mutateInput = true
	m := manager(t, transport, store)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	_, _ = m.Send(ctx, Request{ID: "order", To: common.HexToAddress("0x1234"), Data: []byte{1}})
	if transport.sends != 0 {
		t.Fatal("host transport changed admitted calldata")
	}
}

func TestRestartDoesNotTrustSavedTerminalEvidence(t *testing.T) {
	transport, store := fixture(t)
	m := manager(t, transport, store)
	req := Request{ID: "order", To: common.HexToAddress("0x1234"), Data: []byte{1}}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	pending, _ := m.Send(ctx, req)
	cancel()
	transport.receipt = &types.Receipt{TxHash: pending.Attempts[0], BlockNumber: big.NewInt(10), BlockHash: transport.header.Hash(), Status: types.ReceiptStatusSuccessful}
	if got, err := m.Send(t.Context(), req); err != nil || !got.Confirmed {
		t.Fatalf("did not confirm: %+v %v", got, err)
	}
	transport.receipt = nil
	restores := transport.restores
	recovered := manager(t, transport, store)
	if transport.restores != restores {
		t.Fatal("constructor advanced nonce floor from unverified disk evidence")
	}
	ctx, cancel = context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	got, err := recovered.Send(ctx, req)
	if got.Confirmed || got.Reverted || !got.Pending {
		t.Fatalf("reorged disk evidence trusted: %+v %v", got, err)
	}
}

func TestAcknowledgementRejectsChangedTerminalStatus(t *testing.T) {
	transport, store := fixture(t)
	m := manager(t, transport, store)
	req := Request{ID: "order", To: common.HexToAddress("0x1234"), Data: []byte{1}}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	pending, _ := m.Send(ctx, req)
	cancel()
	transport.receipt = &types.Receipt{TxHash: pending.Attempts[0], BlockNumber: big.NewInt(10), BlockHash: transport.header.Hash(), Status: types.ReceiptStatusSuccessful}
	if _, err := m.Send(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	transport.receipt.Status = types.ReceiptStatusFailed
	if err := m.Acknowledge(t.Context(), req.ID); err == nil {
		t.Fatal("business progress acknowledged for changed execution")
	}
	if transport.owner != req.ID {
		t.Fatal("changed execution released signer lane")
	}
	var saved record
	found, err := store.Load(journalKey, &saved)
	if err != nil || !found || saved.Terminal != "confirmed" {
		t.Fatalf("prior business evidence overwritten: %+v %v", saved, err)
	}
}
