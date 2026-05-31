package sha

import (
	"gnarkabc/logger"
	"testing"
)

func TestCalcSha3(t *testing.T) {
	preImage := "hello"
	for zkSha3Name := range HashCaseMap {
		preImageU8, hashU8 := CalcSha3(preImage, zkSha3Name)

		logger.Info("zkSha3Name: %v", zkSha3Name)
		logger.Info("preImageU8: %v", preImageU8)
		logger.Info("hashU8: %v", hashU8)
		hashLen := HashCaseMap[zkSha3Name].Native().Size()
		logger.Info("hashLen: %v", hashLen)
	}
}
