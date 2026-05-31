package keccak

import (
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/oliverustc/bpiano/circuit"
	"github.com/oliverustc/bpiano/piano"
)

// StateVars 以变量索引形式表示 Keccak 状态。
type StateVars [5][5][64]int

// KeccakCircuit 保存电路结构及变量索引。
type KeccakCircuit struct {
	builder    *circuit.Builder
	InputVars  [512]int // 消息输入位的变量索引（512 位 = 64 字节）
	OutputVars [256]int // 哈希输出位的变量索引
	wh         *circuit.WitnessHelper
	nVarsBefore int // 加入本电路前 b.NbVars() 的值
	nVars       int // Build 后的变量总数
	ci          *piano.CircuitInfo
}

// Build 为 64 字节原像构造 Keccak-256 电路。
// 电路行数补齐至 2 的幂（约 T=2^18）。
func Build() *KeccakCircuit {
	b := circuit.NewBuilder(300000)
	b.Init()

	kc := &KeccakCircuit{
		builder:     b,
		nVarsBefore: b.NbVars(),
	}

	kc.InputVars, kc.OutputVars = buildKeccakGates(b)
	kc.nVars = b.NbVars()

	var ci *piano.CircuitInfo
	var wh *circuit.WitnessHelper
	ci, wh = b.Build()
	kc.ci = ci
	kc.wh = wh

	return kc
}

// CircuitInfo 返回 Build() 之后的电路信息。
func (kc *KeccakCircuit) CircuitInfo() *piano.CircuitInfo {
	return kc.ci
}

// WitnessHelper 返回 witness 辅助对象。
func (kc *KeccakCircuit) WitnessHelper() *circuit.WitnessHelper {
	return kc.wh
}

// buildKeccakGates 向 b 中添加所有 Keccak-256 门，并返回输入/输出变量索引。
func buildKeccakGates(b *circuit.Builder) (inputVars [512]int, outputVars [256]int) {
	// ── 分配输入位变量（512 位 = 64 字节消息）──
	for i := 0; i < 512; i++ {
		v := b.NewVar()
		inputVars[i] = v
		b.BoolCheck(v)
	}

	// ── 构造初始 Keccak 状态（速率=1088 位，容量=512 位）──
	// 状态为 1600 位 = 25 个 lane × 64 位。
	// 速率 = 状态的第 0..1087 位。
	// 容量 = 第 1088..1599 位，全为零。
	//
	// 64 字节消息（速率=136 字节）的填充规则：
	//   字节 0..63：消息
	//   字节 64：   0x01（Keccak 填充，位 0=1，其余为 0）
	//   字节 65..134：0x00
	//   字节 135：  0x80（位 7=1，其余为 0）
	//
	// 按位展开（每字节 LSB 优先）：
	//   位 0..511：   消息位（变量 inputVars[0..511]）
	//   位 512：      1（字节 64 的位 0）
	//   位 513..1079：0
	//   位 1087：     1（字节 135 的位 7 = 0x80）
	//   位 1080..1086：0（字节 135 的位 0-6）
	//   位 1088..1599：0（容量位）

	// 分配用于填充位的常数 1 变量。
	constOne1 := b.NewVar()
	b.AddGate(constOne1, b.ZeroVar(), b.ZeroVar(), 1, 0, 0, 0, -1) // constOne1 - 1 = 0

	constOne2 := b.NewVar()
	b.AddGate(constOne2, b.ZeroVar(), b.ZeroVar(), 1, 0, 0, 0, -1) // constOne2 - 1 = 0

	// 以 [5][5][64]int 形式构建状态变量。
	// State[x][y][z] 对应 lane A[x][y] 的第 z 位。
	// Lane 编号规则：word_i = A[i%5][i/5]。
	// 1600 位的布局：位 (y*5+x)*64 + z = A[x][y][z]。
	// 速率位为线性化状态的第 0..1087 位。
	// 线性索引 = (lane 索引)*64 + lane 内偏移
	// A[x][y] 的 lane 索引 = x + 5*y（标准 word 排序 word_i = A[i%5][i/5]）。

	var stateVars StateVars
	for i := 0; i < 25; i++ {
		x := i % 5
		y := i / 5
		for z := 0; z < 64; z++ {
			bitIdx := i*64 + z
			switch {
			case bitIdx < 512:
				// 消息位
				stateVars[x][y][z] = inputVars[bitIdx]
			case bitIdx == 512:
				// 第一个填充位 = 1
				stateVars[x][y][z] = constOne1
			case bitIdx == 1087:
				// 最后一个速率位 = 1（0x80 填充）
				stateVars[x][y][z] = constOne2
			case bitIdx < 1088:
				// 其他速率位 = 0
				stateVars[x][y][z] = b.ZeroVar()
			default:
				// 容量位 = 0
				stateVars[x][y][z] = b.ZeroVar()
			}
		}
	}

	// ── 执行 24 轮 Keccak ──
	for round := 0; round < 24; round++ {
		stateVars = keccakRoundCircuit(b, stateVars, round)
	}

	// ── 提取输出：前 256 位 ──
	// 输出位来自 word 0,1,2,3（lane A[0][0], A[1][0], A[2][0], A[3][0]），
	// 按小端位序排列。
	for i := 0; i < 4; i++ {
		x := i % 5
		y := i / 5
		for z := 0; z < 64; z++ {
			outputVars[i*64+z] = stateVars[x][y][z]
		}
	}

	return inputVars, outputVars
}

// keccakRoundCircuit 使用 PLONK 门对 stateVars 执行一轮 Keccak 变换。
func keccakRoundCircuit(b *circuit.Builder, s StateVars, round int) StateVars {
	// ── θ 步骤 ──
	s = thetaCircuit(b, s)
	// ── ρ+π 步骤（仅重新索引，无需添加门）──
	s = rhoPiCircuit(s)
	// ── χ 步骤 ──
	s = chiCircuit(b, s)
	// ── ι 步骤 ──
	s = iotaCircuit(b, s, round)
	return s
}

// thetaCircuit 使用 XOR 门执行 θ 步骤。
func thetaCircuit(b *circuit.Builder, s StateVars) StateVars {
	// C[x][z] = 5 个 lane 位的异或
	var C [5][64]int
	for x := 0; x < 5; x++ {
		for z := 0; z < 64; z++ {
			t1 := b.XOR(s[x][0][z], s[x][1][z])
			t2 := b.XOR(t1, s[x][2][z])
			t3 := b.XOR(t2, s[x][3][z])
			C[x][z] = b.XOR(t3, s[x][4][z])
		}
	}

	// D[x][z] = C[(x+4)%5][z] XOR C[(x+1)%5][(z+63)%64]
	var D [5][64]int
	for x := 0; x < 5; x++ {
		for z := 0; z < 64; z++ {
			D[x][z] = b.XOR(C[(x+4)%5][z], C[(x+1)%5][(z+63)%64])
		}
	}

	// state'[x][y][z] = state[x][y][z] XOR D[x][z]（原地更新）
	var newS StateVars
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 64; z++ {
				newS[x][y][z] = b.XOR(s[x][y][z], D[x][z])
			}
		}
	}
	return newS
}

// rhoPiCircuit 通过重新索引变量执行 ρ+π（无需添加门）。
func rhoPiCircuit(s StateVars) StateVars {
	var newS StateVars
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			newX := y
			newY := (2*x + 3*y) % 5
			for z := 0; z < 64; z++ {
				zSrc := (z - rhoOffsets[x][y] + 64) % 64
				newS[newX][newY][z] = s[x][y][zSrc]
			}
		}
	}
	return newS
}

// chiCircuit 使用 NOT 和 AND 门执行 χ 步骤。
func chiCircuit(b *circuit.Builder, s StateVars) StateVars {
	// 保存当前状态的副本，避免读到已修改的值。
	old := s
	var newS StateVars
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 64; z++ {
				notNext := b.NOT(old[(x+1)%5][y][z])
				andResult := b.AND(notNext, old[(x+2)%5][y][z])
				newS[x][y][z] = b.XOR(old[x][y][z], andResult)
			}
		}
	}
	return newS
}

// iotaCircuit 执行 ι 步骤：将轮常数各位异或进 state[0][0]。
func iotaCircuit(b *circuit.Builder, s StateVars, round int) StateVars {
	rc := keccakRC[round]
	for z := 0; z < 64; z++ {
		if (rc>>uint(z))&1 == 1 {
			// 与 1 异或 = NOT
			s[0][0][z] = b.NOT(s[0][0][z])
		}
		// 否则：保持不变
	}
	return s
}

// ────────────────────────────────────────────────────────────────────────────
// Witness 计算
// ────────────────────────────────────────────────────────────────────────────

// traceBits 在 witness 计算过程中跟踪变量取值。
// 其变量分配顺序与 buildKeccakGates 完全对应。
type traceBits struct {
	varVals    []fr.Element
	nextVarIdx int
}

func newTraceBits(nVars int, startIdx int) *traceBits {
	return &traceBits{
		varVals:    make([]fr.Element, nVars),
		nextVarIdx: startIdx,
	}
}

func (tb *traceBits) allocBool(val bool) int {
	idx := tb.nextVarIdx
	tb.nextVarIdx++
	if val {
		tb.varVals[idx].SetOne()
	}
	// BoolCheck 不分配新变量，因此不需要额外递增。
	return idx
}

func (tb *traceBits) allocConstOne() int {
	idx := tb.nextVarIdx
	tb.nextVarIdx++
	tb.varVals[idx].SetOne()
	// 添加了 Ql=1, Qk=-1 的门，但不再分配新变量。
	return idx
}

func (tb *traceBits) xor(a, b int) int {
	idx := tb.nextVarIdx
	tb.nextVarIdx++
	aVal := !tb.varVals[a].IsZero()
	bVal := !tb.varVals[b].IsZero()
	if aVal != bVal {
		tb.varVals[idx].SetOne()
	}
	return idx
}

func (tb *traceBits) and(a, b int) int {
	idx := tb.nextVarIdx
	tb.nextVarIdx++
	aVal := !tb.varVals[a].IsZero()
	bVal := !tb.varVals[b].IsZero()
	if aVal && bVal {
		tb.varVals[idx].SetOne()
	}
	return idx
}

func (tb *traceBits) not(a int) int {
	idx := tb.nextVarIdx
	tb.nextVarIdx++
	aVal := !tb.varVals[a].IsZero()
	if !aVal {
		tb.varVals[idx].SetOne()
	}
	return idx
}

// WitnessFor 为指定的 64 字节消息计算 witness。
// 返回 varVals，其中 varVals[i] = 变量 i 的取值。
func (kc *KeccakCircuit) WitnessFor(msgBytes [64]byte) []fr.Element {
	msgBits := BytesToBits(msgBytes[:])

	tb := newTraceBits(kc.nVars, kc.nVarsBefore)

	// ── 分配输入位变量（顺序与 buildKeccakGates 一致）──
	var inputVarTrace [512]int
	for i := 0; i < 512; i++ {
		v := tb.allocBool(msgBits[i])
		inputVarTrace[i] = v
		// BoolCheck 不分配新变量，仅添加门。
	}

	// ── 分配两个常数 1 填充变量 ──
	constOne1 := tb.allocConstOne()
	constOne2 := tb.allocConstOne()
	_ = constOne1
	_ = constOne2

	// ── 构建初始状态追踪 ──
	var stateTrace [5][5][64]int
	for i := 0; i < 25; i++ {
		x := i % 5
		y := i / 5
		for z := 0; z < 64; z++ {
			bitIdx := i*64 + z
			switch {
			case bitIdx < 512:
				stateTrace[x][y][z] = inputVarTrace[bitIdx]
			case bitIdx == 512:
				stateTrace[x][y][z] = constOne1
			case bitIdx == 1087:
				stateTrace[x][y][z] = constOne2
			default:
				stateTrace[x][y][z] = 0 // 零变量
			}
		}
	}

	// ── 执行 24 轮（追踪模式）──
	for round := 0; round < 24; round++ {
		stateTrace = keccakRoundTrace(tb, stateTrace, round)
	}

	return tb.varVals
}

// keccakRoundTrace 对应 keccakRoundCircuit，但仅计算取值（不添加门）。
func keccakRoundTrace(tb *traceBits, s [5][5][64]int, round int) [5][5][64]int {
	s = thetaTrace(tb, s)
	s = rhoPiTrace(s)
	s = chiTrace(tb, s)
	s = iotaTrace(tb, s, round)
	return s
}

func thetaTrace(tb *traceBits, s [5][5][64]int) [5][5][64]int {
	var C [5][64]int
	for x := 0; x < 5; x++ {
		for z := 0; z < 64; z++ {
			t1 := tb.xor(s[x][0][z], s[x][1][z])
			t2 := tb.xor(t1, s[x][2][z])
			t3 := tb.xor(t2, s[x][3][z])
			C[x][z] = tb.xor(t3, s[x][4][z])
		}
	}
	var D [5][64]int
	for x := 0; x < 5; x++ {
		for z := 0; z < 64; z++ {
			D[x][z] = tb.xor(C[(x+4)%5][z], C[(x+1)%5][(z+63)%64])
		}
	}
	var newS [5][5][64]int
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 64; z++ {
				newS[x][y][z] = tb.xor(s[x][y][z], D[x][z])
			}
		}
	}
	return newS
}

func rhoPiTrace(s [5][5][64]int) [5][5][64]int {
	var newS [5][5][64]int
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			newX := y
			newY := (2*x + 3*y) % 5
			for z := 0; z < 64; z++ {
				zSrc := (z - rhoOffsets[x][y] + 64) % 64
				newS[newX][newY][z] = s[x][y][zSrc]
			}
		}
	}
	return newS
}

func chiTrace(tb *traceBits, s [5][5][64]int) [5][5][64]int {
	old := s
	var newS [5][5][64]int
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 64; z++ {
				notNext := tb.not(old[(x+1)%5][y][z])
				andResult := tb.and(notNext, old[(x+2)%5][y][z])
				newS[x][y][z] = tb.xor(old[x][y][z], andResult)
			}
		}
	}
	return newS
}

func iotaTrace(tb *traceBits, s [5][5][64]int, round int) [5][5][64]int {
	rc := keccakRC[round]
	for z := 0; z < 64; z++ {
		if (rc>>uint(z))&1 == 1 {
			s[0][0][z] = tb.not(s[0][0][z])
		}
	}
	return s
}
