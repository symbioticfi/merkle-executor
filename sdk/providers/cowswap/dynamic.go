package cowswap

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
)

// PolicyVersion identifies the pinned CoW SDK AUTO policy in strategy revisions.
const PolicyVersion = "cow-sdk-auto-9ce323316b01-v1"

const (
	DefaultValiditySeconds uint64 = 1800
	defaultSlippageBPS     uint64 = 50
	settlementAddress             = "0x9008D19f58AAbD9eD0D60971565AA8510560ab41"
	relayerAddress                = "0xC92E8bdf79f0507f65a392b0ab4667716BFE0110"
	volatilityURL                 = "https://bff.cow.fi"
)

// ChainDefaults returns reviewed production deployments. Unknown EVM chains fail
// instead of using an unrelated chain's orderbook.
func ChainDefaults(chainID *big.Int) (protocolio.HTTPConfig, common.Address, common.Address, error) {
	if chainID == nil || !chainID.IsUint64() {
		return protocolio.HTTPConfig{}, common.Address{}, common.Address{}, errors.New("unsupported CoW chain")
	}
	slugs := map[uint64]string{1: "mainnet", 56: "bnb", 100: "xdai", 137: "polygon", 8453: "base", 9745: "plasma", 42161: "arbitrum_one", 43114: "avalanche", 57073: "ink", 59144: "linea", 11155111: "sepolia"}
	slug, ok := slugs[chainID.Uint64()]
	if !ok {
		return protocolio.HTTPConfig{}, common.Address{}, common.Address{}, errors.New("unsupported CoW chain")
	}
	return protocolio.HTTPConfig{URL: "https://api.cow.fi/" + slug}, common.HexToAddress(settlementAddress), common.HexToAddress(relayerAddress), nil
}

func volumeSuggestion(c *protocolio.Client) func(context.Context, *big.Int, common.Address, common.Address) (uint64, error) {
	return func(ctx context.Context, chainID *big.Int, sell, buy common.Address) (uint64, error) {
		var response wire.CowSlippageResponse
		_, err := c.JSON(ctx, http.MethodGet, "/"+chainID.String()+"/markets/"+sell.Hex()+"-"+buy.Hex()+"/slippageTolerance", nil, &response)
		if err != nil {
			return 0, err
		}
		if response.SlippageBPS < 0 || response.SlippageBPS > 10000 {
			return 0, errors.New("invalid CoW volatility slippage")
		}
		return uint64(response.SlippageBPS), nil
	}
}

// applyPercentage deliberately preserves the upstream JavaScript operation
// order, including binary64 coefficient rounding and truncating bigint division.
func applyPercentage(amount *big.Int, percent float64) *big.Int {
	coefficient := int64(math.Floor(percent * 1000000))
	intermediate := new(big.Int).Quo(new(big.Int).Mul(amount, big.NewInt(coefficient)), big.NewInt(1000000))
	result, remainder := new(big.Int), new(big.Int)
	result.QuoRem(intermediate, big.NewInt(100), remainder)
	if remainder.Cmp(big.NewInt(50)) >= 0 {
		result.Add(result, big.NewInt(1))
	}
	return result
}

func autoSlippageBPS(netSell, fee *big.Int, volumeBPS uint64) uint64 {
	volumePercent := 0.5
	if volumeBPS != 0 {
		volumePercent = float64(volumeBPS) / 100
	}
	total := new(big.Int).Add(applyPercentage(fee, 50), applyPercentage(netSell, volumePercent))
	scale := new(big.Int).Quo(new(big.Int).Mul(big.NewInt(1000000), new(big.Int).Sub(netSell, total)), netSell)
	scale.Sub(big.NewInt(1000000), scale)
	// Only values below the 100% cap need converting to binary64.
	if scale.Cmp(big.NewInt(1000000)) >= 0 {
		return 10000
	}
	percent := float64(scale.Int64()) / 1000000
	bps := math.Round(percent * 10000)
	if bps <= 0 {
		return defaultSlippageBPS
	}
	return uint64(bps)
}

func buyAfterSlippage(buy *big.Int, bps uint64) *big.Int {
	deduction := new(big.Int).Quo(new(big.Int).Mul(buy, new(big.Int).SetUint64(bps)), big.NewInt(10000))
	return new(big.Int).Sub(buy, deduction)
}

func appData(bps uint64) (string, common.Hash) {
	// Struct field order makes this exact document reproducible across restarts.
	document := struct {
		AppCode  string `json:"appCode"`
		Version  string `json:"version"`
		Metadata struct {
			OrderClass struct {
				OrderClass string `json:"orderClass"`
			} `json:"orderClass"`
			Quote struct {
				SlippageBips  uint64 `json:"slippageBips"`
				SmartSlippage bool   `json:"smartSlippage"`
			} `json:"quote"`
		} `json:"metadata"`
	}{AppCode: "merkle-executor", Version: "1.15.0"}
	document.Metadata.OrderClass.OrderClass = "market"
	document.Metadata.Quote.SlippageBips = bps
	document.Metadata.Quote.SmartSlippage = true
	raw, _ := json.Marshal(document)
	return string(raw), crypto.Keccak256Hash(raw)
}
