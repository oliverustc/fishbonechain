package keccak

import (
	"encoding/hex"
	"testing"
)

func TestKeccak256Empty(t *testing.T) {
	got := Hash256([]byte{})
	want := "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
	if hex.EncodeToString(got[:]) != want {
		t.Errorf("Keccak256([]) = %x，期望 %s", got, want)
	}
}

func TestKeccak256Abc(t *testing.T) {
	got := Hash256([]byte("abc"))
	want := "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45"
	if hex.EncodeToString(got[:]) != want {
		t.Errorf("Keccak256(abc) = %x，期望 %s", got, want)
	}
}
