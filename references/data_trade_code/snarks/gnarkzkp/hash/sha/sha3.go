package sha

import (
	"hash"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	zkhash "github.com/consensys/gnark/std/hash"
	zksha3 "github.com/consensys/gnark/std/hash/sha3"
	"github.com/consensys/gnark/std/math/uints"
	"golang.org/x/crypto/sha3"
)

type HashCase struct {
	ZK     func(api frontend.API) (zkhash.BinaryHasher, error)
	Native func() hash.Hash
}

var HashCaseMap = map[string]HashCase{
	"SHA3-256":   {zksha3.New256, sha3.New256},
	"SHA3-384":   {zksha3.New384, sha3.New384},
	"SHA3-512":   {zksha3.New512, sha3.New512},
	"Keccak-256": {zksha3.NewLegacyKeccak256, sha3.NewLegacyKeccak256},
	"Keccak-512": {zksha3.NewLegacyKeccak512, sha3.NewLegacyKeccak512},
}

var Sha3ScalarFieldMap = map[string]*big.Int{
	"BN254":     ecc.BN254.ScalarField(),
	"BLS12-377": ecc.BLS12_377.ScalarField(),
	"BLS12-381": ecc.BLS12_381.ScalarField(),
	"BLS24-315": ecc.BLS24_315.ScalarField(),
	"BLS24-317": ecc.BLS24_317.ScalarField(),
	"BW6-633":   ecc.BW6_633.ScalarField(),
	"BW6-761":   ecc.BW6_761.ScalarField(),
}

func CalcSha3(preImage string, zkSha3Name string) ([]uints.U8, []uints.U8) {
	preImageBytes := []byte(preImage)
	preImageU8 := uints.NewU8Array(preImageBytes)
	hashStrategy := HashCaseMap[zkSha3Name]
	hashFunc := hashStrategy.Native()

	hashFunc.Reset()
	hashFunc.Write(preImageBytes)
	hashBytes := hashFunc.Sum(nil)
	hashU8 := uints.NewU8Array(hashBytes)
	return preImageU8, hashU8
}
