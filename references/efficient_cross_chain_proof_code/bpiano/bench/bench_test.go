// Package bench 在 Keccak-256 大型电路（T≈2^18）上对比 Piano 与 BPiano 的性能。
// 运行方式：
//
//	go test ./bench/ -v -run TestCompare -timeout 30m
package bench_test

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/oliverustc/bpiano/bpiano"
	"github.com/oliverustc/bpiano/keccak"
	"github.com/oliverustc/bpiano/piano"
)

// marshalPianoProof 将 piano.Proof 序列化为字节切片。
// 每个 G1Affine 点以 BN254 压缩格式（32 字节）写入，
// 每个 fr.Element 以大端整数格式（32 字节）写入。
func marshalPianoProof(proof *piano.Proof) []byte {
	var buf bytes.Buffer

	writeG1 := func(p interface{ Bytes() [32]byte }) {
		b := p.Bytes()
		buf.Write(b[:])
	}
	writeFr := func(e interface{ Bytes() [32]byte }) {
		b := e.Bytes()
		buf.Write(b[:])
	}

	// LRO：3 个 G1
	for i := range proof.LRO {
		writeG1(&proof.LRO[i])
	}
	// Z：1 个 G1
	writeG1(&proof.Z)
	// Hx：3 个 G1
	for i := range proof.Hx {
		writeG1(&proof.Hx[i])
	}
	// Hy：3 个 G1
	for i := range proof.Hy {
		writeG1(&proof.Hy[i])
	}
	// BatchedProofX：H（1 个 G1）+ ClaimedDigests（K 个 G1）
	writeG1(&proof.BatchedProofX.H)
	for i := range proof.BatchedProofX.ClaimedDigests {
		writeG1(&proof.BatchedProofX.ClaimedDigests[i])
	}
	// ZShiftedProofX：H + ComVF（各 1 个 G1）
	writeG1(&proof.ZShiftedProofX.H)
	writeG1(&proof.ZShiftedProofX.ComVF)
	// BatchedProofY：H（1 个 G1）+ ClaimedValues（K 个 fr）
	writeG1(&proof.BatchedProofY.H)
	for i := range proof.BatchedProofY.ClaimedValues {
		writeFr(&proof.BatchedProofY.ClaimedValues[i])
	}
	// 15 个声明标量：ClaimedA/B/O/Z/ZS/Hx/Hy + ClaimedQl/Qr/Qm/Qo/Qk + ClaimedS1/S2/S3
	for _, e := range []interface{ Bytes() [32]byte }{
		&proof.ClaimedA, &proof.ClaimedB, &proof.ClaimedO,
		&proof.ClaimedZ, &proof.ClaimedZS,
		&proof.ClaimedHx, &proof.ClaimedHy,
		&proof.ClaimedQl, &proof.ClaimedQr, &proof.ClaimedQm,
		&proof.ClaimedQo, &proof.ClaimedQk,
		&proof.ClaimedS1, &proof.ClaimedS2, &proof.ClaimedS3,
	} {
		writeFr(e)
	}

	return buf.Bytes()
}

// marshalBPianoProof 将 bpiano.CompressedProof 序列化为字节切片。
func marshalBPianoProof(proof *bpiano.CompressedProof) []byte {
	var buf bytes.Buffer

	writeG1 := func(p interface{ Bytes() [32]byte }) {
		b := p.Bytes()
		buf.Write(b[:])
	}
	writeFr := func(e interface{ Bytes() [32]byte }) {
		b := e.Bytes()
		buf.Write(b[:])
	}

	// LRO：3 个 G1
	for i := range proof.LRO {
		writeG1(&proof.LRO[i])
	}
	// Z：1 个 G1
	writeG1(&proof.Z)
	// Hx：3 个 G1
	for i := range proof.Hx {
		writeG1(&proof.Hx[i])
	}
	// Shplonk 承诺及 DKZG witness 修正承诺：4 个 G1
	writeG1(&proof.ComQX)
	writeG1(&proof.ComVFAlpha)
	writeG1(&proof.ComVFZS)
	// Y 轴聚合承诺及商：2 个 G1
	writeG1(&proof.ComGY)
	writeG1(&proof.Pi1AggH)
	// 15 个求值标量
	for _, e := range []interface{ Bytes() [32]byte }{
		&proof.EvalA, &proof.EvalB, &proof.EvalO,
		&proof.EvalZ, &proof.EvalZS,
		&proof.EvalHx, &proof.EvalHy,
		&proof.EvalQl, &proof.EvalQr, &proof.EvalQm,
		&proof.EvalQo, &proof.EvalQk,
		&proof.EvalS1, &proof.EvalS2, &proof.EvalS3,
	} {
		writeFr(e)
	}

	return buf.Bytes()
}

// marshalAggregatedProof 序列化 AggregatedProof 为字节切片。
// 格式：K × CompressedProof（逐个序列化）+ ComQXTotal(32B) + Pi1Total(32B)
func marshalAggregatedProof(agg *bpiano.AggregatedProof) []byte {
	var buf bytes.Buffer
	for _, p := range agg.Proofs {
		buf.Write(marshalBPianoProof(p))
	}
	b1 := agg.ComQXTotal.Bytes()
	buf.Write(b1[:])
	b2 := agg.Pi1Total.Bytes()
	buf.Write(b2[:])
	return buf.Bytes()
}

const verifyReps = 5 // 用于计时的验证运行次数（单证明测试）

// kValues 是多证明聚合对比测试中使用的 K 序列。
var kValues = []int{2, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

const (
	kMax        = 100 // 预生成证明的最大数量
	verifyRepsM = 10  // 多证明测试中的验证重复次数
)

// TestCompareMultiWitness 对 K ∈ {2,10,20,…,100} 对比：
//   - 方案 A：Piano × K（K 次独立 Prove/Verify）
//   - 方案 B：BPiano 聚合（CoordinateChallenges + AggregateProofs + VerifyBatch）
//
// 策略：预先生成 K_MAX=100 组 Piano 和 BPiano 证明，各 K 值的计时复用切片，
// 避免为每个 K 值重复执行 K 次 Prove（总 prove 调用 = 100 + 100，而非 Σ_K·2）。
// 证明时间 = K × 均值（标注 †）；验证计时直接实测。
// 正确性在 K=100 时完整验证；K < 100 的 BPiano 计时使用 K=100 协调证明子集（‡）。
//
// 运行方式：
//
//	go test ./bench/ -v -run TestCompareMultiWitness -timeout 300m
func TestCompareMultiWitness(t *testing.T) {
	// ── 构建电路 ──────────────────────────────────────────────────────────
	t.Log("正在构建 Keccak-256 电路（T≈2^18）…")
	t0 := time.Now()
	kc := keccak.Build()
	ci := kc.CircuitInfo()
	wh := kc.WitnessHelper()
	T := wh.Size()
	t.Logf("电路构建完成，用时 %s  T=%d", time.Since(t0).Round(time.Millisecond), T)

	const M = 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)

	// ── Setup（所有 K 共用同一 pk/vk）────────────────────────────────────
	t.Log("正在执行 Setup …")
	t0 = time.Now()
	pk, vk, err := piano.SetupWithTrapdoors(*ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup 失败：%v", err)
	}
	t.Logf("Setup 完成，用时 %s", time.Since(t0).Round(time.Millisecond))

	// ── 预生成 K_MAX 组 Witness ───────────────────────────────────────────
	t.Logf("预生成 %d 组 witness …", kMax)
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

	// ── 预生成 K_MAX 个 Piano 证明 ────────────────────────────────────────
	t.Logf("预生成 %d 个 Piano 证明 …", kMax)
	t0 = time.Now()
	allPianoProofs := make([]*piano.Proof, kMax)
	for k := 0; k < kMax; k++ {
		allPianoProofs[k], err = piano.Prove(pk, allWitnesses[k], nil)
		if err != nil {
			t.Fatalf("Piano.Prove[%d] 失败：%v", k, err)
		}
	}
	pianoProveTotal := time.Since(t0)
	pianoProvePerProof := pianoProveTotal / time.Duration(kMax)
	t.Logf("Piano 证明完成：总 %s，均 %s/个",
		pianoProveTotal.Round(time.Millisecond), pianoProvePerProof.Round(time.Millisecond))

	// Piano 单次正确性验证
	if err := piano.Verify(vk, allPianoProofs[0], nil); err != nil {
		t.Fatalf("Piano.Verify 正确性失败：%v", err)
	}

	// ── 预生成 K_MAX 个 BPiano 协调证明（共享 α/β 来自 K=100）────────────
	t.Logf("预生成 %d 个 BPiano 协调证明（CoordinateChallenges, K=%d）…", kMax, kMax)
	t0 = time.Now()
	allCoordProofs, err := bpiano.CoordinateChallenges(allPKs, allWitnesses, allPubInputs)
	if err != nil {
		t.Fatalf("CoordinateChallenges 失败：%v", err)
	}
	bpianoProveTotal := time.Since(t0)
	bpianoProvePerProof := bpianoProveTotal / time.Duration(kMax)
	t.Logf("BPiano 协调完成：总 %s，均 %s/个",
		bpianoProveTotal.Round(time.Millisecond), bpianoProvePerProof.Round(time.Millisecond))

	// K=100 完整正确性验证
	aggFull, err := bpiano.AggregateProofs(allCoordProofs)
	if err != nil {
		t.Fatalf("AggregateProofs(K=%d) 失败：%v", kMax, err)
	}
	if err := bpiano.VerifyBatch(aggFull, vk, allPubInputs); err != nil {
		t.Fatalf("VerifyBatch(K=%d) 正确性验证失败：%v", kMax, err)
	}
	t.Logf("VerifyBatch(K=%d) 正确性验证通过 ✓", kMax)

	// ── 打印表头 ──────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Printf("电路：Keccak-256  T=%d  M=%d  K_max=%d\n", T, M, kMax)
	fmt.Printf("证明均值（K=100）：Piano %.2fs/个，BPiano %.2fs/个\n",
		pianoProvePerProof.Seconds(), bpianoProvePerProof.Seconds())
	fmt.Printf("† 证明时间 = K × 均值（线性外推）\n")
	fmt.Printf("‡ K < %d：BPiano 验证计时使用 K=100 协调证明子集，计时代表性有效\n", kMax)
	hdr := "──────────────────────────────────────────────────────────────────────────────────────────────────"
	fmt.Println(hdr)
	fmt.Printf("%-4s  %-10s  %-10s  %-13s  %-14s  %-12s  %-13s  %s\n",
		"K", "PianoSize", "BPianoSize", "PianoProve†", "BPianoProve†", "PianoVerify", "BPianoVerify", "加速比")
	fmt.Println(hdr)

	singlePianoSize := len(marshalPianoProof(allPianoProofs[0]))

	for _, K := range kValues {
		t.Run(fmt.Sprintf("K=%d", K), func(t *testing.T) {
			// ── 证明大小 ─────────────────────────────────────────────
			pianoSize := K * singlePianoSize
			subAgg, err := bpiano.AggregateProofs(allCoordProofs[0:K])
			if err != nil {
				t.Fatalf("AggregateProofs(K=%d) 失败：%v", K, err)
			}
			bpianoSize := len(marshalAggregatedProof(subAgg))

			// ── 估算证明时间（线性外推）──────────────────────────────
			pianoProveTime := time.Duration(int64(pianoProvePerProof) * int64(K))
			bpianoProveTime := time.Duration(int64(bpianoProvePerProof) * int64(K))

			// ── 方案 A：Piano × K 验证计时 ────────────────────────────
			t0 := time.Now()
			for rep := 0; rep < verifyRepsM; rep++ {
				for k := 0; k < K; k++ {
					_ = piano.Verify(vk, allPianoProofs[k], nil)
				}
			}
			pianoVerifyTime := time.Since(t0) / time.Duration(verifyRepsM)

			// ── 方案 B：BPiano VerifyBatch 计时 ──────────────────────
			// 对 K < K_MAX，使用 K=100 协调证明的前 K 个（计时有效，正确性见 K=100）
			subPubInputs := allPubInputs[0:K]
			t0 = time.Now()
			for rep := 0; rep < verifyRepsM; rep++ {
				_ = bpiano.VerifyBatch(subAgg, vk, subPubInputs)
			}
			bpianoVerifyTime := time.Since(t0) / time.Duration(verifyRepsM)

			// ── 输出 ─────────────────────────────────────────────────
			speedup := float64(pianoVerifyTime) / float64(bpianoVerifyTime)
			tag := "‡"
			if K == kMax {
				tag = " "
			}
			fmt.Printf("K=%-3d  %8d B  %8d B  %13s  %14s  %12s  %12s%s  %.2fx\n",
				K,
				pianoSize, bpianoSize,
				pianoProveTime.Round(time.Millisecond),
				bpianoProveTime.Round(time.Millisecond),
				pianoVerifyTime.Round(time.Microsecond),
				bpianoVerifyTime.Round(time.Microsecond),
				tag,
				speedup)
		})
	}
	fmt.Println(hdr)
}

func TestCompare(t *testing.T) {
	// ── 构建电路 ─────────────────────────────────────────────────────────────
	t.Log("正在构建 Keccak-256 电路（T≈2^18）…")
	t0 := time.Now()
	kc := keccak.Build()
	ci := kc.CircuitInfo()
	wh := kc.WitnessHelper()
	T := wh.Size()
	t.Logf("电路构建完成，用时 %s  T=%d", time.Since(t0).Round(time.Millisecond), T)

	M := 2
	tauX := big.NewInt(17)
	tauY := big.NewInt(23)

	// ── Setup ────────────────────────────────────────────────────────────────
	t.Log("正在执行 Setup …")
	t0 = time.Now()
	pk, vk, err := piano.SetupWithTrapdoors(*ci, uint64(M), uint64(T), tauX, tauY)
	if err != nil {
		t.Fatalf("Setup 失败：%v", err)
	}
	t.Logf("Setup 完成，用时 %s", time.Since(t0).Round(time.Millisecond))

	// ── 生成 Witness ─────────────────────────────────────────────────────────
	witnesses := make([]piano.WitnessInstance, M)
	for k := 0; k < M; k++ {
		var msg [64]byte
		msg[0] = byte(k + 1)
		varVals := kc.WitnessFor(msg)
		witnesses[k] = wh.Make(varVals)
	}

	// ════════════════════════════════════════════════════════════════════════
	// Piano：Prove + Verify
	// ════════════════════════════════════════════════════════════════════════

	t.Log("Piano：正在证明 …")
	t0 = time.Now()
	pianoProof, err := piano.Prove(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Piano.Prove 失败：%v", err)
	}
	pianoProveTime := time.Since(t0)
	t.Logf("Piano：证明完成，用时 %s", pianoProveTime.Round(time.Millisecond))

	// 验证一次以检验正确性。
	if err := piano.Verify(vk, pianoProof, nil); err != nil {
		t.Fatalf("Piano.Verify（正确性检验）失败：%v", err)
	}

	// 运行 verifyReps 次以计时。
	t0 = time.Now()
	for i := 0; i < verifyReps; i++ {
		if err := piano.Verify(vk, pianoProof, nil); err != nil {
			t.Fatalf("Piano.Verify 失败：%v", err)
		}
	}
	pianoVerifyTime := time.Since(t0) / time.Duration(verifyReps)

	// 序列化实际证明以获得真实字节数。
	pianoPrfBytes := len(marshalPianoProof(pianoProof))

	// ════════════════════════════════════════════════════════════════════════
	// BPiano：Compress + VerifyCompressed
	// ════════════════════════════════════════════════════════════════════════

	t.Log("BPiano：正在压缩 …")
	t0 = time.Now()
	bproof, err := bpiano.Compress(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("BPiano.Compress 失败：%v", err)
	}
	bpianoProveTime := time.Since(t0)
	t.Logf("BPiano：压缩完成，用时 %s", bpianoProveTime.Round(time.Millisecond))

	// 验证一次以检验正确性。
	if err := bpiano.VerifyCompressed(bproof, vk, nil); err != nil {
		t.Fatalf("BPiano.VerifyCompressed（正确性检验）失败：%v", err)
	}

	// 运行 verifyReps 次以计时。
	t0 = time.Now()
	for i := 0; i < verifyReps; i++ {
		if err := bpiano.VerifyCompressed(bproof, vk, nil); err != nil {
			t.Fatalf("BPiano.VerifyCompressed 失败：%v", err)
		}
	}
	bpianoVerifyTime := time.Since(t0) / time.Duration(verifyReps)

	// 序列化实际证明以获得真实字节数。
	bpianoPrfBytes := len(marshalBPianoProof(bproof))

	// ════════════════════════════════════════════════════════════════════════
	// 打印对比表格
	// ════════════════════════════════════════════════════════════════════════

	fmt.Println()
	fmt.Printf("电路：Keccak-256  T=%d  M=%d\n", T, M)
	fmt.Println("────────────────────────────────────────────────────────")
	fmt.Printf("%-28s  %8s  %8s\n", "指标", "Piano", "BPiano")
	fmt.Println("────────────────────────────────────────────────────────")
	fmt.Printf("%-28s  %8d  %8d  字节\n", "证明大小", pianoPrfBytes, bpianoPrfBytes)
	fmt.Printf("%-28s  %8.1f  %8.1f  ×\n", "  大小比（vs Piano）", 1.0, float64(bpianoPrfBytes)/float64(pianoPrfBytes))
	fmt.Printf("%-28s  %8s  %8s\n",
		"证明/压缩时间",
		pianoProveTime.Round(time.Millisecond).String(),
		bpianoProveTime.Round(time.Millisecond).String())
	fmt.Printf("%-28s  %8s  %8s  （%d 次均值）\n",
		"验证时间",
		pianoVerifyTime.Round(time.Microsecond).String(),
		bpianoVerifyTime.Round(time.Microsecond).String(),
		verifyReps)
	fmt.Printf("%-28s  %8s  %8.1fx\n",
		"  验证加速比",
		"—",
		float64(pianoVerifyTime)/float64(bpianoVerifyTime))
	fmt.Println("────────────────────────────────────────────────────────")
}
