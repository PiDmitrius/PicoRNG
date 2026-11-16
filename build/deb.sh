#!/bin/sh
# deb.sh: static binary and dist/picorng_<VERSION>_amd64.deb from HEAD in the image of build/go.image
set -eu
here=$(cd "$(dirname "$0")/.." && pwd)
version=$(git -C "$here" show HEAD:VERSION)
epoch=$(git -C "$here" log -1 --format=%ct)
src=$(mktemp -d)
trap 'rm -rf "$src"' EXIT
git -C "$here" archive HEAD | tar -x -C "$src"
mkdir -p "$here/dist"
docker run --rm --user "$(id -u):$(id -g)" -e HOME=/tmp -e SOURCE_DATE_EPOCH="$epoch" \
    -e VERSION="$version" -v "$src:/src:ro" -v "$here/dist:/dist" -w /src "$(cat "$src/build/go.image")" sh -euc '
root=/tmp/pkg
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -ldflags "-s -w -X main.version=$VERSION" -o /dist/picorng ./cmd/picorng
install -D -m 755 /dist/picorng $root/usr/bin/picorng
install -D -m 644 deploy/picorng.service $root/usr/lib/systemd/system/picorng.service
install -d $root/DEBIAN
sed "s/@VERSION@/$VERSION/" deploy/debian/control > $root/DEBIAN/control
install -m 755 deploy/debian/postinst deploy/debian/prerm deploy/debian/postrm $root/DEBIAN/
find $root -exec touch -h -d @$SOURCE_DATE_EPOCH {} +
dpkg-deb --root-owner-group -Zxz --build $root /dist/picorng_${VERSION}_amd64.deb >/dev/null
'
sha256sum "$here/dist/picorng" "$here/dist/picorng_${version}_amd64.deb"
