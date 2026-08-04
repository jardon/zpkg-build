# zpkg-build

A CLI tool that compiles and packages software inside isolated sandbox environments using a declarative YAML manifest.

## Dependencies

For Debian-based distributions, install these build dependencies:
```bash
sudo apt install -y golang-go libbtrfs-dev libgpgme-dev lxc-dev
```

For Fedora-based distributions, install these build dependencies:
```bash
sudo dnf install -y golang libbtrfs gpgme2 lxc
```

For Arch-based distributions, install these build dependencies:
```bash
sudo pacman -Sy go btrfs-progs gpgme lxc
```

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
zpkg-build -f package.yaml analyze   # check manifest reproducibility
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
  env:
    CGO_ENABLED: "0"
    GOFLAGS: "-tags=netgo,static"
    LDFLAGS: "-s -w"

# Metadata-only — documents a runtime requirement for the base image
build_deps:
  - name: "gcc"
    min: "12.0"

# Prebuilt — downloaded, verified, extracted into the sandbox
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

### Build environment

The following variables are available inside the sandbox during all build steps:

| Variable | Description |
|----------|-------------|
| `ZPKG_WORKSPACE` | Workspace root (`/zpkg-build-workspace`) |
| `ZPKG_COMPONENTS` | Component root (`/zpkg-build-workspace/components/<name>`) |
| `ZPKG_SRC` | Source directory |
| `ZPKG_BUILD` | Build directory (also the working directory) |
| `ZPKG_DEST` | Package destination (install target) |
| `ZPKG_PKG` | Package staging directory |
| `ZPKG_EXPORT` | Export directory |
| `ZPKG_NAME` | Package name from manifest |
| `ZPKG_VERSION` | Package version from manifest |
| `ZPKG_ARCH` | Package architecture from manifest |

Plugin-specific variables (e.g. `GOPATH`, `CARGO_HOME`, `JAVA_HOME`) are set by each plugin's `GetEnvVars()`.

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
