package gnarkadapter

import (
	"math/big"

	"fishbone-data-trade-zk/internal/business"

	"gnarkabc/gnarkwrapper"
	"gnarkabc/hash/mimchash"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// BusinessRangeProof is the Stage 2.2 circuit that proves:
//  1. raw_value in [min_value, max_value]
//  2. masked_value = raw_value + mask_delta
//  3. masked_value_hash = MiMC(masked_value, salt)
//
// Public inputs: MaskedValue, MaskedValueHash, Min, Max
// Private inputs: RawValue, MaskDelta, Salt
type BusinessRangeProof struct {
	RawValue        frontend.Variable
	MaskDelta       frontend.Variable
	MaskedValue     frontend.Variable `gnark:",public"`
	MaskedValueHash frontend.Variable `gnark:",public"`
	Min             frontend.Variable `gnark:",public"`
	Max             frontend.Variable `gnark:",public"`
	Salt            frontend.Variable
}

func (c *BusinessRangeProof) Define(api frontend.API) error {
	// 1. masked_value = raw_value + mask_delta
	masked := api.Add(c.RawValue, c.MaskDelta)
	api.AssertIsEqual(masked, c.MaskedValue)

	// 2. raw_value >= min_value
	api.AssertIsLessOrEqual(c.Min, c.RawValue)
	// 3. raw_value <= max_value
	api.AssertIsLessOrEqual(c.RawValue, c.Max)

	// 4. masked_value_hash = MiMC(masked_value, salt)
	m, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	m.Reset()
	m.Write(c.MaskedValue)
	m.Write(c.Salt)
	hash := m.Sum()
	api.AssertIsEqual(hash, c.MaskedValueHash)

	return nil
}

func (c *BusinessRangeProof) PreCompile(params ...interface{}) {}

func (c *BusinessRangeProof) Assign(curveName string, w business.RangeWitness) {
	mod := gnarkwrapper.CurveMap[curveName].ScalarField()

	rawBig := new(big.Int).SetUint64(w.RawValue)
	minBig := new(big.Int).SetUint64(w.MinValue)
	maxBig := new(big.Int).SetUint64(w.MaxValue)
	deltaBig := new(big.Int).SetUint64(w.MaskDelta)
	maskedBig := new(big.Int).Add(rawBig, deltaBig)

	// Salt: convert hex to bytes mod the field
	saltBytes := mimchash.Convert2Byte(w.SaltHex, mod)

	c.RawValue = rawBig
	c.Min = minBig
	c.Max = maxBig
	c.MaskDelta = deltaBig
	c.MaskedValue = maskedBig
	c.Salt = saltBytes

	// MiMC(masked_value, salt)
	hashFunc := mimchash.MiMCCaseMap[curveName].Hash
	maskedBytes := maskedBig.Bytes()
	c.MaskedValueHash = mimchash.MiMCHash(hashFunc, [][]byte{maskedBytes, saltBytes})
}
