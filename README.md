# zpkg-build

A CLI tool that compiles and packages software inside isolated sandbox environments using a declarative YAML manifest.

## Build

```bash
go build -o zpkg-build ./cmd/zpkg-build/
```

## Usage

```bash
zpkg-build -f package.yaml pull      # fetch sources, verify patches
zpkg-build -f package.yaml build     # compile inside sandbox
zpkg-build -f package.yaml package   # assemble package root + metadata
zpkg-build -f package.yaml export    # archive output to host
zpkg-build -f package.yaml clean build  # purge state from a stage forward
zpkg-build -f package.yaml status    # show cache and step state
```

## Manifest

```yaml
name: "hello-world"
version: "1.0.0"
arch: "amd64"
license: "MIT"

source:
  git: "https://github.com/example/repo"
  ref: "abc123"
  patches:
    - url: "https://example.com/fix.patch"
      sha256: "..."

engine: "podman"          # podman | docker | lxc | chroot
base: "alpine:3.23"

plugin:
  name: "golang"
  version: "1.20"
  source: "https://go.dev/dl/go1.20.linux-amd64.tar.gz"
  sha256: "..."

build:
  override-args:
    build: "-o bin/app main.go"
    install: "mkdir -p $ZPKG_DEST/usr/bin && cp bin/app $ZPKG_DEST/usr/bin/"

build_deps:
  - name: "gcc"
    min: "12.0"
  - name: "zlib"
    version: "1.3.1"
    source: "https://artifact-server/zlib-1.3.1-linux-amd64.tar.gz"
    sha256: "abc123..."
    extract-to: "/usr"

runtime_deps:
  - name: "libc"
    min: "2.31"

package:
  include:
    - "/usr/bin/app"
```

### Engines

| Engine | Backend | Notes |
|--------|---------|-------|
| `podman` | podman v6 (Go bindings) | Default, rootless |
| `docker` | docker v28 (Go bindings) | Requires daemon |
| `lxc` | go-lxc (CGO) | Requires lxc-dev |
| `chroot` | `unshare` namespaces | No container runtime needed |

### Plugins

| Plugin | Language/Tool |
|--------|--------------|
| `golang` | Go |
| `rust` | Rust (rustup) |
| `cmake` | CMake |
| `make` | Make |
| `autotools` | Autotools (configure/make) |
| `meson` | Meson |
| `maven` | Maven (Java) |
| `poetry` | Poetry (Python) |
| `none` | No build tool |

## Dependencies

```bash
sudo apt install -y libbtrfs-dev libgpgme-dev lxc-dev
```
