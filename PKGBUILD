# Maintainer: fm39hz <fm39hz@gmail.com>
#
# Local Arch package (not AUR yet).
#   make pkg          # or: makepkg -f --cleanbuild --skipinteg
#
# When publishing AUR: replace source with tagged tarball + sha256sums.
# Also ship the manpage in the tarball.

pkgname=gotomux
pkgver=r38.abe0e64 # rewritten by pkgver()
pkgrel=1
pkgdesc='Fuzzy tmux session picker with presets, zoxide and sticky templates'
arch=('x86_64' 'aarch64')
url='https://github.com/fm39hz/gotomux'
license=('MIT')
depends=('tmux')
optdepends=('zoxide: frequent project paths in the picker')
makedepends=('go' 'git')
options=('!lto' '!debug')

source=()
sha256sums=()

pkgver() {
  local desc
  desc=$(git describe --tags --long --match 'v*' 2>/dev/null || true)
  if [[ -n $desc ]]; then
    echo "$desc" | sed -E 's/^v//; s/-([0-9]+)-g/.r\1.g/; s/-/./g'
  else
    printf 'r%s.%s' "$(git rev-list --count HEAD)" "$(git rev-parse --short HEAD)"
  fi
}

prepare() {
  mkdir -p "${srcdir}/${pkgname}"
  git -C "${startdir}" archive --format=tar HEAD | tar -x -C "${srcdir}/${pkgname}"
  git -C "${startdir}" diff HEAD -- . ':!gotomux' ':!dist' ':!src' ':!pkg' |
    patch -d "${srcdir}/${pkgname}" -p1 --forward --batch >/dev/null 2>&1 || true
}

build() {
  cd "${srcdir}/${pkgname}"
  export CGO_ENABLED=0
  export GOFLAGS='-buildmode=pie -trimpath -mod=readonly -modcacherw'
  go build -ldflags="-s -w -X main.version=${pkgver}" -o "${pkgname}" .
}

check() {
  cd "${srcdir}/${pkgname}"
  go test ./internal/project/ ./internal/store/ ./internal/template/ \
    ./internal/picker/ ./internal/tmux/ -count=1 -short
}

package() {
  cd "${srcdir}/${pkgname}"
  install -Dm755 "${pkgname}" "${pkgdir}/usr/bin/${pkgname}"
  install -Dm644 LICENSE "${pkgdir}/usr/share/licenses/${pkgname}/LICENSE"
  install -Dm644 README.md "${pkgdir}/usr/share/doc/${pkgname}/README.md"
  install -Dm644 man/gotomux.1 "${pkgdir}/usr/share/man/man1/gotomux.1"
}
