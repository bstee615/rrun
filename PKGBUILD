# Maintainer: benjis
pkgname=rrun
pkgver=0.1.0
pkgrel=1
pkgdesc="Sync git-tracked files to a remote machine and run commands there"
arch=('x86_64' 'aarch64')
url="https://github.com/benjis/rrun"
license=('MIT')
depends=('openssh' 'rsync' 'git')
makedepends=('go')
source=("$pkgname::git+ssh://git@github.com/benjis/rrun.git#tag=v$pkgver")
sha256sums=('SKIP')

prepare() {
    cd "$pkgname"
    go mod download
}

build() {
    cd "$pkgname"
    go build \
        -trimpath \
        -buildvcs=false \
        -ldflags "-s -w -X rrun/cmd.version=$pkgver" \
        -o "$pkgname" .
}

check() {
    cd "$pkgname"
    go test ./...
}

package() {
    cd "$pkgname"
    install -Dm755 "$pkgname"   "$pkgdir/usr/bin/$pkgname"
    install -Dm644 README.md    "$pkgdir/usr/share/doc/$pkgname/README.md"
    install -Dm644 CHANGELOG.md "$pkgdir/usr/share/doc/$pkgname/CHANGELOG.md"
    install -Dm644 LICENSE      "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
}
