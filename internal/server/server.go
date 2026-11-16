// Package server serves the generator over HTTP and takes the reseed key on
// a separate control handler.
package server

import (
	"encoding/base64"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/PiDmitrius/PicoRNG/internal/dsrf"
	"github.com/PiDmitrius/PicoRNG/internal/rng"
)

const (
	maxBytes = 128 << 10
	maxKeys  = 1000
	maxKey   = 65536
)

// Public serves random output; every route answers 503 until the first reseed.
func Public(g *rng.Generator) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", seeded(g, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "seeded\n")
	}))
	mux.HandleFunc("GET /bytes", seeded(g, func(w http.ResponseWriter, r *http.Request) {
		n, ok := intParam(w, r, "n", maxBytes)
		if !ok {
			return
		}
		format := r.URL.Query().Get("format")
		if format != "" && format != "bin" && format != "hex" && format != "base64" {
			http.Error(w, "format must be bin, hex or base64", http.StatusBadRequest)
			return
		}
		buf := make([]byte, n)
		defer clear(buf)
		if _, err := g.Read(buf); err != nil {
			internalError(w, err)
			return
		}
		out, ct := buf, "application/octet-stream"
		switch format {
		case "hex":
			out, ct = hex.AppendEncode(nil, buf), "text/plain"
		case "base64":
			out, ct = base64.StdEncoding.AppendEncode(nil, buf), "text/plain"
		}
		defer clear(out)
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Length", strconv.Itoa(len(out)))
		w.Write(out)
	}))
	mux.HandleFunc("GET /dsrf", seeded(g, func(w http.ResponseWriter, r *http.Request) {
		keys, ok := intParam(w, r, "keys", maxKeys)
		if !ok {
			return
		}
		id := make([]byte, 4)
		kis, err := dsrf.Kis(g, keys)
		if err == nil {
			_, err = g.Read(id)
		}
		if err != nil {
			internalError(w, err)
			return
		}
		defer clear(kis)
		w.Header().Set("Content-Type", "application/x-tar")
		dsrf.WriteTar(w, hex.EncodeToString(id), kis)
	}))
	return mux
}

// Control takes the reseed key as the body of POST /reseed and answers after
// the seeding procedure has mixed it into the state.
func Control(g *rng.Generator) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /reseed", func(w http.ResponseWriter, r *http.Request) {
		key, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxKey))
		defer clear(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		if len(key) == 0 {
			http.Error(w, "empty key", http.StatusBadRequest)
			return
		}
		g.Reseed(key)
		log.Print("reseeded")
		io.WriteString(w, "seeded\n")
	})
	return mux
}

func seeded(g *rng.Generator, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !g.Seeded() {
			http.Error(w, "not seeded", http.StatusServiceUnavailable)
			return
		}
		h(w, r)
	}
}

func intParam(w http.ResponseWriter, r *http.Request, name string, max int) (int, bool) {
	n, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || n < 1 || n > max {
		http.Error(w, name+" must be 1.."+strconv.Itoa(max), http.StatusBadRequest)
		return 0, false
	}
	return n, true
}

func internalError(w http.ResponseWriter, err error) {
	log.Print(err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
