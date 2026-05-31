// Package keccak 提供原生 Keccak-256 实现（以太坊风格，0x01 填充）
// 以及用于 Piano PLONK 协议的 Keccak-256 电路。
package keccak

// ────────────────────────────────────────────────────────────────────────────
// 原生 Keccak-256
// ────────────────────────────────────────────────────────────────────────────

// rhoOffsets[x][y] 给出 lane A[x][y] 的旋转量。
var rhoOffsets = [5][5]int{
	{0, 36, 3, 41, 18},
	{1, 44, 10, 45, 2},
	{62, 6, 43, 15, 61},
	{28, 55, 25, 21, 56},
	{27, 20, 39, 8, 14},
}

// keccakRC 保存 ι 步骤所需的 24 个轮常数。
var keccakRC = [24]uint64{
	0x0000000000000001, 0x0000000000008082, 0x800000000000808A,
	0x8000000080008000, 0x000000000000808B, 0x0000000080000001,
	0x8000000080008081, 0x8000000000008009, 0x000000000000008A,
	0x0000000000000088, 0x0000000080008009, 0x000000008000000A,
	0x000000008000808B, 0x800000000000008B, 0x8000000000008089,
	0x8000000000008003, 0x8000000000008002, 0x8000000000000080,
	0x000000000000800A, 0x800000008000000A, 0x8000000080008081,
	0x8000000000008080, 0x0000000080000001, 0x8000000080008008,
}

// rotl64 将 x 循环左移 n 位（64 位）。
func rotl64(x uint64, n int) uint64 {
	return (x << uint(n)) | (x >> uint(64-n))
}

// keccakF 对 state 原地执行 Keccak-f[1600] 置换。
func keccakF(state *[25]uint64) {
	var C [5]uint64
	var D [5]uint64
	var tmp [25]uint64

	for round := 0; round < 24; round++ {
		// θ 步骤
		for x := 0; x < 5; x++ {
			C[x] = state[x] ^ state[x+5] ^ state[x+10] ^ state[x+15] ^ state[x+20]
		}
		for x := 0; x < 5; x++ {
			D[x] = C[(x+4)%5] ^ rotl64(C[(x+1)%5], 1)
		}
		for i := 0; i < 25; i++ {
			state[i] ^= D[i%5]
		}

		// ρ + π 步骤
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				newX := y
				newY := (2*x + 3*y) % 5
				tmp[newX+5*newY] = rotl64(state[x+5*y], rhoOffsets[x][y])
			}
		}
		copy(state[:], tmp[:])

		// χ 步骤
		for y := 0; y < 5; y++ {
			for x := 0; x < 5; x++ {
				tmp[x+5*y] = state[x+5*y] ^ ((^state[(x+1)%5+5*y]) & state[(x+2)%5+5*y])
			}
		}
		copy(state[:], tmp[:])

		// ι 步骤
		state[0] ^= keccakRC[round]
	}
}

// Hash256 计算 data 的 Keccak-256（以太坊风格，0x01 填充）。
func Hash256(data []byte) [32]byte {
	// 速率 = 136 字节（1088 位），容量 = 64 字节（512 位）
	const rate = 136

	var state [25]uint64

	// 吸收阶段
	padded := make([]byte, len(data))
	copy(padded, data)

	// 处理完整块
	for len(padded) >= rate {
		for i := 0; i < rate/8; i++ {
			state[i] ^= uint64(padded[8*i]) |
				uint64(padded[8*i+1])<<8 |
				uint64(padded[8*i+2])<<16 |
				uint64(padded[8*i+3])<<24 |
				uint64(padded[8*i+4])<<32 |
				uint64(padded[8*i+5])<<40 |
				uint64(padded[8*i+6])<<48 |
				uint64(padded[8*i+7])<<56
		}
		keccakF(&state)
		padded = padded[rate:]
	}

	// 最后一块加填充
	last := make([]byte, rate)
	copy(last, padded)
	last[len(padded)] ^= 0x01       // Keccak 填充（非 SHA3 的 0x06）
	last[rate-1] ^= 0x80

	for i := 0; i < rate/8; i++ {
		state[i] ^= uint64(last[8*i]) |
			uint64(last[8*i+1])<<8 |
			uint64(last[8*i+2])<<16 |
			uint64(last[8*i+3])<<24 |
			uint64(last[8*i+4])<<32 |
			uint64(last[8*i+5])<<40 |
			uint64(last[8*i+6])<<48 |
			uint64(last[8*i+7])<<56
	}
	keccakF(&state)

	// 挤出阶段
	var out [32]byte
	for i := 0; i < 4; i++ {
		w := state[i]
		out[8*i] = byte(w)
		out[8*i+1] = byte(w >> 8)
		out[8*i+2] = byte(w >> 16)
		out[8*i+3] = byte(w >> 24)
		out[8*i+4] = byte(w >> 32)
		out[8*i+5] = byte(w >> 40)
		out[8*i+6] = byte(w >> 48)
		out[8*i+7] = byte(w >> 56)
	}
	return out
}

// Hash256Bits 计算 Keccak-256，并以 256 个布尔位返回（每字节 LSB 优先）。
func Hash256Bits(data []byte) [256]bool {
	h := Hash256(data)
	var bits [256]bool
	for i, b := range h {
		for z := 0; z < 8; z++ {
			bits[i*8+z] = (b>>uint(z))&1 == 1
		}
	}
	return bits
}

// BytesToBits 将字节转换为位（每字节 LSB 优先）。
func BytesToBits(data []byte) []bool {
	bits := make([]bool, len(data)*8)
	for i, b := range data {
		for z := 0; z < 8; z++ {
			bits[i*8+z] = (b>>uint(z))&1 == 1
		}
	}
	return bits
}

// BitsToBytes 将位（每字节 LSB 优先）转换回字节。
func BitsToBytes(bts []bool) []byte {
	out := make([]byte, (len(bts)+7)/8)
	for i, b := range bts {
		if b {
			out[i/8] |= 1 << uint(i%8)
		}
	}
	return out
}

// StateFromWords 从 25 个字中提取 Keccak 状态，表示为 [5][5][64]bool。
// 字索引 i = A[i%5][i/5]，按小端位序（位 z = (word>>z)&1）。
func StateFromWords(words [25]uint64) [5][5][64]bool {
	var s [5][5][64]bool
	for i, w := range words {
		x := i % 5
		y := i / 5
		for z := 0; z < 64; z++ {
			s[x][y][z] = (w>>uint(z))&1 == 1
		}
	}
	return s
}

// ────────────────────────────────────────────────────────────────────────────
// 基于位数组的 Keccak（用于电路追踪计算）
// ────────────────────────────────────────────────────────────────────────────

// keccakBitsState 以 [5][5][64]bool 形式保存 Keccak 状态。
type keccakBitsState [5][5][64]bool

// thetaBits 对位状态执行 θ 步骤。
func thetaBits(s *keccakBitsState) {
	var C [5][64]bool
	var D [5][64]bool

	for x := 0; x < 5; x++ {
		for z := 0; z < 64; z++ {
			C[x][z] = s[x][0][z] != s[x][1][z]
			C[x][z] = C[x][z] != s[x][2][z]
			C[x][z] = C[x][z] != s[x][3][z]
			C[x][z] = C[x][z] != s[x][4][z]
		}
	}

	for x := 0; x < 5; x++ {
		for z := 0; z < 64; z++ {
			D[x][z] = C[(x+4)%5][z] != C[(x+1)%5][(z+63)%64]
		}
	}

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 64; z++ {
				s[x][y][z] = s[x][y][z] != D[x][z]
			}
		}
	}
}

// rhoPiBits 对位状态执行 ρ+π 步骤（仅重新索引）。
func rhoPiBits(s *keccakBitsState) {
	var newS keccakBitsState
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
	*s = newS
}

// chiBits 执行 χ 步骤。
func chiBits(s *keccakBitsState) {
	var old keccakBitsState = *s
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 64; z++ {
				// out = old[x][y][z] XOR ((NOT old[(x+1)%5][y][z]) AND old[(x+2)%5][y][z])
				notNext := !old[(x+1)%5][y][z]
				andResult := notNext && old[(x+2)%5][y][z]
				s[x][y][z] = old[x][y][z] != andResult
			}
		}
	}
}

// iotaBits 对指定轮次执行 ι 步骤。
func iotaBits(s *keccakBitsState, round int) {
	rc := keccakRC[round]
	for z := 0; z < 64; z++ {
		if (rc>>uint(z))&1 == 1 {
			s[0][0][z] = !s[0][0][z]
		}
	}
}

// keccakFBits 对位状态执行 24 轮 Keccak-f。
func keccakFBits(s *keccakBitsState) {
	for round := 0; round < 24; round++ {
		thetaBits(s)
		rhoPiBits(s)
		chiBits(s)
		iotaBits(s, round)
	}
}
