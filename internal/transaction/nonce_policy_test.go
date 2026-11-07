package transaction

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	rt "github.com/symbioticfi/merkle-executor/sdk/runtime"
)

func TestResumeObservesAuthorizedAttemptAboveCurrentFeeCeiling(t *testing.T) {
	f := newFixture()
	f.base = 10_000_000_000
	d := driver(t, f)
	reserve(t, d)
	saved := signedRecord(t, d)
	sent := f.sent()[0]
	f.include(sent, 1, types.ReceiptStatusSuccessful)
	cfg := config()
	cfg.MaxFeeGwei = 1
	resumed, err := New(f, f, testSigner(t), big.NewInt(31337), cfg)
	if err != nil {
		t.Fatal(err)
	}
	reserve(t, resumed)
	result := resumed.Resume(t.Context(), saved, persist(&saved))
	if result.Err != nil || result.Execution == nil {
		t.Fatalf("lowering ceiling prevented observation of confirmed persisted %s wei attempt: %+v", sent.GasFeeCap(), result)
	}
}

type closingNonceFixture struct {
	*fixture
	nonceCalls int
}

func (f *closingNonceFixture) NonceAt(context.Context, common.Address, *big.Int) (uint64, error) {
	f.nonceCalls++
	if f.nonceCalls == 4 {
		// The first replacement's closing nonce observation sees consumption.
		// The next response is a lagging write node returning the old nonce.
		f.mine(1)
		return 1, nil
	}
	return 0, nil
}
func TestClosingNonceConsumptionPermanentlyStopsMutation(t *testing.T) {
	f := &closingNonceFixture{fixture: newFixture()}
	d, err := New(f, f, testSigner(t), big.NewInt(31337), config())
	if err != nil {
		t.Fatal(err)
	}
	reserve(t, d)
	f.onSend = func(tx *types.Transaction, n int) error {
		if n == 1 {
			f.mu.Lock()
			f.base = 10_000_000_000
			f.mu.Unlock()
			f.mine(1)
		} else {
			f.include(tx, 1, types.ReceiptStatusSuccessful)
		}
		return nil
	}
	var saved rt.Transaction
	result := d.Execute(t.Context(), intent(), persist(&saved))
	if len(f.sent()) != 1 || len(saved.Attempts) != 1 {
		t.Fatalf("signed/sent after observing consumed nonce: sends=%d attempts=%d nonceReads=%d result=%+v", len(f.sent()), len(saved.Attempts), f.nonceCalls, result)
	}
}

func TestStalledRebroadcastClosesNonceAfterEstimate(t *testing.T) {
	f := newFixture()
	cfg := config()
	cfg.Horizon.BlockTime = time.Millisecond
	cfg.PendingTimeout = 200 * time.Millisecond
	d, err := New(f, f, testSigner(t), big.NewInt(31337), cfg)
	if err != nil {
		t.Fatal(err)
	}
	reserve(t, d)
	estimates := 0
	f.estimate = func(context.Context) (uint64, error) {
		estimates++
		if estimates > 1 {
			f.mu.Lock()
			f.nonce = 1
			f.mu.Unlock()
		}
		return 21000, nil
	}
	f.onSend = func(tx *types.Transaction, n int) error {
		if n == 1 {
			go func() { time.Sleep(6 * time.Millisecond); f.mine(3) }()
		} else {
			f.include(tx, 1, types.ReceiptStatusSuccessful)
		}
		return nil
	}
	var saved rt.Transaction
	result := d.Execute(t.Context(), intent(), persist(&saved))
	if len(f.sent()) != 1 {
		t.Fatalf("rebroadcast after nonce advanced during estimate: sends=%d estimates=%d result=%+v", len(f.sent()), estimates, result)
	}
}

func TestNewSignatureStillRequiresCurrentFeeCeiling(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	tx := rt.Transaction{Intent: intent(), Sender: testSigner(t).Address(), ChainID: big.NewInt(31337), Admitted: true}
	saved := 0
	_, err := d.sign(t.Context(), &tx, 21_000, feeQuote{baseFee: big.NewInt(1_000_000_000), tip: big.NewInt(20_000_000), maxFee: big.NewInt(101_000_000_000)}, func(rt.Transaction) error { saved++; return nil })
	if err == nil || saved != 0 || len(tx.Attempts) != 0 || len(f.sent()) != 0 {
		t.Fatalf("new signature bypassed current ceiling: err=%v saves=%d", err, saved)
	}
}
