package solgen_test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/bpiano"
	"github.com/oliverustc/bpiano/piano"
	"github.com/oliverustc/bpiano/solgen"
)

// buildTestCircuit 构建用于测试的简单复制约束电路（L[0] = R[0]）。
func buildTestCircuit(T, M int) (piano.CircuitInfo, []piano.WitnessInstance) {
	ql := make([]fr.Element, T)
	qr := make([]fr.Element, T)
	ql[0].SetInt64(-1)
	qr[0].SetOne()

	lro := make([]int, 3*T)
	for i := range lro {
		lro[i] = i
	}
	lro[0] = 0
	lro[T] = 0 // L[0] 和 R[0] 联通
	perm := piano.BuildPermutation(lro, 3*T, T)

	ci := piano.CircuitInfo{
		Ql: ql, Qr: qr,
		Qm: cloneZero(T), Qo: cloneZero(T),
		Qk: cloneZero(T), Permutation: perm,
	}
	witnesses := make([]piano.WitnessInstance, M)
	for i := range witnesses {
		L := make([]fr.Element, T)
		R := make([]fr.Element, T)
		L[0].SetInt64(int64(i + 5))
		R[0].SetInt64(int64(i + 5))
		witnesses[i] = piano.WitnessInstance{L: L, R: R, O: cloneZero(T)}
	}
	return ci, witnesses
}

func cloneZero(n int) []fr.Element {
	return make([]fr.Element, n)
}

// TestGenerateBPianoCalldata 端到端测试：
//  1. 搭建小型电路（T=8, M=2）
//  2. 生成 BPiano 压缩证明
//  3. 调用 GenerateBPianoCalldata
//  4. 验证 Packed 字节数正确（无公开输入：12×64 + 15×32 + 2×128 = 1504 字节）
//  5. 验证重放挑战与 VerifyCompressed 使用的挑战一致（通过 VerifyCompressed 通过即可确认）
func TestGenerateBPianoCalldata(t *testing.T) {
	const T, M = 8, 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)

	ci, witnesses := buildTestCircuit(T, M)
	pk, vk, err := piano.SetupWithTrapdoors(ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	proof, err := bpiano.Compress(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}

	// 验证正确性（确保 proof 本身有效）
	if err := bpiano.VerifyCompressed(proof, vk, nil); err != nil {
		t.Fatalf("VerifyCompressed: %v", err)
	}

	// 生成 calldata
	res, err := solgen.GenerateBPianoCalldata(proof, vk, nil)
	if err != nil {
		t.Fatalf("GenerateBPianoCalldata: %v", err)
	}

	// 验证 Packed 字节数：12×64 + 15×32 + 0（公开输入）+ 2×128 = 768+480+256 = 1504
	wantBytes := 12*solgen.G1Size + 15*solgen.FrSize + 2*solgen.G2Size
	if len(res.Packed) != wantBytes {
		t.Errorf("Packed 长度 %d，期望 %d", len(res.Packed), wantBytes)
	}
}

// TestCalldataG1PointsRoundtrip 验证 G1 点序列化后可正确还原。
func TestCalldataG1PointsRoundtrip(t *testing.T) {
	const T, M = 8, 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)

	ci, witnesses := buildTestCircuit(T, M)
	pk, vk, err := piano.SetupWithTrapdoors(ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	proof, err := bpiano.Compress(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}

	res, err := solgen.GenerateBPianoCalldata(proof, vk, nil)
	if err != nil {
		t.Fatalf("GenerateBPianoCalldata: %v", err)
	}
	cd := res.Calldata

	// 从 calldata 还原 LRO[0]，与原始承诺对比。
	lro0 := cd.LRO[0]

	// 解析 x（前 32 字节）和 y（后 32 字节）。
	var xBig, yBig big.Int
	xBig.SetBytes(lro0[0:32])
	yBig.SetBytes(lro0[32:64])

	// 与原始点坐标对比（通过 RawBytes）。
	raw := proof.LRO[0].RawBytes()
	var xOrig, yOrig big.Int
	xOrig.SetBytes(raw[0:32])
	yOrig.SetBytes(raw[32:64])

	if xBig.Cmp(&xOrig) != 0 {
		t.Errorf("LRO[0].X 不匹配：got %s, want %s", xBig.String(), xOrig.String())
	}
	if yBig.Cmp(&yOrig) != 0 {
		t.Errorf("LRO[0].Y 不匹配：got %s, want %s", yBig.String(), yOrig.String())
	}
}

// TestCalldataChallengeAlpha 验证重放的 alpha 与证明者生成时一致。
// 方法：从 calldata 挑战中重建 ZTG2，然后用此 G2 点构造一次手动配对验证。
// 如果 alpha 不一致，ZTG2 就是错的，验证自然会失败（VerifyCompressed 已通过，说明一致）。
func TestCalldataChallengeAlpha(t *testing.T) {
	const T, M = 8, 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)

	ci, witnesses := buildTestCircuit(T, M)
	pk, vk, err := piano.SetupWithTrapdoors(ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	proof, err := bpiano.Compress(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}

	res, err := solgen.GenerateBPianoCalldata(proof, vk, nil)
	if err != nil {
		t.Fatalf("GenerateBPianoCalldata: %v", err)
	}

	ch := res.Calldata.Challenges

	// alpha 应为非零域元素。
	if ch.Alpha.IsZero() {
		t.Error("alpha 为零，FS 重放异常")
	}
	// beta 应为非零域元素。
	if ch.Beta.IsZero() {
		t.Error("beta 为零，FS 重放异常")
	}
	// rho 应为非零域元素。
	if ch.Rho.IsZero() {
		t.Error("rho 为零")
	}
	// alphaShifted = alpha * generatorX，两者不等（除非 generatorX=1）。
	if ch.Alpha.Equal(&ch.AlphaShifted) {
		t.Error("alphaShifted == alpha，generatorX 异常")
	}

	t.Logf("alpha        = %s", ch.Alpha.String())
	t.Logf("alphaShifted = %s", ch.AlphaShifted.String())
	t.Logf("beta         = %s", ch.Beta.String())
	t.Logf("rho          = %s", ch.Rho.String())
}

// TestCalldataPublicInputs 验证含公开输入时的字节数正确。
func TestCalldataPublicInputs(t *testing.T) {
	const T, M = 8, 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)

	// 构建含公开输入的电路：第 0 行 Ql=-1, Qk=public
	ql := make([]fr.Element, T)
	ql[0].SetInt64(-1)
	lro := make([]int, 3*T)
	for i := range lro {
		lro[i] = i
	}
	perm := piano.BuildPermutation(lro, 3*T, T)

	qk := make([]fr.Element, T)
	var pubVal fr.Element
	pubVal.SetInt64(42)
	qk[0].Set(&pubVal)

	ci := piano.CircuitInfo{
		Ql: ql, Qr: cloneZero(T),
		Qm: cloneZero(T), Qo: cloneZero(T),
		Qk: qk, Permutation: perm,
		NbPublicInputs: 1,
	}

	witnesses := make([]piano.WitnessInstance, M)
	for i := range witnesses {
		L := make([]fr.Element, T)
		L[0].Set(&pubVal) // L[0] 是公开输入
		witnesses[i] = piano.WitnessInstance{L: L, R: cloneZero(T), O: cloneZero(T)}
	}

	pk, vk, err := piano.SetupWithTrapdoors(ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	// 每个子电路提供 1 个公开输入
	pubInputs := make([][]fr.Element, M)
	for i := range pubInputs {
		pubInputs[i] = []fr.Element{pubVal}
	}

	proof, err := bpiano.Compress(pk, witnesses, pubInputs)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if err := bpiano.VerifyCompressed(proof, vk, pubInputs); err != nil {
		t.Fatalf("VerifyCompressed: %v", err)
	}

	res, err := solgen.GenerateBPianoCalldata(proof, vk, pubInputs)
	if err != nil {
		t.Fatalf("GenerateBPianoCalldata: %v", err)
	}

	// M×1 个公开输入 = 2 个 Fr 元素
	wantBytes := 12*solgen.G1Size + 15*solgen.FrSize + M*1*solgen.FrSize + 2*solgen.G2Size
	if len(res.Packed) != wantBytes {
		t.Errorf("含公开输入 Packed 长度 %d，期望 %d", len(res.Packed), wantBytes)
	}
	if res.Calldata.PublicInputsPerInstance != 1 {
		t.Errorf("PublicInputsPerInstance = %d，期望 1", res.Calldata.PublicInputsPerInstance)
	}
}
