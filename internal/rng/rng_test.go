package rng

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sync"
	"testing"
)

// counting is a predictable stand-in for the platform generator.
type counting struct{ n byte }

func (c *counting) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = c.n
		c.n++
	}
	return len(p), nil
}

func TestNotSeeded(t *testing.T) {
	g := New()
	if _, err := g.Read(make([]byte, 1)); !errors.Is(err, errNotSeeded) {
		t.Fatalf("Read before Reseed: %v", err)
	}
	g.Reseed([]byte("key"))
	if _, err := g.Read(make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
}

func TestStep(t *testing.T) {
	s := sha256.Sum256([]byte("state"))
	g := &Generator{state: s, seeded: true, rand: &counting{}}
	out := make([]byte, 100)
	if _, err := g.Read(out); err != nil {
		t.Fatal(err)
	}

	r := make([]byte, 132)
	(&counting{}).Read(r)
	a := make([]byte, 132)
	block, _ := aes.NewCipher(s[:])
	cipher.NewCTR(block, make([]byte, aes.BlockSize)).XORKeyStream(a, a)
	want := make([]byte, 100)
	for i := range want {
		want[i] = a[32+i] ^ r[32+i]
	}
	if !bytes.Equal(out, want) {
		t.Fatal("output is not A[32:] xor R[32:]")
	}
	m := hmac.New(sha256.New, a[:32])
	m.Write(r[:32])
	m.Write(binary.BigEndian.AppendUint64(nil, 0))
	if !bytes.Equal(g.state[:], m.Sum(nil)) || g.step != 1 {
		t.Fatal("next state is not HMAC(A[:32], R[:32] || 0)")
	}
}

func TestConcurrent(t *testing.T) {
	g := New()
	g.Reseed([]byte("key"))
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := g.Read(make([]byte, 4096)); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}
