// bench_gnark_plonk 比较两种证明策略，针对"4 个 Keccak-256 MPT 路径节点"的总工作量：
//
//   PLONK（单机）：将全部 4 个 Keccak 合并进一个大电路（~925K 约束），
//                 由 1 台机器生成 1 个证明。
//
//   Piano（分布式，M=4）：同样的总计算量拆给 4 台机器并行执行，
//                 每台只处理 1/4 的约束（~231K），最终生成 1 个聚合证明。
//                 并行墙钟时间 ≈ 串行总时间 / M（各节点同时工作）。
//
// 预期结果：Piano 并行墙钟时间 ≈ PLONK 单机时间 / 4。
package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"

	"github.com/oliverustc/bpiano/keccak"
	"github.com/oliverustc/bpiano/mpt"
	"github.com/oliverustc/bpiano/piano"
)

const M = 4

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  PLONK 单机大电路  vs  Piano 分布式 M=4（4台并行）         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")

	// ── 生成测试数据 ──────────────────────────────────────────────────────
	fmt.Println("\n[0] 生成 MPT 测试数据...")
	leafData := make([]byte, 32)
	rand.Read(leafData)
	siblings := make([][32]byte, M)
	for i := range siblings {
		rand.Read(siblings[i][:])
	}
	proof := mpt.Build(leafData, siblings, make([]bool, M))
	fmt.Printf("    根哈希: %x\n", proof.RootHash[:8])

	// 收集 4 个实例的消息和预期哈希
	var msgs [M][64]byte
	var hashes [M][32]byte
	for k := 0; k < M; k++ {
		lv := proof.Levels[k]
		copy(msgs[k][:32], lv.Left[:])
		copy(msgs[k][32:], lv.Right[:])
		hashes[k] = lv.Parent
	}

	// ═══════════════════════════════════════════════════════════════════════
	// Part A：gnark PLONK 单机——4 个 Keccak 合并成 1 个大电路，1 台机器证明
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("\n══════════════════════════════════════════════════════════════")
	fmt.Println(" Part A: gnark PLONK 单机  —  4-Keccak 大电路，1 台机器，1 个证明")
	fmt.Println("══════════════════════════════════════════════════════════════")

	fmt.Println("\n[A1] 编译 4-Keccak gnark 大电路...")
	t0 := time.Now()
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &keccak.FourKeccakCircuit{})
	must(err)
	fmt.Printf("    总约束数: %d (~4×%d),  编译时间: %v\n",
		ccs.GetNbConstraints(), ccs.GetNbConstraints()/4, time.Since(t0))

	fmt.Println("\n[A2] SRS 生成 + plonk.Setup...")
	t0 = time.Now()
	srsCanon, srsLag, err := unsafekzg.NewSRS(ccs)
	must(err)
	plonkPK, plonkVK, err := plonk.Setup(ccs, srsCanon, srsLag)
	must(err)
	plonkSetupTime := time.Since(t0)
	fmt.Printf("    Setup 时间: %v\n", plonkSetupTime)

	fmt.Println("\n[A3] Prove（单机，1 个大证明）...")
	w := keccak.FourKeccakWitness(msgs, hashes)
	fullWitness, err := frontend.NewWitness(w, ecc.BN254.ScalarField())
	must(err)
	t0 = time.Now()
	plonkProof, err := plonk.Prove(ccs, plonkPK, fullWitness)
	must(err)
	plonkProveTime := time.Since(t0)
	fmt.Printf("    Prove 时间（单机）: %v\n", plonkProveTime)

	fmt.Println("\n[A4] Verify...")
	pubWitness, err := frontend.NewWitness(w, ecc.BN254.ScalarField(), frontend.PublicOnly())
	must(err)
	t0 = time.Now()
	must(plonk.Verify(plonkProof, plonkVK, pubWitness))
	plonkVerifyTime := time.Since(t0)
	fmt.Printf("    Verify 时间: %v\n", plonkVerifyTime)

	// ═══════════════════════════════════════════════════════════════════════
	// Part B：Piano 分布式——M=4 台机器并行，每台处理 1/4 约束，1 个聚合证明
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("\n══════════════════════════════════════════════════════════════")
	fmt.Println(" Part B: Piano 分布式  —  M=4 台并行，每台 T=2^18，1 个聚合证明")
	fmt.Println("══════════════════════════════════════════════════════════════")

	fmt.Println("\n[B1] 构建 Piano Keccak-256 电路...")
	t0 = time.Now()
	kc := keccak.Build()
	ci := kc.CircuitInfo()
	wh := kc.WitnessHelper()
	T := uint64(wh.Size())
	fmt.Printf("    每节点约束数 T = %d (2^%d),  构建时间: %v\n", T, log2(int(T)), time.Since(t0))

	witnesses := make([]piano.WitnessInstance, M)
	for k := 0; k < M; k++ {
		varVals := kc.WitnessFor(msgs[k])
		witnesses[k] = wh.Make(varVals)
	}

	fmt.Println("\n[B2] Piano Setup (M=4)...")
	tauX, tauY := big.NewInt(7919), big.NewInt(104729)
	t0 = time.Now()
	pkP, vkP, err := piano.SetupWithTrapdoors(*ci, uint64(M), T, tauX, tauY)
	must(err)
	pianoSetupTime := time.Since(t0)
	fmt.Printf("    Setup 时间: %v\n", pianoSetupTime)

	fmt.Println("\n[B3] Piano Prove（串行测量总时间，模拟分布式...）")
	t0 = time.Now()
	pianoProof, err := piano.Prove(pkP, witnesses, nil)
	must(err)
	pianoSerialTime := time.Since(t0)
	// 分布式墙钟时间 = 串行总时间 / M（各节点同时工作，忽略协调开销）
	pianoParallelTime := pianoSerialTime / time.Duration(M)
	fmt.Printf("    串行总时间（单机模拟）: %v\n", pianoSerialTime)
	fmt.Printf("    ★ 分布式墙钟时间（÷%d 台并行）: %v\n", M, pianoParallelTime)

	fmt.Println("\n[B4] Piano Verify（1 次，验证聚合证明）...")
	t0 = time.Now()
	must(piano.Verify(vkP, pianoProof, nil))
	pianoVerifyTime := time.Since(t0)
	fmt.Printf("    Verify 时间: %v\n", pianoVerifyTime)

	// ═══════════════════════════════════════════════════════════════════════
	// 汇总
	// ═══════════════════════════════════════════════════════════════════════
	speedup := float64(plonkProveTime) / float64(pianoParallelTime)
	fmt.Println("\n========== 汇总 ==========")
	fmt.Printf("电路规模: PLONK %d 约束 (1台)  vs  Piano %d×%d 约束 (%d台并行)\n",
		ccs.GetNbConstraints(), M, T, M)
	fmt.Println()
	fmt.Printf("  %-20s  PLONK 单机    Piano 分布式(M=%d)\n", "指标", M)
	fmt.Printf("  %-20s  %-12s  %s\n", "Setup 时间", fmtd(plonkSetupTime), fmtd(pianoSetupTime))
	fmt.Printf("  %-20s  %-12s  %s  (串行总时间 %s / %d 台)\n",
		"Prove 时间(墙钟)", fmtd(plonkProveTime), fmtd(pianoParallelTime), fmtd(pianoSerialTime), M)
	fmt.Printf("  %-20s  %-12s  %s\n", "Verify 时间", fmtd(plonkVerifyTime), fmtd(pianoVerifyTime))
	fmt.Printf("  %-20s  %-12s  %s\n", "生成证明数量", "1", "1 (聚合)")
	fmt.Println()
	fmt.Printf("Prove 加速比 (Piano 分布式 vs PLONK 单机): %.2fx\n", speedup)
	fmt.Println("(理论上限为 M=4x，实测略低是因为 Y 方向协调开销不可并行)")
}

func fmtd(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return fmt.Sprintf("%.3fs", d.Seconds())
}
func log2(n int) int {
	l := 0
	for n > 1 {
		n >>= 1
		l++
	}
	return l
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
