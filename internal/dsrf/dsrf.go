// Package dsrf writes DSRF kits.
package dsrf

import (
	"archive/tar"
	"encoding/binary"
	"hash/crc32"
	"io"
	"time"
)

const (
	BlockSize    = 36
	BlocksPerKey = 2
)

// Kis returns kis_1 for keys: blocks of 32 bytes from r and their CRC32, little-endian.
func Kis(r io.Reader, keys int) ([]byte, error) {
	blocks := keys * BlocksPerKey
	raw := make([]byte, blocks*32)
	defer clear(raw)
	if _, err := io.ReadFull(r, raw); err != nil {
		return nil, err
	}
	out := make([]byte, blocks*BlockSize)
	for i := range blocks {
		b := out[i*BlockSize:]
		copy(b, raw[i*32:(i+1)*32])
		binary.LittleEndian.PutUint32(b[32:], crc32.ChecksumIEEE(b[:32]))
	}
	return out, nil
}

// WriteTar writes kpim with the kit number id and kis as db1/kis_1 and db2/kis_1.
func WriteTar(w io.Writer, id string, kis []byte) error {
	tw := tar.NewWriter(w)
	now := time.Now()
	for _, f := range []struct {
		name string
		data []byte
	}{{"kpim", []byte(id)}, {"db1/kis_1", kis}, {"db2/kis_1", kis}} {
		if err := tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o644, Size: int64(len(f.data)), ModTime: now}); err != nil {
			return err
		}
		if _, err := tw.Write(f.data); err != nil {
			return err
		}
	}
	return tw.Close()
}
