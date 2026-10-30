package claims

import (
	"bytes"
	"context"
	stderrors "errors"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func (p *Provider) discover(ctx context.Context, s provider.Snapshot, r route) ([]map[string]hexutil.Bytes, error) {
	target := r.action.Leaf.Target
	var candidates []*big.Int
	switch r.kind {
	case "noon":
		count, err := p.uint(ctx, s, target, "getUserNextRequestId", s.Account)
		if err != nil {
			return nil, err
		}
		n, err := bounded(count)
		if err != nil {
			return nil, err
		}
		period, err := p.uint(ctx, s, target, "withdrawPeriod")
		if err != nil {
			return nil, err
		}
		rows, err := p.rows(ctx, s, func(int) common.Address { return target }, "getWithdrawalRequest", n, func(i int) []any { return []any{s.Account, big.NewInt(int64(i))} })
		if err != nil {
			return nil, err
		}
		for i, row := range rows {
			amount, timestamp := row[0].(*big.Int), row[1].(*big.Int)
			if amount.Sign() > 0 && !row[2].(bool) && new(big.Int).Add(timestamp, period).Cmp(new(big.Int).SetUint64(s.Timestamp)) <= 0 {
				candidates = append(candidates, big.NewInt(int64(i)))
			}
		}
	case "pareto-deposit", "pareto-withdraw":
		binding, err := p.values(ctx, s, target, "strategy")
		if err != nil {
			return nil, err
		}
		epoch, err := p.uint(ctx, s, binding[0].(common.Address), "epochNumber")
		if err != nil {
			return nil, err
		}
		n, err := bounded(new(big.Int).Add(epoch, big.NewInt(2)))
		if err != nil {
			return nil, err
		}
		ledger := "userWithdrawalsEpochs"
		if r.kind == "pareto-deposit" {
			ledger = "userDepositsEpochs"
		}
		rows, err := p.rows(ctx, s, func(int) common.Address { return target }, ledger, n, func(i int) []any { return []any{s.Account, big.NewInt(int64(i))} })
		if err != nil {
			return nil, err
		}
		for i, row := range rows {
			if row[0].(*big.Int).Sign() > 0 {
				candidates = append(candidates, big.NewInt(int64(i)))
			}
		}
	case "threef":
		exists := func(i int) (bool, error) {
			v, err := p.values(ctx, s, r.factory, "isRequest", crypto.CreateAddress(r.factory, 7+3*uint64(i)))
			if err != nil {
				return false, err
			}
			return v[0].(bool), nil
		}
		tooMany, err := exists(maxScan)
		if err != nil {
			return nil, err
		}
		if tooMany {
			return nil, errors.New("3F claims exceed complete scan limit")
		}
		lo, hi := 0, maxScan
		for lo < hi {
			mid := lo + (hi-lo)/2
			ok, err := exists(mid)
			if err != nil {
				return nil, err
			}
			if ok {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		addressAt := func(i int) common.Address { return crypto.CreateAddress(r.factory, 7+3*uint64(i)) }
		ready, err := p.rows(ctx, s, addressAt, "canWithdraw", lo, func(int) []any { return nil })
		if err != nil {
			return nil, err
		}
		balances, err := p.rows(ctx, s, addressAt, "balancesOf", lo, func(int) []any { return []any{s.Account} })
		if err != nil {
			return nil, err
		}
		for i := range ready {
			if ready[i][0].(bool) && (balances[i][0].(*big.Int).Sign() > 0 || balances[i][1].(*big.Int).Sign() > 0) {
				request := addressAt(i)
				candidates = append(candidates, new(big.Int).SetBytes(request[:]))
			}
		}
	case "mint", "deposit", "withdraw", "redeem":
		ledger, maximum := "claimableDepositRequest", "maxDeposit"
		if r.kind == "mint" {
			maximum = "maxMint"
		}
		if r.kind == "withdraw" || r.kind == "redeem" {
			ledger, maximum = "claimableRedeemRequest", "maxWithdraw"
		}
		if r.kind == "redeem" {
			maximum = "maxRedeem"
		}
		entitlement, err := p.uint(ctx, s, target, ledger, new(big.Int), s.Account)
		if err != nil {
			return nil, err
		}
		if entitlement.Sign() == 0 {
			return nil, nil
		}
		amount, err := p.uint(ctx, s, target, maximum, s.Account)
		if err != nil {
			return nil, err
		}
		if amount.Sign() > 0 {
			candidates = append(candidates, amount)
		}
	case "cancel-deposit", "cancel-redeem":
		ledger := "claimableCancelDepositRequest"
		if r.kind == "cancel-redeem" {
			ledger = "claimableCancelRedeemRequest"
		}
		amount, err := p.uint(ctx, s, target, ledger, new(big.Int), s.Account)
		if err != nil {
			return nil, err
		}
		if amount.Sign() > 0 {
			candidates = append(candidates, new(big.Int))
		}
	case "deposit-refund", "redeem-refund":
		ledger, args := "redeemEscrowBalance", []any{s.Account}
		if r.kind == "deposit-refund" {
			if len(r.action.Leaf.Data) != 68 {
				return nil, errors.New("invalid deposit refund template")
			}
			ledger, args = "depositEscrowBalance", append(args, common.BytesToAddress(r.action.Leaf.Data[36:68]))
		}
		amount, err := p.uint(ctx, s, target, ledger, args...)
		if err != nil {
			return nil, err
		}
		if amount.Sign() > 0 {
			candidates = append(candidates, new(big.Int))
		}
	case "pareto-normal", "pareto-instant":
		binding, err := p.values(ctx, s, target, "strategy")
		if err != nil {
			return nil, err
		}
		balance, err := p.uint(ctx, s, binding[0].(common.Address), "balanceOf", s.Account)
		if err != nil {
			return nil, err
		}
		if balance.Sign() > 0 {
			candidates = append(candidates, new(big.Int))
		}
	}
	var prepared []map[string]hexutil.Bytes
	for _, value := range candidates {
		v := variant(r, value)
		data := v[r.action.Key]
		// Only permission-masked bytes may vary; malformed authoring is an error,
		// not an unready issuer request.
		for i, b := range data {
			if (b^r.action.Leaf.Data[i]) & ^r.action.Leaf.DataMask[i] != 0 {
				return nil, errors.New("claim field is pinned by its permission")
			}
		}
		raw, err := p.reader.CallContract(ctx, ethereum.CallMsg{From: s.Account, To: &target, Data: data}, s.BlockNumber)
		if err != nil {
			var rpcErr interface{ ErrorCode() int }
			if stderrors.As(err, &rpcErr) && rpcErr.ErrorCode() == 3 {
				continue
			}
			return nil, errors.Errorf("claim readiness simulation: %w", err)
		}
		// Direct Pareto claims return the transferred amount and can return zero.
		if (r.kind == "pareto-normal" || r.kind == "pareto-instant") && len(raw) >= 32 && bytes.Equal(raw[:32], make([]byte, 32)) {
			continue
		}
		prepared = append(prepared, v)
		if len(prepared) == maxBatch {
			break
		}
	}
	return prepared, nil
}

// Owner-ledger reads share the transport's bounded batch path. No fixed start
// can hide older requests; a scope beyond the complete bound is an error.
func (p *Provider) rows(ctx context.Context, s provider.Snapshot, target func(int) common.Address, name string, n int, args func(int) []any) ([][]any, error) {
	method := readABI.Methods[name]
	requests := make([]protocolio.ReadRequest, n)
	for i := range requests {
		data, err := method.Inputs.Pack(args(i)...)
		if err != nil {
			return nil, err
		}
		requests[i] = protocolio.ContractRead(target(i), append(bytes.Clone(method.ID), data...), s.BlockNumber, true)
	}
	responses, err := protocolio.ReadGroup(ctx, p.reader, requests...)
	if err != nil {
		return nil, err
	}
	rows := make([][]any, n)
	for i := range rows {
		raw, err := responses.At(i).CallData()
		if err != nil {
			return nil, errors.Errorf("claim %s inventory: %w", name, err)
		}
		rows[i], err = method.Outputs.Unpack(raw)
		if err != nil {
			return nil, errors.Errorf("invalid claim %s inventory", name)
		}
	}
	return rows, nil
}
