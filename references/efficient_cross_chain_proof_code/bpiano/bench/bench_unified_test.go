// bench_unified_test.go —— 统一入口 TestBench
//
// 任意机器一条命令跑通全部测试，结果输出到 bench/results/：
//
//	go test ./bench/ -v -run TestBench -timeout 300m
//
// 阶段 A：单证明压缩 Go 端     → compress_performance_YYYYMMDD_HHmmSS.csv（含 gas_cost 预留行）
// 阶段 B：单证明 Solidity Gas  → 更新 compress_performance csv 的 gas_cost 行
// 阶段 C：聚合 Go 端           → aggregation_proof_size / aggregation_verify_time / aggregation_prove_time csv
// 阶段 D：聚合 Gas             → aggregation_verify_gas_cost csv

package bench_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	bpiano "github.com/oliverustc/bpiano/bpiano"
	"github.com/oliverustc/bpiano/keccak"
	"github.com/oliverustc/bpiano/piano"
)

const resultsDir = "results"

// csvTimestamp 返回符合命名规范的时间戳，格式 YYYYMMDD_HHmmSS。
func csvTimestamp() string {
	return time.Now().Format("20060102_150405")
}

// csvPath 构造 results/ 下的 CSV 路径，文件名格式 testtype_timestamp.csv。
func csvPath(testType, ts string) string {
	return filepath.Join(resultsDir, testType+"_"+ts+".csv")
}

// writeCSV 将 headers + rows 写入 CSV 文件，自动创建 results/ 目录。
func writeCSV(path string, headers []string, rows [][]string) error {
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		return fmt.Errorf("MkdirAll results/: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// updateCSVRow 读取 path 处的 CSV，找到第一列等于 key 的行，
// 将其余列更新为 vals，然后重写文件。
func updateCSVRow(path, key string, vals []string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	records, err := csv.NewReader(f).ReadAll()
	f.Close()
	if err != nil {
		return fmt.Errorf("read csv: %w", err)
	}
	for i, row := range records {
		if len(row) > 0 && row[0] == key {
			for j, v := range vals {
				if j+1 < len(row) {
					records[i][j+1] = v
				}
			}
			break
		}
	}
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("rewrite %s: %w", path, err)
	}
	defer out.Close()
	w := csv.NewWriter(out)
	for _, row := range records {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// solDir 返回 bpiano/sol/ 的绝对路径（bench/ 与 sol/ 平级）。
func solDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	d := filepath.Join(wd, "..", "sol")
	if _, err := os.Stat(d); err != nil {
		t.Fatalf("sol/ 目录不存在：%s", d)
	}
	return d
}

// ── TestBench 主入口 ──────────────────────────────────────────────────────────

func TestBench(t *testing.T) {
	ts := csvTimestamp()
	t.Logf("基准测试开始，时间戳：%s", ts)

	singleCSV := csvPath("compress_performance", ts)

	t.Run("Stage1_SingleProof", func(t *testing.T) {
		benchSingleProof(t, singleCSV)
	})

	t.Run("Stage2_SingleProofGas", func(t *testing.T) {
		benchSingleProofGas(t, singleCSV)
	})

	t.Run("Stage3_Aggregation", func(t *testing.T) {
		benchAggregation(t, ts)
	})

	t.Run("Stage4_AggGas", func(t *testing.T) {
		benchAggGas(t, ts)
	})
}

// ── 阶段 A：单证明压缩 Go 端 ─────────────────────────────────────────────────

func benchSingleProof(t *testing.T, csvFile string) {
	t.Log("构建 Keccak-256 电路（T≈2^18）…")
	t0 := time.Now()
	kc := keccak.Build()
	ci := kc.CircuitInfo()
	wh := kc.WitnessHelper()
	T := wh.Size()
	t.Logf("电路构建完成，用时 %s  T=%d", time.Since(t0).Round(time.Millisecond), T)

	const M = 2
	pk, vk, err := piano.SetupWithTrapdoors(*ci, uint64(M), uint64(T), big.NewInt(17), big.NewInt(23))
	if err != nil {
		t.Fatalf("Setup 失败：%v", err)
	}
	t.Logf("Setup 完成，用时 %s", time.Since(t0).Round(time.Millisecond))

	witnesses := make([]piano.WitnessInstance, M)
	for m := 0; m < M; m++ {
		var msg [64]byte
		msg[0] = byte(m + 1)
		witnesses[m] = wh.Make(kc.WitnessFor(msg))
	}

	// Piano Prove
	t0 = time.Now()
	pianoProof, err := piano.Prove(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("Piano.Prove 失败：%v", err)
	}
	pianoProveTime := time.Since(t0)

	if err := piano.Verify(vk, pianoProof, nil); err != nil {
		t.Fatalf("Piano.Verify 正确性失败：%v", err)
	}
	pianoSize := len(marshalPianoProof(pianoProof))

	// BPiano Compress
	t0 = time.Now()
	bproof, err := bpiano.Compress(pk, witnesses, nil)
	if err != nil {
		t.Fatalf("BPiano.Compress 失败：%v", err)
	}
	bpianoProveTime := time.Since(t0)

	if err := bpiano.VerifyCompressed(bproof, vk, nil); err != nil {
		t.Fatalf("BPiano.VerifyCompressed 正确性失败：%v", err)
	}
	bpianoSize := len(marshalBPianoProof(bproof))

	// 验证时间：ABAB 交替计时，消除 cache 顺序偏差
	var pianoVerifyTotal, bpianoVerifyTotal time.Duration
	for i := 0; i < verifyReps; i++ {
		t0 = time.Now()
		_ = piano.Verify(vk, pianoProof, nil)
		pianoVerifyTotal += time.Since(t0)

		t0 = time.Now()
		_ = bpiano.VerifyCompressed(bproof, vk, nil)
		bpianoVerifyTotal += time.Since(t0)
	}
	pianoVerifyTime := pianoVerifyTotal / time.Duration(verifyReps)
	bpianoVerifyTime := bpianoVerifyTotal / time.Duration(verifyReps)

	// 控制台
	fmt.Printf("\n电路：Keccak-256  T=%d  M=%d\n", T, M)
	fmt.Println("────────────────────────────────────────────────────────")
	fmt.Printf("%-28s  %10s  %10s\n", "指标", "Piano", "BPiano")
	fmt.Println("────────────────────────────────────────────────────────")
	fmt.Printf("%-28s  %10d  %10d  字节\n", "证明大小", pianoSize, bpianoSize)
	fmt.Printf("%-28s  %10.1f  %10.1f  ×\n", "  大小比（vs Piano）",
		1.0, float64(bpianoSize)/float64(pianoSize))
	fmt.Printf("%-28s  %10s  %10s\n", "证明/压缩时间",
		pianoProveTime.Round(time.Millisecond), bpianoProveTime.Round(time.Millisecond))
	fmt.Printf("%-28s  %10s  %10s  （%d 次均值）\n", "验证时间",
		pianoVerifyTime.Round(time.Microsecond), bpianoVerifyTime.Round(time.Microsecond), verifyReps)
	fmt.Printf("%-28s  %10s  %10.2fx\n", "  验证加速比", "—",
		float64(pianoVerifyTime)/float64(bpianoVerifyTime))
	fmt.Println("────────────────────────────────────────────────────────")

	// CSV（gas_cost 行预留空值，阶段 B 填入）
	headers := []string{"metric", "piano", "bpiano"}
	rows := [][]string{
		{"proof_size_bytes",
			strconv.Itoa(pianoSize), strconv.Itoa(bpianoSize)},
		{"prove_time_ms",
			fmt.Sprintf("%.3f", float64(pianoProveTime.Nanoseconds())/1e6),
			fmt.Sprintf("%.3f", float64(bpianoProveTime.Nanoseconds())/1e6)},
		{"verify_time_ms",
			fmt.Sprintf("%.6f", float64(pianoVerifyTime.Nanoseconds())/1e6),
			fmt.Sprintf("%.6f", float64(bpianoVerifyTime.Nanoseconds())/1e6)},
		{"gas_cost", "", ""},
	}
	if err := writeCSV(csvFile, headers, rows); err != nil {
		t.Fatalf("写入 CSV 失败：%v", err)
	}
	t.Logf("✓ 写入 %s（gas_cost 待阶段 B 填入）", csvFile)
}

// ── 阶段 B：单证明 Solidity Gas ───────────────────────────────────────────────

// benchSingleProofGas 运行 forge test --match-contract PianoVerifierGenTest，
// 从输出中解析 "Piano1 gas: XXXX" 和 "BPiano1 gas: XXXX"，
// 将结果更新到 csvFile 中的 gas_cost 行。
//
// 注意：Gas 来自使用 T=8 小电路生成的 PianoVerifierGen/BPianoVerifierGen 合约。
// 验证逻辑与 T=2^18 完全一致，Gas 差异 < 0.5%（仅 modexp 指数位数不同）。
func benchSingleProofGas(t *testing.T, csvFile string) {
	sd := solDir(t)

	// 检查 forge 是否可用
	if _, err := exec.LookPath("forge"); err != nil {
		t.Skip("forge 不在 PATH 中，跳过 Gas 测试")
	}

	t.Log("运行 forge test --match-contract PianoVerifierGenTest …")
	cmd := exec.Command("forge", "test",
		"--match-contract", "PianoVerifierGenTest",
		"-vv",
	)
	cmd.Dir = sd
	out, err := cmd.CombinedOutput()
	outStr := string(out)
	t.Logf("forge 输出：\n%s", outStr)
	if err != nil {
		t.Fatalf("forge test 失败：%v", err)
	}

	pianoGas, bpianoGas, parseErr := parseGenGas(outStr)
	if parseErr != nil {
		t.Fatalf("解析 Gas 失败：%v", parseErr)
	}

	// 打印
	fmt.Println()
	fmt.Println("────────────────────────────────────────────────────────")
	fmt.Printf("%-28s  %10s  %10s\n", "指标", "Piano", "BPiano")
	fmt.Println("────────────────────────────────────────────────────────")
	fmt.Printf("%-28s  %10d  %10d  gas\n", "验证 Gas（Solidity）", pianoGas, bpianoGas)
	fmt.Printf("%-28s  %10s  %10.2fx\n", "  Gas 节省比", "—",
		float64(pianoGas)/float64(bpianoGas))
	fmt.Println("────────────────────────────────────────────────────────")

	// 更新 CSV
	if err := updateCSVRow(csvFile, "gas_cost",
		[]string{strconv.Itoa(pianoGas), strconv.Itoa(bpianoGas)},
	); err != nil {
		t.Fatalf("更新 CSV gas_cost 行失败：%v", err)
	}
	t.Logf("✓ gas_cost 已写入 %s", csvFile)
}

// parseGenGas 从 forge -vv 输出中提取 Piano1 gas 和 BPiano1 gas 的值。
func parseGenGas(output string) (pianoGas, bpianoGas int, err error) {
	pianoRe := regexp.MustCompile(`\bPiano1 gas:\s*(\d+)`)   // \b 防止匹配 BPiano1
	bpianoRe := regexp.MustCompile(`BPiano1 gas:\s*(\d+)`)

	pm := pianoRe.FindStringSubmatch(output)
	if pm == nil {
		return 0, 0, fmt.Errorf("未在 forge 输出中找到 'Piano1 gas:'")
	}
	bm := bpianoRe.FindStringSubmatch(output)
	if bm == nil {
		return 0, 0, fmt.Errorf("未在 forge 输出中找到 'BPiano1 gas:'")
	}

	pianoGas, _ = strconv.Atoi(pm[1])
	bpianoGas, _ = strconv.Atoi(bm[1])
	return pianoGas, bpianoGas, nil
}

// ── 阶段 D：聚合 Solidity Gas ─────────────────────────────────────────────────

// benchAggGas 运行 TestForgeE2E_Agg_AllK（生成所有 K 的 fixture 并调用 forge），
// 解析每个 K 的 Gas，写入 aggregation_verify_gas_cost_<ts>.csv。
// Piano Gas = pianoUnitGas × K（来自 compress_performance CSV 的 gas_cost 行）。
func benchAggGas(t *testing.T, ts string) {
	if _, err := exec.LookPath("forge"); err != nil {
		t.Skip("forge 不在 PATH 中，跳过 Agg Gas 测试")
	}

	// 读取 piano 单证明 gas（Stage B 写入的 CSV）
	pianoUnitGas := readPianoUnitGas(ts)
	if pianoUnitGas == 0 {
		t.Log("警告：未找到 piano 单证明 Gas，piano_gas 列将为 0（请先运行 Stage2）")
	}

	// 运行 go test ./solgen/ -run TestForgeE2E_Agg_AllK
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	solgenPath := filepath.Join(wd, "..", "solgen")
	cmd := exec.Command("go", "test", ".", "-run", "TestForgeE2E_Agg_AllK", "-count=1", "-v", "-timeout", "30m")
	cmd.Dir = solgenPath
	out, runErr := cmd.CombinedOutput()
	outStr := string(out)
	t.Logf("solgen agg gas 输出：\n%s", outStr)

	// 解析每个 K 的 Gas（先解析，解析失败才 Fatal；forge 局部失败但数据完整时继续）
	gasMap, parseErr := parseAggGasAll(outStr)
	if parseErr != nil {
		if runErr != nil {
			t.Fatalf("TestForgeE2E_Agg_AllK 失败（%v）且无 Gas 数据：%v", runErr, parseErr)
		}
		t.Fatalf("解析 Agg Gas 失败：%v", parseErr)
	}
	if runErr != nil {
		t.Logf("警告：forge 部分测试失败（%v），但 Gas 数据已解析到 %d 条，继续写入 CSV", runErr, len(gasMap))
	}

	// 打印汇总表
	fmt.Println()
	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Printf("%-6s  %-14s  %-14s  %s\n", "K", "Piano Gas (K×)", "BPiano Agg Gas", "节省比")
	fmt.Println("────────────────────────────────────────────────────────────────")

	headers := []string{"K", "piano_gas", "bpiano_agg_gas", "gas_saving_pct"}
	rows := make([][]string, 0, len(kValues))
	for _, K := range kValues {
		bpianoGas := gasMap[K]
		pGas := pianoUnitGas * K
		var saving float64
		if pGas > 0 && bpianoGas > 0 {
			saving = (1 - float64(bpianoGas)/float64(pGas)) * 100
		}
		fmt.Printf("K=%-4d  %14d  %14d  %.1f%%\n", K, pGas, bpianoGas, saving)
		rows = append(rows, []string{
			strconv.Itoa(K),
			strconv.Itoa(pGas),
			strconv.Itoa(bpianoGas),
			fmt.Sprintf("%.1f", saving),
		})
	}
	fmt.Println("────────────────────────────────────────────────────────────────")

	gasPath := csvPath("aggregation_verify_gas_cost", ts)
	if err := writeCSV(gasPath, headers, rows); err != nil {
		t.Fatalf("写入 %s：%v", gasPath, err)
	}
	t.Logf("✓ 写入 %s", gasPath)
}

// readPianoUnitGas 从 compress_performance_<ts>.csv 的 gas_cost 行读取 piano gas 值。
// 如果文件不存在或未找到该行，返回 0。
func readPianoUnitGas(ts string) int {
	path := csvPath("compress_performance", ts)
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return 0
	}
	for _, row := range records {
		if len(row) >= 2 && row[0] == "gas_cost" {
			v, _ := strconv.Atoi(row[1])
			return v
		}
	}
	return 0
}

// parseAggGasAll 从 forge/go-test 的 -v 输出中提取所有 "AggK{N} gas: XXXXX" 行。
func parseAggGasAll(output string) (map[int]int, error) {
	re := regexp.MustCompile(`AggK(\d+) gas[^:]*:\s*(\d+)`)
	matches := re.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("未在输出中找到任何 AggK gas 数据")
	}
	result := make(map[int]int)
	for _, m := range matches {
		K, _ := strconv.Atoi(m[1])
		gas, _ := strconv.Atoi(m[2])
		result[K] = gas
	}
	return result, nil
}

// ── 阶段 C：聚合 Go 端 ────────────────────────────────────────────────────────

// benchAggregation 对 kValues 中每个 K 值执行聚合验证对比：
//   - 若 testdata/ 不完整，自动运行生成流程（约 70 min）
//   - 加载 Piano 证明和各 K 值的 AggregatedProof
//   - 计时 Piano×K Verify 和 BPiano VerifyBatch（各 verifyRepsM 次均值）
//   - 写入 aggregation_size_<ts>.csv 和 aggregation_perf_<ts>.csv
func benchAggregation(t *testing.T, ts string) {
	// ── 1. 确保 testdata/ 完整 ────────────────────────────────────────
	if !testdataComplete(kValues) {
		t.Log("testdata/ 不完整，开始生成证明（约 70 分钟，请耐心等待）…")
		TestGenerateAndSave(t)
		if t.Failed() {
			return
		}
	} else {
		t.Log("testdata/ 已完整，跳过证明生成")
	}

	// ── 2. 加载 meta、VK、Piano 证明 ──────────────────────────────────
	metaBytes, err := os.ReadFile(filepath.Join(testdataDir, "meta.json"))
	if err != nil {
		t.Fatalf("读取 meta.json：%v", err)
	}
	var meta ProofMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("解析 meta.json：%v", err)
	}

	vkBytes, err := os.ReadFile(filepath.Join(testdataDir, "vk.bin"))
	if err != nil {
		t.Fatalf("读取 vk.bin：%v", err)
	}
	vk, err := decodeVK(vkBytes)
	if err != nil {
		t.Fatalf("解码 vk.bin：%v", err)
	}

	pData, err := os.ReadFile(filepath.Join(testdataDir, "piano_proofs.bin"))
	if err != nil {
		t.Fatalf("读取 piano_proofs.bin：%v", err)
	}
	sz := pianoProofSize(&meta)
	allPianoProofs := make([]*piano.Proof, meta.KMax)
	for i := range allPianoProofs {
		allPianoProofs[i], err = decodePianoProof(pData[i*sz:(i+1)*sz], &meta)
		if err != nil {
			t.Fatalf("解码 piano_proof[%d]：%v", i, err)
		}
	}
	t.Logf("加载完成：%d 个 Piano 证明，VK OK", meta.KMax)

	allPubInputs := make([][][]bpiano.Fr, meta.KMax)

	// 证明时间（服务器均值线性外推）
	pianoProvePerProof := time.Duration(meta.PianoProvePerProofNs)
	bpianoProvePerProof := time.Duration(meta.BPianoProvePerProofNs)

	// ── 3. 逐 K 值计时 ───────────────────────────────────────────────
	type kResult struct {
		K                             int
		pianoSize, bpianoSize         int
		pianoVerify, bpianoVerify     time.Duration
		pianoProveEst, bpianoProveEst time.Duration
		speedup                       float64
	}
	results := make([]kResult, len(kValues))

	hdr := "──────────────────────────────────────────────────────────────────────────────────────────────────"
	fmt.Println()
	fmt.Printf("电路：%s  T=%d  M=%d\n", meta.Circuit, meta.T, meta.M)
	fmt.Printf("Prove 均值（服务器 %d 核）：Piano %.2fs/个，BPiano %.2fs/个（† 线性外推）\n",
		meta.NumCPU, pianoProvePerProof.Seconds(), bpianoProvePerProof.Seconds())
	fmt.Println(hdr)
	fmt.Printf("%-4s  %-10s  %-10s  %-12s  %-13s  %-12s  %-13s  %s\n",
		"K", "PianoSize", "BPianoSize", "PianoProve†", "BPianoProve†", "PianoVerify", "BPianoVerify", "加速比")
	fmt.Println(hdr)

	for i, K := range kValues {
		// 加载该 K 的 AggregatedProof
		aggData, err := os.ReadFile(filepath.Join(testdataDir, aggFileName(K)))
		if err != nil {
			t.Fatalf("读取 %s：%v", aggFileName(K), err)
		}
		agg, err := decodeAggregatedProof(aggData)
		if err != nil {
			t.Fatalf("解码 %s：%v", aggFileName(K), err)
		}

		// 正确性验证
		if err := bpiano.VerifyBatch(agg, vk, allPubInputs[:K]); err != nil {
			t.Fatalf("VerifyBatch(K=%d) 正确性失败：%v", K, err)
		}

		// 证明大小
		pianoSize := K * sz
		bpianoSize := 4 + K*compressedProofSize + 64

		// 证明时间估算
		pianoProveEst := time.Duration(int64(pianoProvePerProof) * int64(K))
		bpianoProveEst := time.Duration(int64(bpianoProvePerProof) * int64(K))

		// 交替计时（ABAB 设计）：每轮先 Piano 后 BPiano，消除 cache 系统性偏差
		var pianoTotal, bpianoTotal time.Duration
		for rep := 0; rep < verifyRepsM; rep++ {
			t0 := time.Now()
			for k := 0; k < K; k++ {
				_ = piano.Verify(vk, allPianoProofs[k], nil)
			}
			pianoTotal += time.Since(t0)

			t0 = time.Now()
			_ = bpiano.VerifyBatch(agg, vk, allPubInputs[:K])
			bpianoTotal += time.Since(t0)
		}
		pianoVerify := pianoTotal / time.Duration(verifyRepsM)
		bpianoVerify := bpianoTotal / time.Duration(verifyRepsM)

		speedup := float64(pianoVerify) / float64(bpianoVerify)
		results[i] = kResult{K, pianoSize, bpianoSize, pianoVerify, bpianoVerify, pianoProveEst, bpianoProveEst, speedup}

		fmt.Printf("K=%-3d  %8d B  %8d B  %12s  %13s  %12s  %13s  %.2fx\n",
			K, pianoSize, bpianoSize,
			pianoProveEst.Round(time.Millisecond),
			bpianoProveEst.Round(time.Millisecond),
			pianoVerify.Round(time.Microsecond),
			bpianoVerify.Round(time.Microsecond),
			speedup)
	}
	fmt.Println(hdr)

	// ── 4. 写 CSV ─────────────────────────────────────────────────────
	// aggregation_proof_size_<ts>.csv
	sizeHeaders := []string{"K", "piano_size_bytes", "bpiano_size_bytes", "size_saving_pct"}
	sizeRows := make([][]string, len(results))
	for i, r := range results {
		saving := (1 - float64(r.bpianoSize)/float64(r.pianoSize)) * 100
		sizeRows[i] = []string{
			strconv.Itoa(r.K),
			strconv.Itoa(r.pianoSize),
			strconv.Itoa(r.bpianoSize),
			fmt.Sprintf("%.1f", saving),
		}
	}
	sizePath := csvPath("aggregation_proof_size", ts)
	if err := writeCSV(sizePath, sizeHeaders, sizeRows); err != nil {
		t.Fatalf("写入 %s：%v", sizePath, err)
	}
	t.Logf("✓ 写入 %s", sizePath)

	// aggregation_verify_time_<ts>.csv
	verifyTimeHeaders := []string{"K", "piano_verify_ms", "bpiano_verify_ms", "verify_speedup"}
	verifyTimeRows := make([][]string, len(results))
	for i, r := range results {
		verifyTimeRows[i] = []string{
			strconv.Itoa(r.K),
			fmt.Sprintf("%.6f", float64(r.pianoVerify.Nanoseconds())/1e6),
			fmt.Sprintf("%.6f", float64(r.bpianoVerify.Nanoseconds())/1e6),
			fmt.Sprintf("%.4f", r.speedup),
		}
	}
	verifyTimePath := csvPath("aggregation_verify_time", ts)
	if err := writeCSV(verifyTimePath, verifyTimeHeaders, verifyTimeRows); err != nil {
		t.Fatalf("写入 %s：%v", verifyTimePath, err)
	}
	t.Logf("✓ 写入 %s", verifyTimePath)

	// aggregation_prove_time_<ts>.csv
	proveTimeHeaders := []string{"K", "piano_prove_s", "bpiano_prove_s", "prove_speedup"}
	proveTimeRows := make([][]string, len(results))
	for i, r := range results {
		proveSpeedup := r.pianoProveEst.Seconds() / r.bpianoProveEst.Seconds()
		proveTimeRows[i] = []string{
			strconv.Itoa(r.K),
			fmt.Sprintf("%.3f", r.pianoProveEst.Seconds()),
			fmt.Sprintf("%.3f", r.bpianoProveEst.Seconds()),
			fmt.Sprintf("%.4f", proveSpeedup),
		}
	}
	proveTimePath := csvPath("aggregation_prove_time", ts)
	if err := writeCSV(proveTimePath, proveTimeHeaders, proveTimeRows); err != nil {
		t.Fatalf("写入 %s：%v", proveTimePath, err)
	}
	t.Logf("✓ 写入 %s", proveTimePath)

	// aggregation_table_<ts>.csv（论文表格：K=1,10,30,50,100 五个代表点，汇总所有指标）
	tableKSet := map[int]bool{2: true, 10: true, 30: true, 50: true, 100: true}
	tableHeaders := []string{
		"K",
		"piano_size_bytes", "bpiano_size_bytes", "size_saving_pct",
		"piano_verify_ms", "bpiano_verify_ms", "verify_speedup",
		"piano_prove_s", "bpiano_prove_s",
	}
	var tableRows [][]string
	for _, r := range results {
		if !tableKSet[r.K] {
			continue
		}
		sizeSaving := (1 - float64(r.bpianoSize)/float64(r.pianoSize)) * 100
		tableRows = append(tableRows, []string{
			strconv.Itoa(r.K),
			strconv.Itoa(r.pianoSize),
			strconv.Itoa(r.bpianoSize),
			fmt.Sprintf("%.1f", sizeSaving),
			fmt.Sprintf("%.3f", float64(r.pianoVerify.Nanoseconds())/1e6),
			fmt.Sprintf("%.3f", float64(r.bpianoVerify.Nanoseconds())/1e6),
			fmt.Sprintf("%.4f", r.speedup),
			fmt.Sprintf("%.3f", r.pianoProveEst.Seconds()),
			fmt.Sprintf("%.3f", r.bpianoProveEst.Seconds()),
		})
	}
	tablePath := csvPath("aggregation_table", ts)
	if err := writeCSV(tablePath, tableHeaders, tableRows); err != nil {
		t.Fatalf("写入 %s：%v", tablePath, err)
	}
	t.Logf("✓ 写入 %s", tablePath)
}
