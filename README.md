# PicoRNG

A service of pseudorandom bytes, with DSRF kits as one of the output formats,
over local sockets and optionally HTTP.

## Generator

Seeding gathers candidates for one second into separate SHA-256 pools: timing
jitter of a memory walk, the platform generator and the wall clock. After every
round each pool absorbs the digests of the others; the seed is HMAC-SHA256
keyed by the operator key over the final digests.

The service seeds its state without a key at start and serves nothing until
the first reseed. A reseed seeds with the given key and mixes the result into
the current state: `S = HMAC-SHA256(S, seed)`.

Each read of the generator is one ratchet step. With `A` the
AES-256-CTR keystream under the state `S_i` and `R` bytes of the platform
generator, both 32 bytes longer than the output:

```
output  = A[32:] ⊕ R[32:]
S_{i+1} = HMAC-SHA256(A[:32], R[:32] ‖ i)
```

The first 32 bytes of both are never output, so the next state depends on
platform bytes nobody saw: knowing the key and the seed does not give the
state after the first step, and a known state gives neither earlier output nor
later output while the platform generator holds. The output stays
unpredictable while either the state or the platform generator is.

## Requests

The service answers the same requests on `picorng.sock` and over HTTP, each
with `Cache-Control: no-store`; `POST /reseed` goes to `control.sock` only.

| Request | Response |
|---|---|
| `GET /status` | `seeded` |
| `GET /bytes?n=N[&format=bin\|hex\|base64]` | `N` random bytes, 1 ≤ `N` ≤ 131072, as is, in hex or in base64 |
| `GET /dsrf?keys=N` | DSRF kit for `N` keys as tar, 1 ≤ `N` ≤ 1000 |

| Status | When |
|---|---|
| `400` | `n`, `keys` or `format` out of range, empty reseed key |
| `404` | another path |
| `405` | another method |
| `413` | reseed key over 65536 bytes |
| `503` | before the first reseed |

HTTP is plain; TLS and access control belong to a reverse proxy in front of
it.

## Running

```
picorng                       # service on its sockets
picorng listen 127.0.0.1:8180 # service, also over HTTP
picorng reseed < key          # root only
picorng bytes N [hex|base64]
picorng dsrf N > kit.tar
```

The service listens on `/run/picorng/control.sock`, `0600`, for `reseed`
and on `/run/picorng/picorng.sock`, `0666`, for everything else; the commands
other than `listen` are clients of a running service. `reseed` sends stdin,
1 to 65536 bytes taken as is, as the key. With the key kept by
[picoseal](https://github.com/PiDmitrius/picoseal):

```sh
picoseal open picorng | picorng reseed
```

The Debian package installs `/usr/bin/picorng` and `picorng.service`, which
runs under a dynamic user with the arguments from `PICORNG_ARGS` in
`/etc/default/picorng`, for HTTP `PICORNG_ARGS="listen 127.0.0.1:8180"`. A
restart, including a package upgrade, needs a reseed again.

## Build

```sh
build/test.sh      # gofmt, go vet, go test -race in Docker
build/deb.sh       # dist/picorng and dist/picorng_<VERSION>_amd64.deb in Docker
build/deb-test.sh  # install the package under systemd, reseed, fetch, remove
```

GitHub Actions runs them on every push and pull request; a tag `vX.Y.Z`
matching `VERSION` publishes the release with the binary, the package and
`SHA256SUMS`.
