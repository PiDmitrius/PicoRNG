#!/bin/sh
# test.sh: gofmt, go vet and go test -race of PicoRNG
set -eu
here=$(cd "$(dirname "$0")" && pwd)
docker run --rm -v "$here/..:/src:ro" -v picorng-gocache:/root/.cache/go-build -w /src "$(cat "$here/go.image")" \
    sh -c '! gofmt -l . | grep . && go vet ./... && go test -race -count=1 ./...'
