# Maintainer: Your Name <your.email@example.com>
pkgname='tock'
_pkgname='tock'
pkgver='0.1.0' # TODO: Update with your package version (e.g., from a git tag)
pkgrel=1
pkgdesc="A CLI tool to keep track of your schedule from a TOML or CSV config."
arch=('x86_64') # You can add 'aarch64' if you build for ARM as well
url="https://github.com/your-username/tock" # TODO: Update with your actual project URL
license=('MIT') # TODO: Update with your actual license

# Dependencies required at runtime (if any, usually none for a Go binary)
depends=()

# Dependencies required to build the package
makedepends=('go')

# Source files for the package. Use 'git+' for building directly from a git repo.
# For tagged releases, you might use 'git+https://...#tag=v${pkgver}'
# or a direct tarball download 'https://github.com/your-username/tock/archive/v${pkgver}.tar.gz'
source=("${_pkgname}::git+${url}.git" # This builds from the latest master/main branch
        'sample.csv') # Include the sample.csv file

# Checksums for source files. For git sources, use 'SKIP'.
# For other files, generate with 'updpkgsums' or 'makepkg -g'
sha256sums=('SKIP'
            'SKIP')

# Build function: compiles the Go application
build() {
  cd "${srcdir}/${_pkgname}"
  # Ensure the build directory exists
  mkdir -p build
  # Build the main executable
  go build -o build/"${_pkgname}" ./cmd/tock
}

# Check function: runs tests (optional but recommended)
check() {
  cd "${srcdir}/${_pkgname}"
  go test ./...
}

# Package function: installs compiled files into the package directory
package() {
  # Install the binary to /usr/bin
  install -Dm755 "${srcdir}/${_pkgname}/build/${_pkgname}" "${pkgdir}/usr/bin/${_pkgname}"

  # Install the sample config file to /usr/share/doc/tock/
  # This makes it available for users to copy to their XDG config directory.
  install -Dm644 "${srcdir}/sample.csv" "${pkgdir}/usr/share/doc/${_pkgname}/sample.csv"
}
