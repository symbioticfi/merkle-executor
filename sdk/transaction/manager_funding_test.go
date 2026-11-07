package transaction

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"

	"github.com/symbioticfi/merkle-executor/sdk/signer"
)

type fundingSigner struct {
	signer.Signer
	calls int
}

func (s *fundingSigner) SignTx(ctx context.Context, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	s.calls++
	return s.Signer.SignTx(ctx, tx, chainID)
}

func TestSubmitChecksMaximumFundingBeforeSigning(t *testing.T) {
	// Quote includes the final 10% gas margin and the full EIP-1559 fee cap.
	cost := new(big.Int).Mul(big.NewInt(23_100), big.NewInt(2_100_000_000))
	readFailure := errors.New("sending balance unavailable")
	for _, tc := range []struct {
		name    string
		balance *big.Int
		readErr error
		wantErr string
	}{
		{"exact funding", cost, nil, ""},
		{"surplus funding", new(big.Int).Add(cost, big.NewInt(1)), nil, ""},
		{"one wei short", new(big.Int).Sub(cost, big.NewInt(1)), nil, "insufficient transaction funding"},
		{"empty account", new(big.Int), nil, "insufficient transaction funding"},
		{"missing balance", nil, nil, "invalid transaction sender balance"},
		{"negative balance", big.NewInt(-1), nil, "invalid transaction sender balance"},
		{"overflow balance", new(big.Int).Lsh(big.NewInt(1), 256), nil, "invalid transaction sender balance"},
		{"unavailable balance", cost, readFailure, "read transaction sender balance"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newBackend()
			b.balance, b.balanceErr = tc.balance, tc.readErr
			m := newManager(t, b)
			s := &fundingSigner{Signer: m.signer}
			m.signer = s
			err := m.Submit(t.Context(), testRequest())
			if tc.wantErr == "" {
				if err != nil || s.calls != 1 || len(b.sent) != 1 || !m.Busy() || b.sent[0].Cost().Cmp(cost) != 0 {
					t.Fatalf("funded submission: err=%v signatures=%d sends=%d busy=%v", err, s.calls, len(b.sent), m.Busy())
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) || s.calls != 0 || len(b.sent) != 0 || m.Busy() {
				t.Fatalf("unfunded submission: err=%v signatures=%d sends=%d busy=%v", err, s.calls, len(b.sent), m.Busy())
			}
			if tc.readErr != nil && !errors.Is(err, tc.readErr) {
				t.Fatalf("lost balance read error: %v", err)
			}
			if b.balanceReads != 1 || b.balanceAccount != s.Address() || b.balanceBlock != nil {
				t.Fatalf("incorrect funding read: reads=%d account=%s block=%v", b.balanceReads, b.balanceAccount, b.balanceBlock)
			}
		})
	}
}

func TestSubmitRejectsOverflowingMaximumCostBeforeSigning(t *testing.T) {
	b := newBackend()
	b.headers[0].BaseFee = new(big.Int).Lsh(big.NewInt(1), 254)
	b.balance = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	m := newManager(t, b)
	m.feeLimit = new(big.Int).Lsh(big.NewInt(1), 255)
	s := &fundingSigner{Signer: m.signer}
	m.signer = s
	err := m.Submit(t.Context(), testRequest())
	if err == nil || !strings.Contains(err.Error(), "invalid maximum transaction cost") || s.calls != 0 || len(b.sent) != 0 || m.Busy() {
		t.Fatalf("overflowing cost admitted: err=%v signatures=%d sends=%d busy=%v", err, s.calls, len(b.sent), m.Busy())
	}
}

func TestSubmitCancellationDuringFundingReadDoesNotSign(t *testing.T) {
	b := newBackend()
	m := newManager(t, b)
	s := &fundingSigner{Signer: m.signer}
	m.signer = s
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	b.afterBalance = cancel
	err := m.Submit(ctx, testRequest())
	if !errors.Is(err, context.Canceled) || s.calls != 0 || len(b.sent) != 0 || m.Busy() {
		t.Fatalf("cancelled funding read admitted: err=%v signatures=%d sends=%d busy=%v", err, s.calls, len(b.sent), m.Busy())
	}
}

func TestSubmitCanRetryAfterFundingRecovers(t *testing.T) {
	b := newBackend()
	b.balance = new(big.Int)
	m := newManager(t, b)
	s := &fundingSigner{Signer: m.signer}
	m.signer = s
	if err := m.Submit(t.Context(), testRequest()); err == nil || m.Busy() || s.calls != 0 {
		t.Fatalf("unfunded request captured: %v", err)
	}
	b.balance = big.NewInt(1_000_000_000_000_000_000)
	if err := m.Submit(t.Context(), testRequest()); err != nil || s.calls != 1 || len(b.sent) != 1 || !m.Busy() || b.sent[0].Nonce() != 0 {
		t.Fatalf("refunded request blocked: err=%v signatures=%d sends=%d busy=%v", err, s.calls, len(b.sent), m.Busy())
	}
}
