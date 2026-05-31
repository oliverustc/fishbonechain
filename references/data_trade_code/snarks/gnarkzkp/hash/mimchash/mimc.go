package mimchash

import (
	"gnarkabc/logger"
	"hash"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	gchash "github.com/consensys/gnark-crypto/hash"
)

type MiMCCase struct {
	Curve ecc.ID
	Hash  hash.Hash
}

var MiMCCaseMap = map[string]MiMCCase{
	"BN254":     {ecc.BN254, gchash.MIMC_BN254.New()},
	"BLS12-377": {ecc.BLS12_377, gchash.MIMC_BLS12_377.New()},
	"BLS12-381": {ecc.BLS12_381, gchash.MIMC_BLS12_381.New()},
	"BLS24-315": {ecc.BLS24_315, gchash.MIMC_BLS24_315.New()},
	"BLS24-317": {ecc.BLS24_317, gchash.MIMC_BLS24_317.New()},
	"BW6-633":   {ecc.BW6_633, gchash.MIMC_BW6_633.New()},
	"BW6-761":   {ecc.BW6_761, gchash.MIMC_BW6_761.New()},
}

func Convert2Byte(input string, mod *big.Int) []byte {
	logger.Debug("input: %s", input)
	inputBytes := []byte(input)
	logger.Debug("inputBytes: %x", inputBytes)
	inputBigInt := new(big.Int).SetBytes(inputBytes)
	logger.Debug("inputBigInt: %v", inputBigInt)
	if inputBigInt.Cmp(mod) >= 0 {
		logger.Debug("inputBigInt >= mod, %v", inputBigInt)
		inputBigInt.Mod(inputBigInt, mod)
		logger.Debug("after mod, inputBigInt: %v", inputBigInt)
	}
	return inputBigInt.Bytes()
}

func MiMCHash(h hash.Hash, data [][]byte) []byte {
	h.Reset()
	for _, d := range data {
		h.Write(d)
	}
	return h.Sum(nil)
}
