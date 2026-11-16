// Package rng is the PicoRNG generator: a ratcheting AES-256-CTR keystream
// XORed with the platform generator.
package rng

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

var errNotSeeded = errors.New("rng: not seeded")

type Generator struct {
	mu     sync.Mutex
	state  [32]byte
	step   uint64
	seeded bool
	rand   io.Reader
}

// New seeds the state without a key; Read fails until Reseed.
func New() *Generator {
	return &Generator{state: seed(nil), rand: rand.Reader}
}

// Reseed mixes seed(key) into the state: S = HMAC-SHA256(S, seed(key)).
func (g *Generator) Reseed(key []byte) {
	s := seed(key)
	g.mu.Lock()
	m := hmac.New(sha256.New, g.state[:])
	m.Write(s[:])
	m.Sum(g.state[:0])
	g.seeded = true
	g.mu.Unlock()
	clear(s[:])
}

func (g *Generator) Seeded() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.seeded
}

// Read fills p in one ratchet step with a hidden tail:
// A = AES-256-CTR(S_i), R from the platform generator, both 32+len(p) bytes;
// p = A[32:] ⊕ R[32:]; S_{i+1} = HMAC-SHA256(A[:32], R[:32] ‖ i).
func (g *Generator) Read(p []byte) (int, error) {
	var a, r [32]byte
	if _, err := io.ReadFull(g.rand, r[:]); err != nil {
		return 0, err
	}
	g.mu.Lock()
	if !g.seeded {
		g.mu.Unlock()
		return 0, errNotSeeded
	}
	block, _ := aes.NewCipher(g.state[:])
	ctr := cipher.NewCTR(block, make([]byte, aes.BlockSize))
	ctr.XORKeyStream(a[:], a[:])
	m := hmac.New(sha256.New, a[:])
	m.Write(r[:])
	m.Write(binary.BigEndian.AppendUint64(nil, g.step))
	m.Sum(g.state[:0])
	g.step++
	g.mu.Unlock()
	clear(a[:])
	clear(r[:])

	if _, err := io.ReadFull(g.rand, p); err != nil {
		return 0, err
	}
	ctr.XORKeyStream(p, p)
	return len(p), nil
}
