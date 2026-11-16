// Command picorng serves pseudorandom bytes and DSRF kits.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/PiDmitrius/PicoRNG/internal/rng"
	"github.com/PiDmitrius/PicoRNG/internal/server"
)

var version = "dev"

const (
	control = "/run/picorng/control.sock"
	socket  = "/run/picorng/picorng.sock"
)

const usage = `picorng — pseudorandom bytes and DSRF kits

  picorng                    Run the service on its sockets; it answers 503
                             until the first reseed
  picorng listen addr:port   Run the service, also over HTTP on addr:port
  picorng reseed             Mix stdin into the state of the service by the
                             seeding procedure keyed by it; root only
  picorng bytes N [hex|base64]
                             Print N random bytes from the service
  picorng dsrf N             Print a DSRF kit for N keys as tar
`

func main() {
	log.SetFlags(0)
	args := os.Args[1:]
	switch {
	case len(args) == 0:
		serve("")
	case len(args) == 2 && args[0] == "listen":
		serve(args[1])
	case len(args) == 1 && args[0] == "reseed":
		call(control, http.MethodPost, "/reseed", os.Stdin)
	case (len(args) == 2 || len(args) == 3) && args[0] == "bytes":
		q := url.Values{"n": {args[1]}}
		if len(args) == 3 {
			q.Set("format", args[2])
		}
		call(socket, http.MethodGet, "/bytes?"+q.Encode(), nil)
	case len(args) == 2 && args[0] == "dsrf":
		call(socket, http.MethodGet, "/dsrf?"+url.Values{"keys": {args[1]}}.Encode(), nil)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func serve(listen string) {
	g := rng.New()
	pub := server.Public(g)
	errc := make(chan error)
	run := func(l net.Listener, h http.Handler) {
		go func() { errc <- http.Serve(l, h) }()
	}
	run(listenUnix(control, 0o600), server.Control(g))
	run(listenUnix(socket, 0o666), pub)
	if listen != "" {
		l, err := net.Listen("tcp", listen)
		if err != nil {
			log.Fatal(err)
		}
		run(l, pub)
	}
	log.Printf("picorng %s: waiting for reseed", version)
	log.Fatal(<-errc)
}

func listenUnix(path string, mode os.FileMode) net.Listener {
	l, err := net.Listen("unix", path)
	if err == nil {
		err = os.Chmod(path, mode)
	}
	if err != nil {
		log.Fatal(err)
	}
	return l
}

// call sends a request to the service over a socket and copies a 200 answer
// to stdout.
func call(path, method, target string, body io.Reader) {
	c := http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", path)
		},
	}}
	req, err := http.NewRequest(method, "http://picorng"+target, body)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		log.Fatalf("%s: %s", resp.Status, msg)
	}
	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		log.Fatal(err)
	}
}
