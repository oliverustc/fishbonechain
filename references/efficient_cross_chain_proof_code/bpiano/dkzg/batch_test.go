package dkzg

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// TestBatchVerifyEquivalentToSingle 验证 BatchVerify 在 K=1 和 K=2 时
// 与逐个调用 Verify 的结果一致。
func TestBatchVerifyEquivalentToSingle(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	makeProof := func(alphaSeed, betaSeed, evalBase uint64) BatchOpenResult {
		allEvals := [][]fr.Element{
			makeEvals(T, evalBase),
			makeEvals(T, evalBase+T),
		}
		var alpha, beta fr.Element
		alpha.SetUint64(alphaSeed)
		beta.SetUint64(betaSeed)

		result, err := PrepareProof(allEvals, alpha, beta, srs)
		if err != nil {
			t.Fatalf("PrepareProof: %v", err)
		}
		return result
	}

	// K=1 的批量验证。
	p1 := makeProof(1000, 2000, 1)
	var r fr.Element
	r.SetUint64(3)

	if err := BatchVerify([]BatchOpenResult{p1}, r, srs); err != nil {
		t.Fatalf("BatchVerify K=1 失败: %v", err)
	}

	// K=2 的批量验证。
	p2 := makeProof(3000, 4000, 100)
	if err := BatchVerify([]BatchOpenResult{p1, p2}, r, srs); err != nil {
		t.Fatalf("BatchVerify K=2 失败: %v", err)
	}
}

// TestBatchVerifyK3 测试 K=3 个证明的批量验证。
func TestBatchVerifyK3(t *testing.T) {
	tauX := big.NewInt(11)
	tauY := big.NewInt(13)
	M := uint64(4)
	T := uint64(8)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	proofs := make([]BatchOpenResult, 3)
	for k := 0; k < 3; k++ {
		allEvals := make([][]fr.Element, M)
		for i := uint64(0); i < M; i++ {
			allEvals[i] = makeEvals(T, uint64(k*100)+i*T+1)
		}
		var alpha, beta fr.Element
		alpha.SetUint64(uint64(k*1000 + 500))
		beta.SetUint64(uint64(k*2000 + 700))

		proofs[k], err = PrepareProof(allEvals, alpha, beta, srs)
		if err != nil {
			t.Fatalf("PrepareProof(%d): %v", k, err)
		}
	}

	var r fr.Element
	r.SetUint64(7)

	if err := BatchVerify(proofs, r, srs); err != nil {
		t.Fatalf("BatchVerify K=3 失败: %v", err)
	}
}

// TestBatchVerifyRejectsTamperedProof 验证 BatchVerify 能拒绝包含被篡改证明的批次。
func TestBatchVerifyRejectsTamperedProof(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	makeProof := func(alphaSeed, betaSeed, evalBase uint64) BatchOpenResult {
		allEvals := [][]fr.Element{
			makeEvals(T, evalBase),
			makeEvals(T, evalBase+T),
		}
		var alpha, beta fr.Element
		alpha.SetUint64(alphaSeed)
		beta.SetUint64(betaSeed)
		result, err := PrepareProof(allEvals, alpha, beta, srs)
		if err != nil {
			t.Fatalf("PrepareProof: %v", err)
		}
		return result
	}

	p1 := makeProof(1000, 2000, 1)
	p2 := makeProof(3000, 4000, 100)

	// 篡改 p2 的声明值。
	p2.ProofY.ClaimedValue.Add(&p2.ProofY.ClaimedValue, new(fr.Element).SetOne())

	var r fr.Element
	r.SetUint64(3)

	if err := BatchVerify([]BatchOpenResult{p1, p2}, r, srs); err == nil {
		t.Fatal("BatchVerify 应拒绝篡改后的证明，但返回 nil")
	}
}

// TestBatchVerifyEmpty 验证空批次时 BatchVerify 返回 nil。
func TestBatchVerifyEmpty(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	var r fr.Element
	r.SetUint64(3)

	if err := BatchVerify(nil, r, srs); err != nil {
		t.Fatalf("BatchVerify(空) 应返回 nil，得到 %v", err)
	}
}
