package keccak

import (
	"github.com/consensys/gnark/frontend"
)

// gnarkKeccakOneBlock 是可复用的核心函数：约束 keccak256(input) == output。
// input 须为 512 个 frontend.Variable，output 须为 256 个 frontend.Variable。
func gnarkKeccakOneBlock(api frontend.API, input, output []frontend.Variable) {
	// 断言所有输入位均为布尔值。
	for i := 0; i < 512; i++ {
		api.AssertIsBoolean(input[i])
	}

	// ── 将消息吸收到初始全零状态 ────────────────────────────────────────────
	var state [5][5][64]frontend.Variable
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 64; z++ {
				bitIdx := (x+5*y)*64 + z
				switch {
				case bitIdx < 512:
					state[x][y][z] = input[bitIdx]
				case bitIdx == 512:
					state[x][y][z] = 1
				case bitIdx == 1087:
					state[x][y][z] = 1
				default:
					state[x][y][z] = 0
				}
			}
		}
	}

	// ── 执行 24 轮 Keccak ────────────────────────────────────────────────────
	for round := 0; round < 24; round++ {
		state = gnarkRound(api, state, round)
	}

	// ── 断言输出与声明的哈希值一致 ──────────────────────────────────────────
	for i := 0; i < 256; i++ {
		x := (i / 64) % 5
		y := (i / 64) / 5
		z := i % 64
		api.AssertIsEqual(state[x][y][z], output[i])
	}
}

// gnarkRound 对状态执行一轮 Keccak-f 变换。
func gnarkRound(api frontend.API, state [5][5][64]frontend.Variable, round int) [5][5][64]frontend.Variable {
	// ── θ（theta）──────────────────────────────────────────────────────────
	var C [5][64]frontend.Variable
	for x := 0; x < 5; x++ {
		for z := 0; z < 64; z++ {
			t := api.Xor(state[x][0][z], state[x][1][z])
			t = api.Xor(t, state[x][2][z])
			t = api.Xor(t, state[x][3][z])
			C[x][z] = api.Xor(t, state[x][4][z])
		}
	}
	var D [5][64]frontend.Variable
	for x := 0; x < 5; x++ {
		for z := 0; z < 64; z++ {
			// D[x][z] = C[x-1][z] XOR ROT(C[x+1], 1)[z]
			//         = C[(x+4)%5][z] XOR C[(x+1)%5][(z+63)%64]
			D[x][z] = api.Xor(C[(x+4)%5][z], C[(x+1)%5][(z+63)%64])
		}
	}
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 64; z++ {
				state[x][y][z] = api.Xor(state[x][y][z], D[x][z])
			}
		}
	}

	// ── ρ + π（旋转 + 置换，仅重新索引变量）────────────────────────────────
	var tmp [5][5][64]frontend.Variable
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			newX := y
			newY := (2*x + 3*y) % 5
			for z := 0; z < 64; z++ {
				zSrc := (z - rhoOffsets[x][y] + 64*1000) % 64
				tmp[newX][newY][z] = state[x][y][zSrc]
			}
		}
	}
	state = tmp

	// ── χ（chi）───────────────────────────────────────────────────────────
	before := state
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 64; z++ {
				notNext := api.Sub(1, before[(x+1)%5][y][z])
				andVal := api.And(notNext, before[(x+2)%5][y][z])
				state[x][y][z] = api.Xor(before[x][y][z], andVal)
			}
		}
	}

	// ── ι（iota）：将轮常数异或进 state[0][0] ────────────────────────────
	for z := 0; z < 64; z++ {
		if (keccakRC[round]>>uint(z))&1 == 1 {
			state[0][0][z] = api.Sub(1, state[0][0][z]) // RC 位为 1 时取 NOT
		}
	}

	return state
}

// FourKeccakCircuit 将 4 个独立的 Keccak-256 哈希计算串联为一个大型电路
//（约 4 × 231K ≈ 925K 个约束）。这代表了 Piano 分配到 M=4 个节点的
//"单机 PLONK"工作负载。
type FourKeccakCircuit struct {
	Inputs  [4][512]frontend.Variable
	Outputs [4][256]frontend.Variable `gnark:",public"`
}

func (c *FourKeccakCircuit) Define(api frontend.API) error {
	for k := 0; k < 4; k++ {
		gnarkKeccakOneBlock(api, c.Inputs[k][:], c.Outputs[k][:])
	}
	return nil
}

// FourKeccakWitness 为 4-Keccak 电路构建 witness。
func FourKeccakWitness(msgs [4][64]byte, hashes [4][32]byte) *FourKeccakCircuit {
	w := &FourKeccakCircuit{}
	for k := 0; k < 4; k++ {
		for i := 0; i < 64; i++ {
			for b := 0; b < 8; b++ {
				w.Inputs[k][i*8+b] = int((msgs[k][i] >> uint(b)) & 1)
			}
		}
		for i := 0; i < 32; i++ {
			for b := 0; b < 8; b++ {
				w.Outputs[k][i*8+b] = int((hashes[k][i] >> uint(b)) & 1)
			}
		}
	}
	return w
}

// rhoOffsets 引用自同包的 keccak.go。
