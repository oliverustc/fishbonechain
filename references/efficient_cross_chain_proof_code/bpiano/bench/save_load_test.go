package bench_test

// save_load_test.go —— 证明持久化与基准测试
//
// TestGenerateAndSave（服务器执行一次）：
//   - 生成 K_MAX=100 个 Piano 证明
//   - 对 kValues 中每个 K 值独立运行 CoordinateChallenges(K) + AggregateProofs
//   - 将所有结果序列化写入 bench/testdata/
//
// TestBenchmarkFromFile（本地任意执行）：
//   - 读取 testdata/，对每个 K 值运行正确性验证 + 验证计时
//   - 无任何 Prove 调用

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	bpiano "github.com/oliverustc/bpiano/bpiano"
	"github.com/oliverustc/bpiano/dkzg"
	"github.com/oliverustc/bpiano/keccak"
	"github.com/oliverustc/bpiano/piano"

	bn254 "github.com/consensys/gnark-crypto/ecc/bn254"
	fr "github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

const testdataDir = "testdata"

// ── 元数据 ───────────────────────────────────────────────────────────────────

type ProofMeta struct {
	Circuit string `json:"circuit"`
	T       int    `json:"T"`
	M       int    `json:"M"`
	KMax    int    `json:"kMax"`
	KValues []int  `json:"kValues"`

	// Piano Proof 内切片长度（电路固定，反序列化时使用）
	NBatchedProofXClaims int `json:"nBatchedProofXClaims"`
	NBatchedProofYClaims int `json:"nBatchedProofYClaims"`

	PianoProvePerProofNs  int64  `json:"pianoProvePerProofNs"`
	BPianoProvePerProofNs int64  `json:"bpianoProvePerProofNs"` // K=KMax 时的均值
	GeneratedAt           string `json:"generatedAt"`
	Host                  string `json:"host"`
	GoVersion             string `json:"goVersion"`
	NumCPU                int    `json:"numCPU"`
}

// ── 底层序列化原语 ────────────────────────────────────────────────────────────

func writeG1(buf []byte, off int, p *bn254.G1Affine) int {
	b := p.Bytes()
	copy(buf[off:], b[:])
	return off + 32
}

func readG1(buf []byte, off int, p *bn254.G1Affine) (int, error) {
	if _, err := p.SetBytes(buf[off : off+32]); err != nil {
		return off, fmt.Errorf("readG1 at %d: %w", off, err)
	}
	return off + 32, nil
}

func writeG2(buf []byte, off int, p *bn254.G2Affine) int {
	b := p.Bytes()
	copy(buf[off:], b[:])
	return off + 64
}

func readG2(buf []byte, off int, p *bn254.G2Affine) (int, error) {
	if _, err := p.SetBytes(buf[off : off+64]); err != nil {
		return off, fmt.Errorf("readG2 at %d: %w", off, err)
	}
	return off + 64, nil
}

func writeFr(buf []byte, off int, e *fr.Element) int {
	b := e.Bytes()
	copy(buf[off:], b[:])
	return off + 32
}

func readFr(buf []byte, off int, e *fr.Element) int {
	e.SetBytes(buf[off : off+32])
	return off + 32
}

func writeU64(buf []byte, off int, v uint64) int {
	binary.BigEndian.PutUint64(buf[off:], v)
	return off + 8
}

func readU64(buf []byte, off int) (uint64, int) {
	return binary.BigEndian.Uint64(buf[off:]), off + 8
}

// ── Piano Proof 序列化 ────────────────────────────────────────────────────────

func pianoProofSize(meta *ProofMeta) int {
	// G1: LRO(3)+Z(1)+Hx(3)+Hy(3)+BatchedProofX.H(1)+ClaimedDigests(N)+ZShiftedProofX.H(1)+ComVF(1)+BatchedProofY.H(1)
	nG1 := 14 + meta.NBatchedProofXClaims
	// Fr: ClaimedValues(N) + 15 ClaimedXxx
	nFr := meta.NBatchedProofYClaims + 15
	return nG1*32 + nFr*32
}

func encodePianoProof(proof *piano.Proof, meta *ProofMeta) []byte {
	sz := pianoProofSize(meta)
	buf := make([]byte, sz)
	off := 0
	for i := range proof.LRO {
		off = writeG1(buf, off, &proof.LRO[i])
	}
	off = writeG1(buf, off, &proof.Z)
	for i := range proof.Hx {
		off = writeG1(buf, off, &proof.Hx[i])
	}
	for i := range proof.Hy {
		off = writeG1(buf, off, &proof.Hy[i])
	}
	off = writeG1(buf, off, &proof.BatchedProofX.H)
	for i := range proof.BatchedProofX.ClaimedDigests {
		off = writeG1(buf, off, &proof.BatchedProofX.ClaimedDigests[i])
	}
	off = writeG1(buf, off, &proof.ZShiftedProofX.H)
	off = writeG1(buf, off, &proof.ZShiftedProofX.ComVF)
	off = writeG1(buf, off, &proof.BatchedProofY.H)
	for i := range proof.BatchedProofY.ClaimedValues {
		off = writeFr(buf, off, &proof.BatchedProofY.ClaimedValues[i])
	}
	for _, e := range []*fr.Element{
		&proof.ClaimedA, &proof.ClaimedB, &proof.ClaimedO,
		&proof.ClaimedZ, &proof.ClaimedZS,
		&proof.ClaimedHx, &proof.ClaimedHy,
		&proof.ClaimedQl, &proof.ClaimedQr, &proof.ClaimedQm,
		&proof.ClaimedQo, &proof.ClaimedQk,
		&proof.ClaimedS1, &proof.ClaimedS2, &proof.ClaimedS3,
	} {
		off = writeFr(buf, off, e)
	}
	return buf
}

func decodePianoProof(buf []byte, meta *ProofMeta) (*piano.Proof, error) {
	proof := &piano.Proof{
		BatchedProofX: dkzg.BatchedProofX{
			ClaimedDigests: make([]bn254.G1Affine, meta.NBatchedProofXClaims),
		},
		BatchedProofY: dkzg.BatchedProofY{
			ClaimedValues: make([]fr.Element, meta.NBatchedProofYClaims),
		},
	}
	off := 0
	var err error
	for i := range proof.LRO {
		if off, err = readG1(buf, off, &proof.LRO[i]); err != nil {
			return nil, err
		}
	}
	if off, err = readG1(buf, off, &proof.Z); err != nil {
		return nil, err
	}
	for i := range proof.Hx {
		if off, err = readG1(buf, off, &proof.Hx[i]); err != nil {
			return nil, err
		}
	}
	for i := range proof.Hy {
		if off, err = readG1(buf, off, &proof.Hy[i]); err != nil {
			return nil, err
		}
	}
	if off, err = readG1(buf, off, &proof.BatchedProofX.H); err != nil {
		return nil, err
	}
	for i := range proof.BatchedProofX.ClaimedDigests {
		if off, err = readG1(buf, off, &proof.BatchedProofX.ClaimedDigests[i]); err != nil {
			return nil, err
		}
	}
	if off, err = readG1(buf, off, &proof.ZShiftedProofX.H); err != nil {
		return nil, err
	}
	if off, err = readG1(buf, off, &proof.ZShiftedProofX.ComVF); err != nil {
		return nil, err
	}
	if off, err = readG1(buf, off, &proof.BatchedProofY.H); err != nil {
		return nil, err
	}
	for i := range proof.BatchedProofY.ClaimedValues {
		off = readFr(buf, off, &proof.BatchedProofY.ClaimedValues[i])
	}
	for _, e := range []*fr.Element{
		&proof.ClaimedA, &proof.ClaimedB, &proof.ClaimedO,
		&proof.ClaimedZ, &proof.ClaimedZS,
		&proof.ClaimedHx, &proof.ClaimedHy,
		&proof.ClaimedQl, &proof.ClaimedQr, &proof.ClaimedQm,
		&proof.ClaimedQo, &proof.ClaimedQk,
		&proof.ClaimedS1, &proof.ClaimedS2, &proof.ClaimedS3,
	} {
		off = readFr(buf, off, e)
	}
	return proof, nil
}

// ── AggregatedProof 序列化 ────────────────────────────────────────────────────
//
// 格式：4字节 K（uint32 BE）+ K×864字节 CompressedProof + 32字节 ComQXTotal + 32字节 Pi1Total

const compressedProofSize = 12*32 + 15*32 // 864 bytes

func encodeAggregatedProof(agg *bpiano.AggregatedProof) []byte {
	K := len(agg.Proofs)
	buf := make([]byte, 4+K*compressedProofSize+32+32)
	binary.BigEndian.PutUint32(buf[0:], uint32(K))
	off := 4
	for _, p := range agg.Proofs {
		off = encodeCompressedProofInto(buf, off, p)
	}
	off = writeG1(buf, off, &agg.ComQXTotal)
	off = writeG1(buf, off, &agg.Pi1Total)
	_ = off
	return buf
}

func encodeCompressedProofInto(buf []byte, off int, proof *bpiano.CompressedProof) int {
	for i := range proof.LRO {
		off = writeG1(buf, off, &proof.LRO[i])
	}
	off = writeG1(buf, off, &proof.Z)
	for i := range proof.Hx {
		off = writeG1(buf, off, &proof.Hx[i])
	}
	off = writeG1(buf, off, &proof.ComQX)
	off = writeG1(buf, off, &proof.ComVFAlpha)
	off = writeG1(buf, off, &proof.ComVFZS)
	off = writeG1(buf, off, &proof.ComGY)
	off = writeG1(buf, off, &proof.Pi1AggH)
	for _, e := range []*fr.Element{
		&proof.EvalA, &proof.EvalB, &proof.EvalO,
		&proof.EvalZ, &proof.EvalZS,
		&proof.EvalHx, &proof.EvalHy,
		&proof.EvalQl, &proof.EvalQr, &proof.EvalQm,
		&proof.EvalQo, &proof.EvalQk,
		&proof.EvalS1, &proof.EvalS2, &proof.EvalS3,
	} {
		off = writeFr(buf, off, e)
	}
	return off
}

func decodeAggregatedProof(buf []byte) (*bpiano.AggregatedProof, error) {
	if len(buf) < 4 {
		return nil, fmt.Errorf("decodeAggregatedProof: buffer too short")
	}
	K := int(binary.BigEndian.Uint32(buf[0:4]))
	expected := 4 + K*compressedProofSize + 64
	if len(buf) != expected {
		return nil, fmt.Errorf("decodeAggregatedProof: size mismatch: got %d want %d", len(buf), expected)
	}
	agg := &bpiano.AggregatedProof{
		K:      K,
		Proofs: make([]*bpiano.CompressedProof, K),
	}
	off := 4
	var err error
	for k := 0; k < K; k++ {
		agg.Proofs[k], off, err = decodeCompressedProofFrom(buf, off)
		if err != nil {
			return nil, fmt.Errorf("proof[%d]: %w", k, err)
		}
	}
	if off, err = readG1(buf, off, &agg.ComQXTotal); err != nil {
		return nil, fmt.Errorf("ComQXTotal: %w", err)
	}
	if _, err = readG1(buf, off, &agg.Pi1Total); err != nil {
		return nil, fmt.Errorf("Pi1Total: %w", err)
	}
	return agg, nil
}

func decodeCompressedProofFrom(buf []byte, off int) (*bpiano.CompressedProof, int, error) {
	proof := &bpiano.CompressedProof{}
	var err error
	for i := range proof.LRO {
		if off, err = readG1(buf, off, &proof.LRO[i]); err != nil {
			return nil, off, err
		}
	}
	if off, err = readG1(buf, off, &proof.Z); err != nil {
		return nil, off, err
	}
	for i := range proof.Hx {
		if off, err = readG1(buf, off, &proof.Hx[i]); err != nil {
			return nil, off, err
		}
	}
	if off, err = readG1(buf, off, &proof.ComQX); err != nil {
		return nil, off, err
	}
	if off, err = readG1(buf, off, &proof.ComVFAlpha); err != nil {
		return nil, off, err
	}
	if off, err = readG1(buf, off, &proof.ComVFZS); err != nil {
		return nil, off, err
	}
	if off, err = readG1(buf, off, &proof.ComGY); err != nil {
		return nil, off, err
	}
	if off, err = readG1(buf, off, &proof.Pi1AggH); err != nil {
		return nil, off, err
	}
	for _, e := range []*fr.Element{
		&proof.EvalA, &proof.EvalB, &proof.EvalO,
		&proof.EvalZ, &proof.EvalZS,
		&proof.EvalHx, &proof.EvalHy,
		&proof.EvalQl, &proof.EvalQr, &proof.EvalQm,
		&proof.EvalQo, &proof.EvalQk,
		&proof.EvalS1, &proof.EvalS2, &proof.EvalS3,
	} {
		off = readFr(buf, off, e)
	}
	return proof, off, nil
}

// ── VerifyingKey 序列化（仅验证所需字段）────────────────────────────────────

// vkSize = 8×3（整数）+ 32×3（Fr）+ 64×5（G2）+ 32×8（G1 selector）= 696 bytes
const vkSize = 8*3 + 32*3 + 64*5 + 32*8

func encodeVK(vk *piano.VerifyingKey) []byte {
	buf := make([]byte, vkSize)
	off := 0
	off = writeU64(buf, off, vk.SizeX)
	off = writeU64(buf, off, vk.SizeY)
	off = writeU64(buf, off, uint64(vk.NbPublicInputs))
	off = writeFr(buf, off, &vk.GeneratorX)
	off = writeFr(buf, off, &vk.GeneratorY)
	off = writeFr(buf, off, &vk.CosetShift)
	srs := vk.DKZGSRS
	for i := range srs.G2 {
		off = writeG2(buf, off, &srs.G2[i])
	}
	for i := range srs.G2Y {
		off = writeG2(buf, off, &srs.G2Y[i])
	}
	for _, d := range []dkzg.Digest{vk.Ql, vk.Qr, vk.Qm, vk.Qo, vk.Qk, vk.S1, vk.S2, vk.S3} {
		p := d
		off = writeG1(buf, off, &p)
	}
	return buf
}

func decodeVK(buf []byte) (*piano.VerifyingKey, error) {
	vk := &piano.VerifyingKey{}
	slimSRS := &dkzg.SRS{}
	off := 0
	var err error

	var sizeX, sizeY, nbPI uint64
	sizeX, off = readU64(buf, off)
	sizeY, off = readU64(buf, off)
	nbPI, off = readU64(buf, off)
	vk.SizeX = sizeX
	vk.SizeY = sizeY
	vk.NbPublicInputs = int(nbPI)

	off = readFr(buf, off, &vk.GeneratorX)
	off = readFr(buf, off, &vk.GeneratorY)
	off = readFr(buf, off, &vk.CosetShift)

	for i := range slimSRS.G2 {
		if off, err = readG2(buf, off, &slimSRS.G2[i]); err != nil {
			return nil, fmt.Errorf("G2[%d]: %w", i, err)
		}
	}
	for i := range slimSRS.G2Y {
		if off, err = readG2(buf, off, &slimSRS.G2Y[i]); err != nil {
			return nil, fmt.Errorf("G2Y[%d]: %w", i, err)
		}
	}

	// 从 SizeX 重建 DomainX.CardinalityInv（= 1/T mod Fr），
	// bpiano.VerifyBatch 的代数约束检查需要此字段。
	var cardInvX fr.Element
	cardInvX.SetUint64(vk.SizeX).Inverse(&cardInvX)
	slimSRS.DomainX.CardinalityInv = cardInvX

	vk.DKZGSRS = slimSRS

	selectors := []*dkzg.Digest{&vk.Ql, &vk.Qr, &vk.Qm, &vk.Qo, &vk.Qk, &vk.S1, &vk.S2, &vk.S3}
	for i, d := range selectors {
		if off, err = readG1(buf, off, d); err != nil {
			return nil, fmt.Errorf("selector[%d]: %w", i, err)
		}
	}
	return vk, nil
}

// ── aggFileName ───────────────────────────────────────────────────────────────

func aggFileName(K int) string {
	return fmt.Sprintf("agg_k%03d.bin", K)
}

// testdataComplete 检查 testdata/ 目录中是否存在所有必需文件。
// 用于 TestBench 自动判断是否需要先执行生成步骤。
func testdataComplete(kVals []int) bool {
	for _, name := range []string{"piano_proofs.bin", "vk.bin", "meta.json"} {
		if _, err := os.Stat(filepath.Join(testdataDir, name)); err != nil {
			return false
		}
	}
	for _, K := range kVals {
		if _, err := os.Stat(filepath.Join(testdataDir, aggFileName(K))); err != nil {
			return false
		}
	}
	return true
}

// ── TestGenerateAndSave（在服务器上执行一次）────────────────────────────────

// TestGenerateAndSave 生成 K_MAX=100 个 Piano 证明，并对 kValues 中每个 K 值
// 独立运行 CoordinateChallenges(K) + AggregateProofs，序列化写入 bench/testdata/。
//
// 总 prove 调用：100（Piano）+ Σ_K kValues（BPiano）= 650 次
// 预计服务器（112 核）耗时约 70 分钟。
//
// 运行方式（服务器）：
//
//	go test ./bench/ -v -run TestGenerateAndSave -timeout 300m
func TestGenerateAndSave(t *testing.T) {
	// ── 电路与 Setup ──────────────────────────────────────────────────────
	t.Log("构建 Keccak-256 电路（T≈2^18）…")
	t0 := time.Now()
	kc := keccak.Build()
	ci := kc.CircuitInfo()
	wh := kc.WitnessHelper()
	T := wh.Size()
	t.Logf("电路构建完成，用时 %s  T=%d", time.Since(t0).Round(time.Millisecond), T)

	const M = 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)

	t.Log("执行 Setup …")
	t0 = time.Now()
	pk, vk, err := piano.SetupWithTrapdoors(*ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup 失败：%v", err)
	}
	t.Logf("Setup 完成，用时 %s", time.Since(t0).Round(time.Millisecond))

	// ── 生成 K_MAX 组 Witness（所有 K 值共用）────────────────────────────
	t.Logf("生成 %d 组 Witness …", kMax)
	allPKs := make([]*piano.ProvingKey, kMax)
	allWitnesses := make([][]piano.WitnessInstance, kMax)
	allPubInputs := make([][][]bpiano.Fr, kMax)
	for k := 0; k < kMax; k++ {
		allPKs[k] = pk
		ws := make([]piano.WitnessInstance, M)
		for m := 0; m < M; m++ {
			var msg [64]byte
			msg[0] = byte(k*M + m + 1)
			varVals := kc.WitnessFor(msg)
			ws[m] = wh.Make(varVals)
		}
		allWitnesses[k] = ws
		allPubInputs[k] = nil
	}

	// ── 生成 K_MAX 个 Piano 证明 ─────────────────────────────────────────
	t.Logf("生成 %d 个 Piano 证明 …", kMax)
	t0 = time.Now()
	allPianoProofs := make([]*piano.Proof, kMax)
	for k := 0; k < kMax; k++ {
		allPianoProofs[k], err = piano.Prove(pk, allWitnesses[k], nil)
		if err != nil {
			t.Fatalf("Piano.Prove[%d] 失败：%v", k, err)
		}
		if (k+1)%10 == 0 {
			t.Logf("  Piano.Prove 进度：%d/%d  已用 %s", k+1, kMax, time.Since(t0).Round(time.Second))
		}
	}
	pianoProveTotal := time.Since(t0)
	pianoProvePerProof := pianoProveTotal / time.Duration(kMax)
	t.Logf("Piano 证明完成：总 %s，均 %s/个", pianoProveTotal.Round(time.Second), pianoProvePerProof.Round(time.Millisecond))

	if err := piano.Verify(vk, allPianoProofs[0], nil); err != nil {
		t.Fatalf("Piano.Verify[0] 正确性失败：%v", err)
	}

	// ── 确定切片长度 ──────────────────────────────────────────────────────
	nBatchedX := len(allPianoProofs[0].BatchedProofX.ClaimedDigests)
	nBatchedY := len(allPianoProofs[0].BatchedProofY.ClaimedValues)

	// ── 对每个 K 值独立生成 BPiano 协调证明 ──────────────────────────────
	var bpianoProveTotalAll time.Duration
	var bpianoProveCalls int

	if err := os.MkdirAll(testdataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll 失败：%v", err)
	}

	for _, K := range kValues {
		t.Logf("CoordinateChallenges(K=%d) …", K)
		t0 := time.Now()
		coordProofs, err := bpiano.CoordinateChallenges(
			allPKs[0:K], allWitnesses[0:K], allPubInputs[0:K],
		)
		if err != nil {
			t.Fatalf("CoordinateChallenges(K=%d) 失败：%v", K, err)
		}
		agg, err := bpiano.AggregateProofs(coordProofs)
		if err != nil {
			t.Fatalf("AggregateProofs(K=%d) 失败：%v", K, err)
		}
		elapsed := time.Since(t0)
		bpianoProveTotalAll += elapsed
		bpianoProveCalls += K

		// 正确性验证
		if err := bpiano.VerifyBatch(agg, vk, allPubInputs[0:K]); err != nil {
			t.Fatalf("VerifyBatch(K=%d) 正确性失败：%v", K, err)
		}
		t.Logf("  K=%3d 正确性验证通过 ✓  用时 %s", K, elapsed.Round(time.Millisecond))

		// 写入 agg_k{K}.bin
		data := encodeAggregatedProof(agg)
		if err := os.WriteFile(filepath.Join(testdataDir, aggFileName(K)), data, 0o644); err != nil {
			t.Fatalf("写入 %s 失败：%v", aggFileName(K), err)
		}
	}

	bpianoProvePerProof := bpianoProveTotalAll / time.Duration(bpianoProveCalls)

	// ── 保存 Piano 证明 ───────────────────────────────────────────────────
	meta := &ProofMeta{
		Circuit:               "Keccak-256",
		T:                     T,
		M:                     M,
		KMax:                  kMax,
		KValues:               kValues,
		NBatchedProofXClaims:  nBatchedX,
		NBatchedProofYClaims:  nBatchedY,
		PianoProvePerProofNs:  pianoProvePerProof.Nanoseconds(),
		BPianoProvePerProofNs: bpianoProvePerProof.Nanoseconds(),
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
		GoVersion:             runtime.Version(),
		NumCPU:                runtime.NumCPU(),
	}
	if h, err2 := os.Hostname(); err2 == nil {
		meta.Host = h
	}

	sz := pianoProofSize(meta)
	pBuf := make([]byte, sz*kMax)
	for i, p := range allPianoProofs {
		copy(pBuf[i*sz:], encodePianoProof(p, meta))
	}
	if err := os.WriteFile(filepath.Join(testdataDir, "piano_proofs.bin"), pBuf, 0o644); err != nil {
		t.Fatalf("写入 piano_proofs.bin 失败：%v", err)
	}

	if err := os.WriteFile(filepath.Join(testdataDir, "vk.bin"), encodeVK(vk), 0o644); err != nil {
		t.Fatalf("写入 vk.bin 失败：%v", err)
	}

	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(testdataDir, "meta.json"), metaBytes, 0o644); err != nil {
		t.Fatalf("写入 meta.json 失败：%v", err)
	}

	t.Log("✓ 所有文件已写入 testdata/")
	for _, name := range append([]string{"piano_proofs.bin", "vk.bin", "meta.json"},
		func() []string {
			var names []string
			for _, K := range kValues {
				names = append(names, aggFileName(K))
			}
			return names
		}()...) {
		info, _ := os.Stat(filepath.Join(testdataDir, name))
		if info != nil {
			t.Logf("  %-22s  %.1f KB", name, float64(info.Size())/1024)
		}
	}
}

// ── TestBenchmarkFromFile（本地执行，无需 Prove）──────────────────────────

// TestBenchmarkFromFile 从 bench/testdata/ 读取证明，
// 对每个 K 值运行正确性验证 + Piano × K / BPiano VerifyBatch 计时。
//
// 运行方式（本地）：
//
//	go test ./bench/ -v -run TestBenchmarkFromFile -timeout 30m
func TestBenchmarkFromFile(t *testing.T) {
	// ── 加载 meta 和 vk ───────────────────────────────────────────────────
	metaBytes, err := os.ReadFile(filepath.Join(testdataDir, "meta.json"))
	if err != nil {
		t.Fatalf("读取 meta.json 失败（请先在服务器上运行 TestGenerateAndSave）：%v", err)
	}
	var meta ProofMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("解析 meta.json 失败：%v", err)
	}

	vkBytes, err := os.ReadFile(filepath.Join(testdataDir, "vk.bin"))
	if err != nil {
		t.Fatalf("读取 vk.bin 失败：%v", err)
	}
	vk, err := decodeVK(vkBytes)
	if err != nil {
		t.Fatalf("解码 vk.bin 失败：%v", err)
	}

	// ── 加载 Piano 证明 ───────────────────────────────────────────────────
	pData, err := os.ReadFile(filepath.Join(testdataDir, "piano_proofs.bin"))
	if err != nil {
		t.Fatalf("读取 piano_proofs.bin 失败：%v", err)
	}
	sz := pianoProofSize(&meta)
	allPianoProofs := make([]*piano.Proof, meta.KMax)
	for i := 0; i < meta.KMax; i++ {
		allPianoProofs[i], err = decodePianoProof(pData[i*sz:(i+1)*sz], &meta)
		if err != nil {
			t.Fatalf("解码 piano_proof[%d] 失败：%v", i, err)
		}
	}
	t.Logf("Piano 证明加载完成（%d 个）", meta.KMax)

	allPubInputs := make([][][]bpiano.Fr, meta.KMax)

	// ── 打印表头 ──────────────────────────────────────────────────────────
	pianoProvePerProof := time.Duration(meta.PianoProvePerProofNs)
	bpianoProvePerProof := time.Duration(meta.BPianoProvePerProofNs)

	fmt.Println()
	fmt.Printf("电路：%s  T=%d  M=%d\n", meta.Circuit, meta.T, meta.M)
	fmt.Printf("生成环境：%s  CPU×%d  %s  %s\n", meta.Host, meta.NumCPU, meta.GoVersion, meta.GeneratedAt)
	fmt.Printf("Prove 均值：Piano %.2fs/个，BPiano %.2fs/个（† = K × 均值线性外推）\n",
		pianoProvePerProof.Seconds(), bpianoProvePerProof.Seconds())
	hdr := "──────────────────────────────────────────────────────────────────────────────────────────────────"
	fmt.Println(hdr)
	fmt.Printf("%-4s  %-10s  %-10s  %-12s  %-13s  %-12s  %-13s  %s\n",
		"K", "PianoSize", "BPianoSize", "PianoProve†", "BPianoProve†", "PianoVerify", "BPianoVerify", "加速比")
	fmt.Println(hdr)

	singlePianoSize := pianoProofSize(&meta) // 等同于 len(marshalPianoProof(proof))

	for _, K := range kValues {
		t.Run(fmt.Sprintf("K=%d", K), func(t *testing.T) {
			// ── 加载该 K 值的 AggregatedProof ────────────────────────
			aggData, err := os.ReadFile(filepath.Join(testdataDir, aggFileName(K)))
			if err != nil {
				t.Fatalf("读取 %s 失败：%v", aggFileName(K), err)
			}
			agg, err := decodeAggregatedProof(aggData)
			if err != nil {
				t.Fatalf("解码 %s 失败：%v", aggFileName(K), err)
			}

			// ── 正确性验证 ────────────────────────────────────────────
			if err := bpiano.VerifyBatch(agg, vk, allPubInputs[0:K]); err != nil {
				t.Fatalf("VerifyBatch(K=%d) 正确性失败：%v", K, err)
			}

			// ── 证明大小 ─────────────────────────────────────────────
			pianoSize := K * singlePianoSize
			bpianoSize := 4 + K*compressedProofSize + 64 // 等同于 len(encodeAggregatedProof(agg))

			// ── 估算证明时间 ──────────────────────────────────────────
			pianoProveTime := time.Duration(int64(pianoProvePerProof) * int64(K))
			bpianoProveTime := time.Duration(int64(bpianoProvePerProof) * int64(K))

			// 交替计时（ABAB 设计）：每轮先 Piano 后 BPiano，消除 cache 系统性偏差
			var pianoTotal, bpianoTotal time.Duration
			for rep := 0; rep < verifyRepsM; rep++ {
				t0 := time.Now()
				for k := 0; k < K; k++ {
					_ = piano.Verify(vk, allPianoProofs[k], nil)
				}
				pianoTotal += time.Since(t0)

				t0 = time.Now()
				if err := bpiano.VerifyBatch(agg, vk, allPubInputs[0:K]); err != nil {
					t.Fatalf("VerifyBatch(K=%d) rep=%d 失败：%v", K, rep, err)
				}
				bpianoTotal += time.Since(t0)
			}
			pianoVerifyTime := pianoTotal / time.Duration(verifyRepsM)
			bpianoVerifyTime := bpianoTotal / time.Duration(verifyRepsM)

			// ── 输出 ─────────────────────────────────────────────────
			speedup := float64(pianoVerifyTime) / float64(bpianoVerifyTime)
			fmt.Printf("K=%-3d  %8d B  %8d B  %12s  %13s  %12s  %13s  %.2fx\n",
				K,
				pianoSize, bpianoSize,
				pianoProveTime.Round(time.Millisecond),
				bpianoProveTime.Round(time.Millisecond),
				pianoVerifyTime.Round(time.Microsecond),
				bpianoVerifyTime.Round(time.Microsecond),
				speedup)
		})
	}
	fmt.Println(hdr)
}
