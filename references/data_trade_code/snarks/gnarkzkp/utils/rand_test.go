package utils

import (
	"gnarkabc/logger"
	"hash"
	"testing"

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
}

func TestRandStr(t *testing.T) {
	for i := 0; i < 10; i++ {
		randStr := RandStr(32)
		t.Log(randStr)
	}
}

func TestRandInt(t *testing.T) {
	for i := 0; i < 10; i++ {
		randInt := RandInt(1, 100)
		t.Log(randInt)
	}
}

func TestRandItem(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	for i := 0; i < 10; i++ {
		randItem := RandItem(items)
		t.Log(randItem)
	}
}
func TestRandBigInt(t *testing.T) {
	for fieldName := range MiMCCaseMap {
		logger.Info("curve: %s", fieldName)
		field := MiMCCaseMap[fieldName].Curve.ScalarField()
		logger.Info("randBigInt: %x", RandBigInt(field))
	}
}
