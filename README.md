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
zpkg-build -f package.yaml pull          # fetch sources, verify patches
zpkg-build -f package.yaml build         # compile inside sandbox
zpkg-build -f package.yaml build --keep  # compile and keep the sandbox alive for debugging
zpkg-build -f package.yaml package       # assemble package root + metadata
zpkg-build -f package.yaml export        # archive output to host
zpkg-build -f package.yaml clean build   # purge state from a stage forward
zpkg-build -f package.yaml destroy       # destroy a sandbox kept alive by --keep
zpkg-build -f package.yaml status        # show cache and step state
zpkg-build -f package.yaml analyze       # check manifest reproducibility
zpkg-build -f package.yaml hash          # print canonical SHA-256 of manifest
```

With `--keep`, the build environment (container/rootfs) is left running after the build stage so it can be inspected. The engine identifier is written to `.zpkg-build-state/kept-engine.json` in the workspace, and `zpkg-build destroy` removes the kept environment (podman/docker/lxc/chroot) and clears that state file.

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
  go-build-args:
    - "-trimpath"
    - "-ldflags=-s -w"
    - "-o"
    - "$ZPKG_DEST/usr/bin/hello"
    - "main.go"

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

### Plugin build arguments

Each plugin accepts a plugin-specific list of arguments that override the default arguments of its primary build command. The command prefix is fixed and cannot be changed.

| Plugin | Key | Command |
|--------|-----|---------|
| `golang` | `go-build-args` | `go build` |
| `rust` | `cargo-build-args` | `cargo build` |
| `cmake` | `cmake-config-args` | `cmake -B build -S .` |
| `make` | `make-args` | `make` |
| `autotools` | `configure-args` | `./configure` |
| `meson` | `meson-args` | `meson setup build` |
| `maven` | `maven-args` | `mvn clean package` |
| `poetry` | `poetry-args` | `poetry install` |

Environment variables such as `$ZPKG_DEST` are expanded inside arguments. Arguments must not contain shell metacharacters (`` ` ``, `$(`, `|`, `;`, `>`, `<`, `&&`, `||`, `&`) or network fetch utilities (`curl`, `wget`, `git clone`, `npm install`, `pip install`, `cargo install`).
