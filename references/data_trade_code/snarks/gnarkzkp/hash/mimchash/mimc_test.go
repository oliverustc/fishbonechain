package mimchash

import (
	"gnarkabc/logger"
	"strings"
	"testing"
)

func TestConvert2Byte(t *testing.T) {
	input := strings.Repeat("helloworld", 10)
	logger.Info("input: %s", input)
	for curveName := range MiMCCaseMap {
		mod := MiMCCaseMap[curveName].Curve.ScalarField()
		data := Convert2Byte(input, mod)
		logger.Info("curveName: %s, data: %x", curveName, data)
	}
}

func TestMiMCHash(t *testing.T) {
	input := strings.Repeat("helloworld", 10)
	logger.Info("input: %s", input)
	for curveName := range MiMCCaseMap {
		hashFunc := MiMCCaseMap[curveName].Hash
		mod := MiMCCaseMap[curveName].Curve.ScalarField()
		data := Convert2Byte(input, mod)
		expectedHash := MiMCHash(hashFunc, [][]byte{data})
		logger.Info("curveName: %s, expectedHash: %x", curveName, expectedHash)
		logger.Info("hash length: %d", len(expectedHash))
	}
}
