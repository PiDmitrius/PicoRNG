package server

import (
	"archive/tar"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PiDmitrius/PicoRNG/internal/rng"
)

func get(t *testing.T, h http.Handler, url string) *http.Response {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	return w.Result()
}

func TestServer(t *testing.T) {
	g := rng.New()
	pub, ctl := Public(g), Control(g)
	for _, u := range []string{"/status", "/bytes?n=1", "/dsrf?keys=1"} {
		if c := get(t, pub, u).StatusCode; c != 503 {
			t.Fatalf("%s before reseed: %d", u, c)
		}
	}

	w := httptest.NewRecorder()
	ctl.ServeHTTP(w, httptest.NewRequest("POST", "/reseed", strings.NewReader("")))
	if w.Code != 400 {
		t.Fatalf("empty key: %d", w.Code)
	}
	w = httptest.NewRecorder()
	ctl.ServeHTTP(w, httptest.NewRequest("POST", "/reseed", strings.NewReader("key")))
	if w.Code != 200 {
		t.Fatalf("reseed: %d", w.Code)
	}

	body := func(u string) []byte {
		r := get(t, pub, u)
		if r.StatusCode != 200 {
			t.Fatalf("%s: %d", u, r.StatusCode)
		}
		if r.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: Cache-Control %q", u, r.Header.Get("Cache-Control"))
		}
		b, _ := io.ReadAll(r.Body)
		return b
	}
	if b := body("/bytes?n=131072"); len(b) != 131072 {
		t.Fatalf("bin: %d bytes", len(b))
	}
	if b, err := hex.DecodeString(string(body("/bytes?n=33&format=hex"))); err != nil || len(b) != 33 {
		t.Fatal("hex", len(b), err)
	}
	if b, err := base64.StdEncoding.DecodeString(string(body("/bytes?n=34&format=base64"))); err != nil || len(b) != 34 {
		t.Fatal("base64", len(b), err)
	}

	tr := tar.NewReader(strings.NewReader(string(body("/dsrf?keys=3"))))
	sizes := map[string]int64{}
	for h, err := tr.Next(); err == nil; h, err = tr.Next() {
		sizes[h.Name] = h.Size
	}
	if sizes["kpim"] != 8 || sizes["db1/kis_1"] != 216 || sizes["db2/kis_1"] != 216 {
		t.Fatalf("dsrf sizes %v", sizes)
	}

	for _, u := range []string{"/bytes", "/bytes?n=0", "/bytes?n=131073", "/bytes?n=1&format=x", "/dsrf?keys=1001"} {
		if c := get(t, pub, u).StatusCode; c != 400 {
			t.Fatalf("%s: %d", u, c)
		}
	}
}
