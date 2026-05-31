package sha

import (
	"crypto/sha256"

	"github.com/consensys/gnark/std/math/uints"
)

func CalcSha256(preImage string) ([]uints.U8, [32]uints.U8) {
	preImageBytes := []byte(preImage)
	preImageU8 := uints.NewU8Array(preImageBytes)
	hash := sha256.Sum256(preImageBytes)
	hashU8 := uints.NewU8Array(hash[:])
	hashU8Arr := [32]uints.U8(hashU8)
	return preImageU8, hashU8Arr
}
