package dsrf

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"hash/crc32"
	"io"
	"testing"
)

func TestKis(t *testing.T) {
	kis, err := Kis(rand.Reader, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(kis) != 72000 {
		t.Fatalf("len = %d, want 72000", len(kis))
	}
	for b := kis; len(b) > 0; b = b[BlockSize:] {
		if binary.LittleEndian.Uint32(b[32:]) != crc32.ChecksumIEEE(b[:32]) {
			t.Fatal("bad CRC32")
		}
	}
}

func TestWriteTar(t *testing.T) {
	kis, _ := Kis(rand.Reader, 2)
	var buf bytes.Buffer
	if err := WriteTar(&buf, "2a3dab43", kis); err != nil {
		t.Fatal(err)
	}
	want := map[string][]byte{"kpim": []byte("2a3dab43"), "db1/kis_1": kis, "db2/kis_1": kis}
	tr := tar.NewReader(&buf)
	for h, err := tr.Next(); err != io.EOF; h, err = tr.Next() {
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(tr)
		if !bytes.Equal(data, want[h.Name]) {
			t.Fatalf("%s differs", h.Name)
		}
		delete(want, h.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing %v", want)
	}
}
