package rng

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"time"
)

// seed mixes a second of timing jitter, platform bytes and the wall clock in
// cross-fed SHA-256 pools and returns HMAC-SHA256(key, pool digests).
func seed(key []byte) [32]byte {
	pools := [3]hash.Hash{sha256.New(), sha256.New(), sha256.New()}
	const jitter, platform, clock = 0, 1, 2
	walk := make([]byte, 1<<16)
	var x byte
	buf := make([]byte, 0, 64)
	digests := make([][]byte, len(pools))
	start := time.Now()
	for round := uint64(0); time.Since(start) < time.Second; round++ {
		for range 1024 {
			t := time.Now()
			for j := 0; j < len(walk); j += 64 {
				walk[j] ^= x + byte(j)
				x = walk[(j*7+int(x))&(len(walk)-1)]
			}
			buf = binary.LittleEndian.AppendUint64(buf[:0], uint64(time.Since(t)))
			pools[jitter].Write(append(buf, x))
		}
		rand.Read(buf[:32])
		pools[platform].Write(buf[:32])
		pools[clock].Write(binary.LittleEndian.AppendUint64(buf[:0], uint64(time.Now().UnixNano())))
		pools[clock].Write(binary.LittleEndian.AppendUint64(buf[:0], round))

		for i, p := range pools {
			digests[i] = p.Sum(digests[i][:0])
		}
		for i, p := range pools {
			for k := 1; k < len(pools); k++ {
				p.Write(digests[(i+k)%len(pools)])
			}
		}
	}
	m := hmac.New(sha256.New, key)
	for i, p := range pools {
		m.Write(p.Sum(digests[i][:0]))
	}
	var out [32]byte
	m.Sum(out[:0])
	return out
}
