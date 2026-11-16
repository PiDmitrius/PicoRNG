#!/bin/sh
# deb-test.sh: install dist/picorng_<VERSION>_amd64.deb into Debian under systemd, reseed, fetch over the socket and HTTP, remove
set -eu
here=$(cd "$(dirname "$0")/.." && pwd)
deb=picorng_$(cat "$here/VERSION")_amd64.deb
printf 'FROM debian:trixie-slim\nRUN apt-get update && apt-get install -y --no-install-recommends systemd curl && rm -rf /var/lib/apt/lists/*\nCMD ["/lib/systemd/systemd"]\n' |
    docker build -q -t picorng-systemd - >/dev/null
c=$(docker run -d --privileged --cgroupns=host -v /sys/fs/cgroup:/sys/fs/cgroup:rw -v "$here/dist:/dist:ro" picorng-systemd)
trap 'docker rm -f "$c" >/dev/null' EXIT
docker exec "$c" sh -euc '
nobody() { setpriv --reuid=65534 --regid=65534 --clear-groups "$@"; }
until systemctl is-system-running 2>/dev/null | grep -qE "running|degraded"; do sleep 0.2; done
dpkg -i /dist/'"$deb"' >/dev/null
systemctl is-active picorng
until [ -S /run/picorng/picorng.sock ]; do sleep 0.2; done
test "$(stat -c %a /run/picorng /run/picorng/control.sock /run/picorng/picorng.sock | tr "\n" " ")" = "755 600 666 "
nobody picorng bytes 1 && exit 1
head -c 32 /dev/urandom | picorng reseed
head -c 32 /dev/urandom | nobody picorng reseed && exit 1
test "$(nobody picorng bytes 131072 | wc -c)" = 131072
test "$(nobody picorng bytes 16 hex | wc -c)" = 32
nobody picorng dsrf 1000 | tar -t -v
curl -s http://127.0.0.1:8180/status && exit 1
echo "PICORNG_ARGS=\"listen 127.0.0.1:8180\"" > /etc/default/picorng
systemctl restart picorng
until curl -s -o /dev/null http://127.0.0.1:8180/status; do sleep 0.2; done
test "$(curl -s -o /dev/null -w %{http_code} "http://127.0.0.1:8180/bytes?n=1")" = 503
head -c 32 /dev/urandom | picorng reseed
test "$(curl -fsS "http://127.0.0.1:8180/bytes?n=131072" | wc -c)" = 131072
journalctl -u picorng -o cat --no-pager
dpkg -r picorng >/dev/null
! systemctl is-active picorng
'
echo DEB-TEST PASS
