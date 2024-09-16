package ws

import (
	"crypto/rand"
	"io"
)

// maskRand is an io.Reader for generating mask bytes. The reader is initialized
// to crypto/rand Reader. Tests swap the reader to a math/rand reader for
// reproducible results.
var maskRand = rand.Reader

// newMaskKey returns a new 32 bit value for masking client frames.
func newMaskKey() [4]byte {
	var k [4]byte
	_, _ = io.ReadFull(maskRand, k[:])
	return k
}

func maskBytes(key [4]byte, pos int, b []byte) int {
	for i := range b {
		b[i] ^= key[pos&3]
		pos++
	}
	return pos & 3
}
