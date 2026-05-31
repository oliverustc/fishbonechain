package dkzg

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// Digest 是对双变量多项式 F(Y,X) 的承诺，是 G1 中的一个元素。
type Digest = bn254.G1Affine

// CommitLocal 计算子节点 i 对其多项式 f_i(X) 的本地承诺。
//
// 多项式 f_i 以 Lagrange（求值）形式给出：
//
//	evals[j] = f_i(ω_X^j)，j = 0,...,T-1
//
// 本地承诺为：
//
//	com_i = Σ_{j=0}^{T-1} evals[j] · Ux[nodeIdx][j]
//
// 由 SRS 定义可知，该值等于 g^{R_{nodeIdx}(τ_Y) · f_i(τ_X)}。
func CommitLocal(nodeIdx uint64, evals []fr.Element, srs *SRS) (Digest, error) {
	M := uint64(len(srs.Ux))
	if nodeIdx >= M {
		return Digest{}, ErrInvalidNodeIndex
	}
	T := uint64(len(srs.Ux[nodeIdx]))
	if uint64(len(evals)) != T {
		return Digest{}, ErrInvalidPolySize
	}

	var res Digest
	if _, err := res.MultiExp(srs.Ux[nodeIdx], evals, ecc.MultiExpConfig{}); err != nil {
		return Digest{}, err
	}
	return res, nil
}

// AggregateDigests 将 M 个本地承诺聚合为对 F(Y,X) 的全局承诺。
//
// 全局承诺为：
//
//	com_F = Σ_{i=0}^{M-1} com_i
//
// 因为 com_i = g^{R_i(τ_Y) · f_i(τ_X)}，求和后得到：
//
//	g^{Σ_i R_i(τ_Y) · f_i(τ_X)} = g^{F(τ_Y, τ_X)}
//
// 即对双变量多项式 F 在 (τ_Y, τ_X) 处的 KZG 承诺。
func AggregateDigests(localDigests []Digest) (Digest, error) {
	if len(localDigests) == 0 {
		return Digest{}, ErrMismatchedInputs
	}

	var sum bn254.G1Jac
	for i := range localDigests {
		var pt bn254.G1Jac
		pt.FromAffine(&localDigests[i])
		sum.AddAssign(&pt)
	}

	var result Digest
	result.FromJacobian(&sum)
	return result, nil
}

// CommitGlobal 是一个便捷函数，一次性对全部 M 个子多项式进行承诺，
// 直接计算 F(Y,X) 的全局承诺。
//
// allEvals[i][j] = f_i(ω_X^j)，i ∈ [0,M)，j ∈ [0,T)。
func CommitGlobal(allEvals [][]fr.Element, srs *SRS) (Digest, error) {
	M := uint64(len(srs.Ux))
	if uint64(len(allEvals)) != M {
		return Digest{}, ErrInvalidEvalsSize
	}

	localDigests := make([]Digest, M)
	for i := uint64(0); i < M; i++ {
		var err error
		localDigests[i], err = CommitLocal(i, allEvals[i], srs)
		if err != nil {
			return Digest{}, err
		}
	}

	return AggregateDigests(localDigests)
}
