package transaction

// Arithmetic and policy regression fixtures retained from the former txengine.
import (
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/go-errors/errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

const horizonTestGas = 4_000_000

func block(number uint64, baseFeeGwei, ratio, rewardGwei float64) blockFee {
	return blockFee{number: number, baseFee: gweiToWei(baseFeeGwei), gasUsedRatio: ratio, reward: gweiToWei(rewardGwei)}
}

func testSnapshot(nextBaseFeeGwei float64, blocks ...blockFee) feeSnapshot {
	return feeSnapshot{
		head: blocks[len(blocks)-1].number, gasLimit: 60_000_000,
		nextBaseFee: gweiToWei(nextBaseFeeGwei), blocks: blocks,
	}
}

func TestGrowBaseFee(t *testing.T) {
	for _, tc := range []struct {
		base   int64
		blocks int
		want   int64
	}{
		{base: 100, blocks: 0, want: 100},
		{base: 100, blocks: 1, want: 112},
		{base: 8, blocks: 1, want: 9},
		{base: 7, blocks: 1, want: 8}, // the protocol raises a full block's base fee by at least 1 wei
		{base: 0, blocks: 3, want: 3},
		{base: 1_000_000_000, blocks: 5, want: 1_802_032_470},
	} {
		if got := growBaseFee(big.NewInt(tc.base), tc.blocks); got.Cmp(big.NewInt(tc.want)) != 0 {
			t.Errorf("growBaseFee(%d, %d) = %s, want %d", tc.base, tc.blocks, got, tc.want)
		}
	}
	if got := horizonMaxFee(gweiToWei(1), gweiToWei(0.02), 6); got.Cmp(big.NewInt(1_822_032_470)) != 0 {
		t.Errorf("horizonMaxFee(1 gwei, 0.02 gwei, 6) = %s, want 1822032470", got)
	}
}

func TestHorizonTip(t *testing.T) {
	policy := defaultPolicy()
	for name, tc := range map[string]struct {
		blocks []blockFee
		want   float64
	}{
		"both recent blocks had room": {
			blocks: []blockFee{block(98, 1, 0.5, 9), block(99, 1, 0.5, 9), block(100, 1, 0.93, 9)}, want: 0.02,
		},
		"latest block was full": {
			blocks: []blockFee{block(98, 1, 0.5, 9), block(99, 1, 0.5, 9), block(100, 1, 0.94, 9)}, want: 0.1,
		},
		"previous block was full": {
			blocks: []blockFee{block(98, 1, 0.5, 9), block(99, 1, 0.95, 9), block(100, 1, 0.5, 9)}, want: 0.1,
		},
		"run of full blocks follows the market": {
			blocks: []blockFee{block(98, 1, 0.5, 0.001), block(99, 1, 1, 3), block(100, 1, 0.99, 2)}, want: 3,
		},
		"market tip is capped": {
			blocks: []blockFee{block(98, 1, 0.5, 0.001), block(99, 1, 1, 30), block(100, 1, 1, 12)}, want: 15,
		},
		"market tip has a floor": {
			blocks: []blockFee{block(98, 1, 0.5, 0.001), block(99, 1, 1, 0.05), block(100, 1, 1, 0.01)}, want: 0.2,
		},
		"reward window ignores older blocks": {
			blocks: []blockFee{
				block(97, 1, 0.5, 9), block(98, 1, 0.5, 0.5), block(99, 1, 1, 1), block(100, 1, 1, 2),
			},
			want: 2,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := horizonTip(testSnapshot(1, tc.blocks...), horizonTestGas, policy); got.Cmp(gweiToWei(tc.want)) != 0 {
				t.Fatalf("horizonTip = %s, want %s", got, gweiToWei(tc.want))
			}
		})
	}
}

func TestHorizonFees(t *testing.T) {
	policy := defaultPolicy()
	roomy := testSnapshot(1, block(98, 1, 0.5, 0), block(99, 1, 0.5, 0), block(100, 1, 0.5, 0))
	for name, tc := range map[string]struct {
		limit   *big.Int
		wantTip *big.Int
		wantMax *big.Int
		wantErr string
	}{
		"exact bound for the horizon":   {wantTip: gweiToWei(0.02), wantMax: big.NewInt(1_822_032_470)},
		"limit shortens the horizon":    {limit: gweiToWei(1.5), wantTip: gweiToWei(0.02), wantMax: gweiToWei(1.5)},
		"limit trims the tip":           {limit: gweiToWei(1.01), wantTip: gweiToWei(0.01), wantMax: gweiToWei(1.01)},
		"limit below the next base fee": {limit: gweiToWei(0.9), wantErr: "fee limit reached"},
	} {
		t.Run(name, func(t *testing.T) {
			fees, err := horizonFees(roomy, horizonTestGas, tc.limit, policy)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("horizonFees error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || fees.tip.Cmp(tc.wantTip) != 0 || fees.maxFee.Cmp(tc.wantMax) != 0 ||
				fees.baseFee.Cmp(gweiToWei(1)) != 0 {
				t.Fatalf("horizonFees = %+v, %v; want tip %s max %s base 1 gwei", fees, err, tc.wantTip, tc.wantMax)
			}
		})
	}
}

func TestJudgedSentHead(t *testing.T) {
	sentAt := time.Unix(1_000, 0)
	for name, tc := range map[string]struct {
		sentHead, head uint64
		elapsed        time.Duration
		want           uint64
	}{
		"an accurate send head stands":                  {sentHead: 100, head: 101, elapsed: 12400 * time.Millisecond, want: 100},
		"a stale send head is raised to the wall clock": {sentHead: 97, head: 101, elapsed: 12400 * time.Millisecond, want: 99},
		"within the first slot one block can follow":    {sentHead: 97, head: 101, elapsed: time.Second, want: 100},
		"enough elapsed slots keep the send head":       {sentHead: 97, head: 101, elapsed: time.Minute, want: 97},
		"a clock step back counts as no time":           {sentHead: 97, head: 101, elapsed: -time.Minute, want: 100},
		"a head within the bound keeps the send head":   {sentHead: 0, head: 1, elapsed: 0, want: 0},
	} {
		t.Run(name, func(t *testing.T) {
			got := judgedSentHead(tc.sentHead, tc.head, sentAt, sentAt.Add(tc.elapsed), 12*time.Second)
			if got != tc.want {
				t.Fatalf("judged send head = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDecidePending(t *testing.T) {
	policy := defaultPolicy()
	attempt := feeQuote{baseFee: gweiToWei(1), tip: gweiToWei(0.02), maxFee: gweiToWei(1.8)}
	sent := block(100, 1, 0.5, 0.001)
	for name, tc := range map[string]struct {
		snapshot feeSnapshot
		attempt  feeQuote
		want     pendingDecision
	}{
		"no block since send": {
			snapshot: testSnapshot(1, block(98, 1, 0.5, 0), block(99, 1, 0.5, 0), sent),
			want:     pendingDecision{action: pendingHold},
		},
		"fee cap lapses within two blocks": {
			snapshot: testSnapshot(1.7, block(99, 1, 0.5, 0), sent, block(101, 1, 0.5, 0)),
			want:     pendingDecision{action: pendingReprice, reason: replaceReasonValidity},
		},
		"one full block is not a run": {
			snapshot: testSnapshot(1, block(99, 1, 0.5, 0), sent, block(101, 1, 1, 1)),
			want:     pendingDecision{action: pendingHold},
		},
		"run of full blocks escalates": {
			snapshot: testSnapshot(1, sent, block(101, 1, 1, 1), block(102, 1, 1, 1)),
			want:     pendingDecision{action: pendingReprice, reason: replaceReasonCongestion},
		},
		"run of full blocks holds a tip already above the market": {
			snapshot: testSnapshot(1, sent, block(101, 1, 1, 1), block(102, 1, 1, 1)),
			attempt:  feeQuote{baseFee: gweiToWei(1), tip: gweiToWei(2), maxFee: gweiToWei(3)},
			want:     pendingDecision{action: pendingHold},
		},
		"full blocks the attempt could not enter do not escalate": {
			snapshot: testSnapshot(1, sent, block(101, 1.9, 1, 1), block(102, 1.9, 1, 1)),
			want:     pendingDecision{action: pendingHold},
		},
		"only a trailing run escalates": {
			snapshot: testSnapshot(1, block(101, 1, 1, 1), block(102, 1, 0.5, 0), block(103, 1, 1, 1)),
			want:     pendingDecision{action: pendingHold},
		},
		"blocks with room lost while valid stall": {
			snapshot: testSnapshot(1, block(101, 1, 0.5, 0), block(102, 1, 0.5, 0), block(103, 1, 0.5, 0)),
			want:     pendingDecision{action: pendingStall, reason: replaceReasonStall},
		},
		"blocks up to the send head are not misses": {
			snapshot: testSnapshot(1, block(99, 1, 0.5, 0), sent, block(101, 1, 0.5, 0)),
			want:     pendingDecision{action: pendingHold},
		},
	} {
		t.Run(name, func(t *testing.T) {
			current := attempt
			if tc.attempt.maxFee != nil {
				current = tc.attempt
			}
			if got := decidePending(tc.snapshot, current, horizonTestGas, 100, policy); got != tc.want {
				t.Fatalf("decidePending = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseFeeSnapshot(t *testing.T) {
	valid := func() *ethereum.FeeHistory {
		return &ethereum.FeeHistory{
			OldestBlock:  big.NewInt(98),
			BaseFee:      []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4)},
			GasUsedRatio: []float64{0.1, 0.2, 1.3},
			Reward:       [][]*big.Int{{big.NewInt(5)}, {big.NewInt(6)}, {big.NewInt(7)}},
		}
	}
	snapshot, err := parseFeeSnapshot(valid(), 60_000_000)
	if err != nil {
		t.Fatalf("parseFeeSnapshot: %v", err)
	}
	if snapshot.head != 100 || snapshot.nextBaseFee.Int64() != 4 || len(snapshot.blocks) != 3 ||
		snapshot.blocks[2].number != 100 || snapshot.blocks[2].gasUsedRatio != 1 || snapshot.blocks[0].reward.Int64() != 5 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	for name, mutate := range map[string]func(*ethereum.FeeHistory){
		"no oldest block":       func(h *ethereum.FeeHistory) { h.OldestBlock = nil },
		"base fee count":        func(h *ethereum.FeeHistory) { h.BaseFee = h.BaseFee[:3] },
		"reward row count":      func(h *ethereum.FeeHistory) { h.Reward = h.Reward[:2] },
		"missing reward":        func(h *ethereum.FeeHistory) { h.Reward[1] = nil },
		"negative ratio":        func(h *ethereum.FeeHistory) { h.GasUsedRatio[0] = -0.1 },
		"missing base fee":      func(h *ethereum.FeeHistory) { h.BaseFee[1] = nil },
		"missing next base fee": func(h *ethereum.FeeHistory) { h.BaseFee[3] = nil },
		"empty history":         func(h *ethereum.FeeHistory) { h.GasUsedRatio, h.BaseFee, h.Reward = nil, h.BaseFee[:1], nil },
	} {
		t.Run(name, func(t *testing.T) {
			history := valid()
			mutate(history)
			if _, err := parseFeeSnapshot(history, 60_000_000); err == nil {
				t.Fatal("invalid fee history was accepted")
			}
		})
	}
}

type testRPCError struct {
	code    int
	message string
}

func (e testRPCError) Error() string  { return e.message }
func (e testRPCError) ErrorCode() int { return e.code }

func TestIsBlockOverrideUnsupported(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{err: testRPCError{code: -32602, message: "invalid params"}, want: true},
		{err: errors.Errorf("estimate: %w", testRPCError{code: -32602, message: "bad"}), want: true},
		{err: errors.New("too many arguments, want at most 3"), want: true},
		{err: testRPCError{code: 3, message: "execution reverted: NonceUsed"}, want: false},
		{err: errors.New("execution reverted"), want: false},
		{err: errors.New("dial tcp: connection refused"), want: false},
	} {
		if got := isBlockOverrideUnsupported(tc.err); got != tc.want {
			t.Errorf("isBlockOverrideUnsupported(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestWithGasHeadroom(t *testing.T) {
	for _, tc := range []struct {
		gas  uint64
		bps  int
		want uint64
	}{
		{gas: 100_000, bps: 500, want: 105_000},
		{gas: 3_674_547, bps: 800, want: 3_968_510},
		{gas: 0, bps: 500, want: 0},
	} {
		if got := withGasHeadroom(tc.gas, tc.bps); got != tc.want {
			t.Errorf("withGasHeadroom(%d, %d) = %d, want %d", tc.gas, tc.bps, got, tc.want)
		}
	}
}

func TestReplacementFeesPreserveGrowthAtCeiling(t *testing.T) {
	previous := feeQuote{baseFee: big.NewInt(100), tip: big.NewInt(10), maxFee: big.NewInt(200)}
	for _, tc := range []struct {
		limit   int64
		wantErr bool
	}{{225, false}, {224, true}} {
		got, err := replacementFees(previous, nil, big.NewInt(tc.limit))
		if (err != nil) != tc.wantErr {
			t.Fatalf("limit=%d err=%v", tc.limit, err)
		}
		if err == nil && (got.tip.Int64() != 12 || got.maxFee.Int64() != 225) {
			t.Fatalf("replacement=%+v", got)
		}
	}
}
func TestGasHeadroomDoesNotOverflowIntermediateProduct(t *testing.T) {
	gas := uint64(20_000_000_000_000_000)
	want := uint64(22_000_000_000_000_000)
	if got := withGasHeadroom(gas, 1000); got != want {
		t.Fatalf("headroom=%d want=%d", got, want)
	}
}

func policyConfig() HorizonConfig {
	return HorizonConfig{MaxBlocks: 6, BlockTime: 12 * time.Second, TipFloorGwei: .02, FullBlockTipGwei: .1, CongestedTipFloorGwei: .2, CongestedTipCapGwei: 15, CongestedRewardBlocks: 3, CongestedRewardPercentile: 50, EscalateAfterFullBlocks: 2, StallAfterBlocks: 3, GasHeadroomBps: 500, FallbackGasHeadroomBps: 1000}
}
func defaultPolicy() horizonPolicy { return newHorizonPolicy(policyConfig()) }
func TestExplicitPolicyConfigurationRequired(t *testing.T) {
	if err := config().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Config){"zero timing": func(c *Config) { c.ReadTimeout = 0 }, "zero confirmations": func(c *Config) { c.Confirmations = 0 }, "zero horizon": func(c *Config) { c.Horizon = HorizonConfig{} }, "inverted tips": func(c *Config) { c.Horizon.TipFloorGwei = 100 }, "invalid reward percentile": func(c *Config) { c.Horizon.CongestedRewardPercentile = 101 }, "negative gas headroom": func(c *Config) { c.Horizon.GasHeadroomBps = -1 }, "fee cap too low": func(c *Config) { c.MaxFeeGwei = .1 }} {
		t.Run(name, func(t *testing.T) {
			c := config()
			mutate(&c)
			if err := c.Validate(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestSimulationPolicyDoesNotRequireSpendingLimit(t *testing.T) {
	c := config()
	c.MaxFeeGwei = 0
	if err := c.ValidatePolicy(); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err == nil {
		t.Fatal("sending accepted absent spending ceiling")
	}
}
