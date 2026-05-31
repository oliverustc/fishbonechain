package utils

import (
	crand "crypto/rand"
	"gnarkabc/logger"
	"math/big"
	"math/rand"
	"time"
	"unsafe"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var src = rand.NewSource(time.Now().UnixNano())

const (
	// 6 bits to represent a letter index
	letterIdBits = 6
	// All 1-bits as many as letterIdBits
	letterIdMask = 1<<letterIdBits - 1
	letterIdMax  = 63 / letterIdBits
)

func RandStr(n int) string {
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdMax letters!
	for i, cache, remain := n-1, src.Int63(), letterIdMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdMax
		}
		if idx := int(cache & letterIdMask); idx < len(letters) {
			b[i] = letters[idx]
			i--
		}
		cache >>= letterIdBits
		remain--
	}
	return *(*string)(unsafe.Pointer(&b))
}

// 使用crypto/rand生成随机整数
func RandInt(min, max int) int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Intn(max-min) + min
}

func RandItem(items []string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return items[r.Intn(len(items))]
}

func IndexOf(items []string, item string) int {
	for i, v := range items {
		if v == item {
			return i
		}
	}
	return -1
}

func RandBigInt(mod *big.Int) []byte {
	v, err := crand.Int(crand.Reader, mod)
	if err != nil {
		logger.Error("rand.Int failed, %v", err)
	}
	vByte := v.Bytes()
	// logger.Debug("mod bit length: %d", mod.BitLen())
	var expectedByteLen int
	modByteLen := mod.BitLen() / 8
	if modByteLen%2 != 0 {
		expectedByteLen = modByteLen + 1
	} else {
		expectedByteLen = modByteLen
	}
	// logger.Debug("expectedByteLen: %d", expectedByteLen)

	if len(vByte) != expectedByteLen {
		vByte = append(make([]byte, expectedByteLen-len(vByte)), vByte...)
	}
	return vByte
}
