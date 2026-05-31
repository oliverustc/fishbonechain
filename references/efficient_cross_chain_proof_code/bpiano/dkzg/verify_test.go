package dkzg

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// TestVerifyFullRoundtrip 是 DKZG 方案的端到端测试。
//
// 完整执行以下协议流程：
//  1. 用固定陷门生成 SRS。
//  2. 为 M 个子多项式创建求值数据。
//  3. 计算全局承诺。
//  4. 各子节点在 alpha 处进行 X 轴开放。
//  5. 主节点聚合 X 轴证明。
//  6. 主节点在 beta 处打开 V_F（Y 轴开放）。
//  7. 验证方检查组合证明。
func TestVerifyFullRoundtrip(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(4)
	T := uint64(8)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	// 步骤 1：创建求值数据 f_i(ω_X^j) = i*T + j + 1。
	allEvals := make([][]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		allEvals[i] = make([]fr.Element, T)
		for j := uint64(0); j < T; j++ {
			allEvals[i][j].SetUint64(i*T + j + 1)
		}
	}

	// 步骤 2：计算全局承诺。
	comF, err := CommitGlobal(allEvals, srs)
	if err != nil {
		t.Fatalf("CommitGlobal: %v", err)
	}

	// 步骤 3：在 alpha 处进行 X 轴开放。
	var alpha fr.Element
	alpha.SetUint64(12345)

	localProofs := make([]OpeningProofX, M)
	for i := uint64(0); i < M; i++ {
		localProofs[i], err = LocalOpenX(i, allEvals[i], alpha, srs)
		if err != nil {
			t.Fatalf("LocalOpenX(%d): %v", i, err)
		}
	}

	// 步骤 4：主节点聚合 X 轴证明。
	comVF, piXAgg, err := AggregateOpenX(localProofs, srs)
	if err != nil {
		t.Fatalf("AggregateOpenX: %v", err)
	}

	// 步骤 5：收集 v_i 值并在 beta 处进行 Y 轴开放。
	var beta fr.Element
	beta.SetUint64(99999)

	viEvals := make([]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		viEvals[i] = localProofs[i].ClaimedValue
	}

	proofY, err := OpenY(viEvals, beta, srs)
	if err != nil {
		t.Fatalf("OpenY: %v", err)
	}

	// 验证聚合结果与 OpenY 中的 com_VF 一致。
	if !comVF.Equal(&proofY.ComVF) {
		t.Fatal("AggregateOpenX 与 OpenY 的 com_VF 不一致")
	}

	// 步骤 6：验证。
	if err := Verify(comF, alpha, beta, piXAgg, proofY, srs); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

// TestVerifySmall 是 M=2，T=4 的最小端到端测试。
func TestVerifySmall(t *testing.T) {
	tauX := big.NewInt(3)
	tauY := big.NewInt(11)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	// 简单求值：f_0 = [1,2,3,4]，f_1 = [5,6,7,8]。
	allEvals := [][]fr.Element{
		makeEvals(T, 1),
		makeEvals(T, 5),
	}

	comF, err := CommitGlobal(allEvals, srs)
	if err != nil {
		t.Fatalf("CommitGlobal: %v", err)
	}

	var alpha fr.Element
	alpha.SetUint64(100)

	localProofs := make([]OpeningProofX, M)
	for i := uint64(0); i < M; i++ {
		localProofs[i], err = LocalOpenX(i, allEvals[i], alpha, srs)
		if err != nil {
			t.Fatalf("LocalOpenX(%d): %v", i, err)
		}
	}

	_, piXAgg, err := AggregateOpenX(localProofs, srs)
	if err != nil {
		t.Fatalf("AggregateOpenX: %v", err)
	}

	viEvals := make([]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		viEvals[i] = localProofs[i].ClaimedValue
	}

	var beta fr.Element
	beta.SetUint64(200)

	proofY, err := OpenY(viEvals, beta, srs)
	if err != nil {
		t.Fatalf("OpenY: %v", err)
	}

	if err := Verify(comF, alpha, beta, piXAgg, proofY, srs); err != nil {
		t.Fatalf("小规模测试 Verify 失败: %v", err)
	}
}

// TestVerifyRejectsWrongZ 验证 Verify 能拒绝被篡改的声明值 z。
func TestVerifyRejectsWrongZ(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	allEvals := [][]fr.Element{
		makeEvals(T, 1),
		makeEvals(T, 3),
	}

	comF, err := CommitGlobal(allEvals, srs)
	if err != nil {
		t.Fatalf("CommitGlobal: %v", err)
	}

	var alpha fr.Element
	alpha.SetUint64(77)

	localProofs := make([]OpeningProofX, M)
	for i := uint64(0); i < M; i++ {
		localProofs[i], err = LocalOpenX(i, allEvals[i], alpha, srs)
		if err != nil {
			t.Fatalf("LocalOpenX: %v", err)
		}
	}

	_, piXAgg, err := AggregateOpenX(localProofs, srs)
	if err != nil {
		t.Fatalf("AggregateOpenX: %v", err)
	}

	viEvals := make([]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		viEvals[i] = localProofs[i].ClaimedValue
	}

	var beta fr.Element
	beta.SetUint64(88)

	proofY, err := OpenY(viEvals, beta, srs)
	if err != nil {
		t.Fatalf("OpenY: %v", err)
	}

	// 篡改 z。
	proofY.ClaimedValue.Add(&proofY.ClaimedValue, new(fr.Element).SetOne())

	if err := Verify(comF, alpha, beta, piXAgg, proofY, srs); err == nil {
		t.Fatal("Verify 应拒绝篡改后的 z，但返回 nil")
	}
}

// TestVerifyRejectsWrongComF 验证 Verify 能拒绝错误的承诺 com_F。
func TestVerifyRejectsWrongComF(t *testing.T) {
	tauX := big.NewInt(5)
	tauY := big.NewInt(7)
	M := uint64(2)
	T := uint64(4)

	srs, err := NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		t.Fatalf("NewTestSRS: %v", err)
	}

	allEvals := [][]fr.Element{
		makeEvals(T, 2),
		makeEvals(T, 4),
	}

	comF, err := CommitGlobal(allEvals, srs)
	if err != nil {
		t.Fatalf("CommitGlobal: %v", err)
	}

	var alpha fr.Element
	alpha.SetUint64(55)

	localProofs := make([]OpeningProofX, M)
	for i := uint64(0); i < M; i++ {
		localProofs[i], err = LocalOpenX(i, allEvals[i], alpha, srs)
		if err != nil {
			t.Fatalf("LocalOpenX: %v", err)
		}
	}

	_, piXAgg, err := AggregateOpenX(localProofs, srs)
	if err != nil {
		t.Fatalf("AggregateOpenX: %v", err)
	}

	viEvals := make([]fr.Element, M)
	for i := uint64(0); i < M; i++ {
		viEvals[i] = localProofs[i].ClaimedValue
	}

	var beta fr.Element
	beta.SetUint64(66)

	proofY, err := OpenY(viEvals, beta, srs)
	if err != nil {
		t.Fatalf("OpenY: %v", err)
	}

	// 篡改 com_F：取反。
	comF.Neg(&comF)

	if err := Verify(comF, alpha, beta, piXAgg, proofY, srs); err == nil {
		t.Fatal("Verify 应拒绝错误的 comF，但返回 nil")
	}
}

// makeEvals 创建长度为 T、从 startVal 开始递增的域元素切片。
func makeEvals(T uint64, startVal uint64) []fr.Element {
	evals := make([]fr.Element, T)
	for j := uint64(0); j < T; j++ {
		evals[j].SetUint64(startVal + j)
	}
	return evals
}
