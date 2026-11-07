// Package transaction implements the daemon's exclusive signer lane.
package transaction

// Fee and gas arithmetic retained from txengine; lifecycle ownership is new.
import (
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-errors/errors"
	"math"
	"math/big"
	"slices"
	"strings"
	"time"
)

const (
	validityHorizonBlocks   = 2
	basisPoints             = 10_000
	replaceReasonValidity   = "validity"
	replaceReasonCongestion = "congestion"
	replaceReasonStall      = "stall"
)

// HorizonConfig is explicit deployment policy, supplied by the host's YAML defaults.
type HorizonConfig struct {
	MaxBlocks                 int           `yaml:"maxBlocks"`
	BlockTime                 time.Duration `yaml:"blockTime"`
	TipFloorGwei              float64       `yaml:"tipFloorGwei"`
	FullBlockTipGwei          float64       `yaml:"fullBlockTipGwei"`
	CongestedTipFloorGwei     float64       `yaml:"congestedTipFloorGwei"`
	CongestedTipCapGwei       float64       `yaml:"congestedTipCapGwei"`
	CongestedRewardBlocks     int           `yaml:"congestedRewardBlocks"`
	CongestedRewardPercentile float64       `yaml:"congestedRewardPercentile"`
	EscalateAfterFullBlocks   int           `yaml:"escalateAfterFullBlocks"`
	StallAfterBlocks          int           `yaml:"stallAfterBlocks"`
	GasHeadroomBps            int           `yaml:"gasHeadroomBps"`
	FallbackGasHeadroomBps    int           `yaml:"fallbackGasHeadroomBps"`
}

func (c HorizonConfig) Validate() error {
	for _, tip := range []float64{c.TipFloorGwei, c.FullBlockTipGwei, c.CongestedTipFloorGwei, c.CongestedTipCapGwei} {
		if math.IsNaN(tip) || math.IsInf(tip, 0) || tip <= 0 || gweiToWei(tip).Sign() == 0 {
			return errors.New("finite positive policy tip values required")
		}
	}
	if c.TipFloorGwei > c.FullBlockTipGwei || c.FullBlockTipGwei > c.CongestedTipFloorGwei || c.CongestedTipFloorGwei > c.CongestedTipCapGwei {
		return errors.New("policy tip floors and cap are not ordered")
	}
	if c.MaxBlocks < 1 || c.MaxBlocks > 1024 || c.BlockTime <= 0 || c.CongestedRewardBlocks < 1 || c.CongestedRewardBlocks > 256 || c.EscalateAfterFullBlocks < 1 || c.EscalateAfterFullBlocks > 256 || c.StallAfterBlocks < 1 || c.StallAfterBlocks > 256 {
		return errors.New("bounded positive horizon blocks and block time required")
	}
	if math.IsNaN(c.CongestedRewardPercentile) || math.IsInf(c.CongestedRewardPercentile, 0) || c.CongestedRewardPercentile < 0 || c.CongestedRewardPercentile > 100 || c.GasHeadroomBps < 1 || c.GasHeadroomBps > basisPoints || c.FallbackGasHeadroomBps < c.GasHeadroomBps || c.FallbackGasHeadroomBps > basisPoints {
		return errors.New("invalid policy reward percentile or gas headroom")
	}
	return nil
}

type horizonPolicy struct {
	maxBlocks                                                  int
	blockTime                                                  time.Duration
	tipFloor, fullBlockTip, congestedTipFloor, congestedTipCap *big.Int
	rewardBlocks                                               int
	rewardPercentile                                           float64
	escalateAfterFullBlocks, stallAfterBlocks                  int
	gasHeadroomBps, fallbackGasHeadroomBps                     int
}

func newHorizonPolicy(c HorizonConfig) horizonPolicy {
	return horizonPolicy{maxBlocks: c.MaxBlocks, blockTime: c.BlockTime, tipFloor: gweiToWei(c.TipFloorGwei), fullBlockTip: gweiToWei(c.FullBlockTipGwei), congestedTipFloor: gweiToWei(c.CongestedTipFloorGwei), congestedTipCap: gweiToWei(c.CongestedTipCapGwei), rewardBlocks: c.CongestedRewardBlocks, rewardPercentile: c.CongestedRewardPercentile, escalateAfterFullBlocks: c.EscalateAfterFullBlocks, stallAfterBlocks: c.StallAfterBlocks, gasHeadroomBps: c.GasHeadroomBps, fallbackGasHeadroomBps: c.FallbackGasHeadroomBps}
}

func (p horizonPolicy) window() int {
	return max(p.rewardBlocks, p.escalateAfterFullBlocks, p.stallAfterBlocks, 2)
}

// blockFee is one mined block as eth_feeHistory reports it.
type blockFee struct {
	number       uint64
	baseFee      *big.Int
	gasUsedRatio float64
	reward       *big.Int // the congestion reward percentile of the block's priority fees
}

// feeSnapshot is one fee-history window ending at the latest block.
type feeSnapshot struct {
	head        uint64
	gasLimit    uint64   // latest header gas limit; block gas limits move slowly
	nextBaseFee *big.Int // base fee of block head+1, as the node computes it
	blocks      []blockFee
}

// roomFor reports whether the block left at least gas units unused, so a transaction of that size
// could have been included without displacing another.
func (b blockFee) roomFor(gasLimit, gas uint64) bool {
	return float64(gasLimit)*(1-b.gasUsedRatio) >= float64(gas)
}

func parseFeeSnapshot(history *ethereum.FeeHistory, gasLimit uint64) (feeSnapshot, error) {
	if history == nil || history.OldestBlock == nil || !history.OldestBlock.IsUint64() {
		return feeSnapshot{}, errors.New("fee history has no oldest block")
	}
	count := len(history.GasUsedRatio)
	if count == 0 || len(history.BaseFee) != count+1 || len(history.Reward) != count {
		return feeSnapshot{}, errors.Errorf(
			"fee history has %d blocks, %d base fees and %d reward rows",
			count, len(history.BaseFee), len(history.Reward),
		)
	}
	oldest := history.OldestBlock.Uint64()
	snapshot := feeSnapshot{
		head:     oldest + uint64(count) - 1,
		gasLimit: gasLimit,
		blocks:   make([]blockFee, count),
	}
	for i := range count {
		baseFee, ratio := history.BaseFee[i], history.GasUsedRatio[i]
		if baseFee == nil || baseFee.Sign() < 0 || math.IsNaN(ratio) || ratio < 0 {
			return feeSnapshot{}, errors.Errorf("fee history block %d has invalid base fee or gas used ratio", oldest+uint64(i))
		}
		if len(history.Reward[i]) != 1 || history.Reward[i][0] == nil || history.Reward[i][0].Sign() < 0 {
			return feeSnapshot{}, errors.Errorf("fee history block %d has invalid rewards", oldest+uint64(i))
		}
		snapshot.blocks[i] = blockFee{
			number:       oldest + uint64(i),
			baseFee:      new(big.Int).Set(baseFee),
			gasUsedRatio: min(ratio, 1),
			reward:       new(big.Int).Set(history.Reward[i][0]),
		}
	}
	next := history.BaseFee[count]
	if next == nil || next.Sign() < 0 {
		return feeSnapshot{}, errors.New("fee history has an invalid next base fee")
	}
	snapshot.nextBaseFee = new(big.Int).Set(next)
	return snapshot, nil
}

// growBaseFee is the largest base fee the protocol allows after the given number of full blocks:
// EIP-1559 raises it by at most max(baseFee/8, 1) per block.
func growBaseFee(baseFee *big.Int, blocks int) *big.Int {
	grown := new(big.Int).Set(baseFee)
	for range blocks {
		step := new(big.Int).Div(grown, big.NewInt(params.DefaultBaseFeeChangeDenominator))
		if step.Sign() == 0 {
			step.SetInt64(1)
		}
		grown.Add(grown, step)
	}
	return grown
}

// horizonMaxFee is the fee cap that keeps a transaction valid, at its full tip, in each of the next
// `blocks` blocks even if every one of them is full.
func horizonMaxFee(nextBaseFee, tip *big.Int, blocks int) *big.Int {
	return new(big.Int).Add(growBaseFee(nextBaseFee, blocks-1), tip)
}

// horizonTip prices the priority fee from whether the last two blocks had room for gas units. Room
// in both means any positive tip is included; a single full block needs a modest premium; only a
// run of full blocks, where the call must outbid the marginal transaction, follows the market
// reward percentile, clamped to the configured range.
func horizonTip(snapshot feeSnapshot, gas uint64, policy horizonPolicy) *big.Int {
	blocks := snapshot.blocks
	lastFull := len(blocks) >= 1 && !blocks[len(blocks)-1].roomFor(snapshot.gasLimit, gas)
	previousFull := len(blocks) >= 2 && !blocks[len(blocks)-2].roomFor(snapshot.gasLimit, gas)
	switch {
	case lastFull && previousFull:
		tip := new(big.Int)
		for _, block := range blocks[max(0, len(blocks)-policy.rewardBlocks):] {
			if block.reward.Cmp(tip) > 0 {
				tip.Set(block.reward)
			}
		}
		if tip.Cmp(policy.congestedTipFloor) < 0 {
			tip.Set(policy.congestedTipFloor)
		}
		if tip.Cmp(policy.congestedTipCap) > 0 {
			tip.Set(policy.congestedTipCap)
		}
		return tip
	case lastFull || previousFull:
		return new(big.Int).Set(policy.fullBlockTip)
	default:
		return new(big.Int).Set(policy.tipFloor)
	}
}

// horizonFees prices a new call of gas units under limit: the tip rule plus the exact base-fee bound
// for the policy horizon. A limit below that bound shortens the horizon; one below the next base
// fee fails, because the call could not be included in the next block.
func horizonFees(snapshot feeSnapshot, gas uint64, limit *big.Int, policy horizonPolicy) (feeQuote, error) {
	tip := horizonTip(snapshot, gas, policy)
	maxFee := horizonMaxFee(snapshot.nextBaseFee, tip, policy.maxBlocks)
	if limit != nil && maxFee.Cmp(limit) > 0 {
		maxFee.Set(limit)
	}
	if maxFee.Cmp(snapshot.nextBaseFee) < 0 {
		return feeQuote{}, errors.Errorf(
			"%w: next base fee %s exceeds tx manager max fee %s", ErrFeeLimitReached, snapshot.nextBaseFee, maxFee,
		)
	}
	if room := new(big.Int).Sub(maxFee, snapshot.nextBaseFee); tip.Cmp(room) > 0 {
		tip.Set(room)
	}
	return feeQuote{baseFee: new(big.Int).Set(snapshot.nextBaseFee), tip: tip, maxFee: maxFee}, nil
}

type pendingAction uint8

const (
	pendingHold pendingAction = iota
	pendingReprice
	pendingStall
)

type pendingDecision struct {
	action pendingAction
	reason string
}

// decidePending judges the current attempt of gas units, sent when sentHead was the latest block,
// against the blocks mined since. It reprices when the fee cap would lapse within
// validityHorizonBlocks at the tip floor, or when the attempt lost the last escalateAfterFullBlocks
// blocks to full blocks while valid and the tip rule now asks for at least a replacement bump. Blocks
// it lost while valid and with room point to a dropped or unreachable transaction, which a higher
// fee does not fix, so enough of them report a stall instead. Anything else holds: waiting is free.
func decidePending(snapshot feeSnapshot, attempt feeQuote, gas, sentHead uint64, policy horizonPolicy) pendingDecision {
	if attempt.maxFee.Cmp(horizonMaxFee(snapshot.nextBaseFee, policy.tipFloor, validityHorizonBlocks)) < 0 {
		return pendingDecision{action: pendingReprice, reason: replaceReasonValidity}
	}
	var fullRun, roomy int
	running := true
	for _, block := range slices.Backward(snapshot.blocks) {
		if block.number <= sentHead {
			break
		}
		valid := attempt.maxFee.Cmp(block.baseFee) >= 0
		hasRoom := block.roomFor(snapshot.gasLimit, gas)
		if running && valid && !hasRoom {
			fullRun++
		} else {
			running = false
		}
		if valid && hasRoom {
			roomy++
		}
	}
	if fullRun >= policy.escalateAfterFullBlocks &&
		horizonTip(snapshot, gas, policy).Cmp(bumpFee(attempt.tip)) >= 0 {
		return pendingDecision{action: pendingReprice, reason: replaceReasonCongestion}
	}
	if roomy >= policy.stallAfterBlocks {
		return pendingDecision{action: pendingStall, reason: replaceReasonStall}
	}
	return pendingDecision{action: pendingHold}
}

// judgedSentHead is the block the current attempt counts evidence from. A read endpoint that lagged
// when the attempt went out reports an older head than the one it actually followed, which would
// count blocks mined before the send as blocks it lost. At most one block per elapsed slot, plus one
// already in flight, can follow a send, so the wall clock bounds how far back the evidence reaches.
func judgedSentHead(sentHead, head uint64, sentAt, now time.Time, blockTime time.Duration) uint64 {
	followed := uint64(max(now.Sub(sentAt), 0)/blockTime) + 1
	if head <= followed {
		return sentHead
	}
	return max(sentHead, head-followed)
}

func withGasHeadroom(gas uint64, bps int) uint64 {
	// Divide first so valid uint64 estimates cannot overflow the intermediate product.
	extra := gas/basisPoints*uint64(bps) + gas%basisPoints*uint64(bps)/basisPoints
	if extra > math.MaxUint64-gas {
		return math.MaxUint64
	}
	return gas + extra
}

// isBlockOverrideUnsupported recognizes an endpoint that rejects eth_estimateGas's fourth
// (blockOverrides) parameter, as opposed to a call that reverts or a transport failure.
func isBlockOverrideUnsupported(err error) bool {
	var rpcErr rpc.Error
	if errors.As(err, &rpcErr) && (rpcErr.ErrorCode() == -32602 || rpcErr.ErrorCode() == -32601) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"too many arguments", "invalid params", "invalid argument", "unknown field"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func gweiToWei(gwei float64) *big.Int {
	wei, _ := new(big.Float).Mul(big.NewFloat(gwei), big.NewFloat(params.GWei)).Int(nil)
	return wei
}
func bumpFee(value *big.Int) *big.Int {
	n := new(big.Int).Mul(value, big.NewInt(9))
	n.Add(n, big.NewInt(7)).Div(n, big.NewInt(8))
	if n.Cmp(value) <= 0 {
		n.Add(value, big.NewInt(1))
	}
	return n
}
func reserveFeeBump(limit *big.Int) *big.Int {
	return new(big.Int).Div(new(big.Int).Mul(limit, big.NewInt(8)), big.NewInt(9))
}
func replacementFees(previous feeQuote, current *feeQuote, limit *big.Int) (feeQuote, error) {
	next := feeQuote{baseFee: new(big.Int).Set(previous.baseFee), tip: bumpFee(previous.tip), maxFee: bumpFee(previous.maxFee)}
	required := new(big.Int).Set(next.maxFee)
	if current != nil {
		next.baseFee.Set(current.baseFee)
		next.maxFee.Set(maxBig(next.maxFee, current.maxFee))
	}
	if next.maxFee.Cmp(limit) > 0 {
		next.maxFee.Set(limit)
	}
	room := new(big.Int).Sub(next.maxFee, next.baseFee)
	if room.Sign() < 0 {
		return feeQuote{}, ErrFeeLimitReached
	}
	if current != nil {
		freshTip := new(big.Int).Set(current.tip)
		if freshTip.Cmp(room) > 0 {
			freshTip.Set(room)
		}
		next.tip.Set(maxBig(next.tip, freshTip))
	}
	if next.maxFee.Cmp(required) < 0 || next.tip.Cmp(next.maxFee) > 0 {
		return feeQuote{}, ErrFeeLimitReached
	}
	return next, nil
}
func maxBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}

type feeQuote struct{ baseFee, tip, maxFee *big.Int }
