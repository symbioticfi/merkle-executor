package transaction

import (
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/params"
	"github.com/go-errors/errors"
)

const basisPoints = 10_000

func positiveGwei(gwei float64) bool {
	return !math.IsNaN(gwei) && !math.IsInf(gwei, 0) && gwei > 0 && gweiToWei(gwei).Sign() > 0 && gweiToWei(gwei).BitLen() <= 256
}

func gweiToWei(gwei float64) *big.Int {
	wei, _ := new(big.Float).Mul(big.NewFloat(gwei), big.NewFloat(params.GWei)).Int(nil)
	return wei
}

func withGasHeadroom(gas uint64, bps int) (uint64, error) {
	extra := gas/basisPoints*uint64(bps) + gas%basisPoints*uint64(bps)/basisPoints
	if gas == 0 || extra > math.MaxUint64-gas {
		return 0, errors.New("invalid gas estimate")
	}
	return gas + extra, nil
}
