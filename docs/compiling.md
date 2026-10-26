# Compiling

janeserver, tarzan and the provisioner jp are separate Go modules in `janeserver/`, `tarzan/` and `provisioner/`; janeserver and jp also use the small library module `localnames/` (multicast DNS for `.local` names, #127, see [Running](running.md#host-names-ending-in-local)) through a `replace` in their `go.mod` (nothing to install separately). Each needs Go 1.26 or later (the `go` line in its `go.mod`) and nothing else: no C libraries, no SGX or other platform SDKs. cgo is off, so every target below builds on any host. The Makefiles use the `go` on your `PATH` (`make GO=/usr/local/go/bin/go` to choose another). Builds are tested on Debian 13 on amd64.

- [Building one component](#building-one-component)
- [Architectures](#architectures)
- [Windows](#windows)
- [Version and build strings](#version-and-build-strings)
- [Tests](#tests)
- [Building the packages](#building-the-packages)

## Building one component

Each module has one Makefile (#116). With no settings it builds for linux/amd64 next to the source, as before:

```bash
cd janeserver      # or tarzan, or provisioner (builds jp)
make build
```

`make build` runs `go mod tidy`, `go mod verify` and `go fmt` (`make libs fmt`) and then the binary (`make binary`). The settings, all optional:

| Setting | Default | Meaning |
|---|---|---|
| `OS` | `linux` | Go `GOOS`; `windows` adds `.exe` |
| `ARCH` | `amd64` | Go `GOARCH`: `amd64`, `arm64`, `s390x`, `ppc64le`, `riscv64`, `arm` |
| `GOARM` | (Go's default, 7) | for `ARCH=arm`: `5`, `6` or `7` |
| `VERSION` | (the source's default) | version string, e.g. `1.5.6` |
| `OUT` | `.` | output directory |

```bash
make binary ARCH=arm64                           # 64-bit ARM
make binary ARCH=arm GOARM=5 OUT=dist/linux-armv5  # 32-bit ARM without FPU
make binary ARCH=s390x VERSION=1.5.6             # IBM Z
```

The per-architecture Makefiles (`Makefile.arm64`, `etc/debbuild/Makefile.*.amd64`) were removed in 1.5.6.

To run janeserver from the source tree, copy and edit `config.yaml` and see [Running](running.md#janeserver).

## Architectures

The targets that are built and packaged (decisions in the 1.5.6 plan: build only what is packaged, plus Windows amd64). PIE (`-buildmode=pie`) and stripping are used only where the target supports them; the Makefiles choose automatically.

| Debian architecture | Go `GOARCH` / `GOARM` | Typical CPUs and boards | janeserver | tarzan | jp | PIE | Stripped |
|---|---|---|---|---|---|---|---|
| amd64 | `amd64` | x86-64 PCs and servers, VMs | ✓ | ✓ | ✓ | yes | yes |
| arm64 | `arm64` | ARM servers, Raspberry Pi 3/4/5 (64-bit OS), Apple-silicon VMs | ✓ | ✓ | ✓ | yes | yes |
| s390x | `s390x` | IBM Z / LinuxONE (or Hercules, QEMU) | ✓ | ✓ | ✓ | no | yes |
| ppc64el | `ppc64le` | IBM POWER8 and later, little-endian | — | ✓ | ✓ | yes | yes |
| riscv64 | `riscv64` | RISC-V boards (e.g. VisionFive 2, Milk-V) | — | ✓ | ✓ | no | yes |
| armhf (ARMv7) | `arm`, `GOARM=7` | 32-bit ARMv7 boards with FPU (Cortex-A7/A8/A9/A15) | — | ✓ | ✓ | no | yes |
| armhf (ARMv6) | `arm`, `GOARM=6` | ARMv6 with FPU: Raspberry Pi Zero/1 (ARM1176) | — | ✓ | ✓ | no | yes |
| armel (ARMv5) | `arm`, `GOARM=5` | any 32-bit ARM, also without FPU (software floating point) | — | ✓ | ✓ | no | yes |
| (Windows) | `windows`/`amd64` | Windows 10/11, Server, x86-64 | ✓ | ✓ | ✓ | yes | yes |

- **PIE:** without cgo, Go can only link position-independent executables itself on amd64, arm64, ppc64le and Windows; on s390x, riscv64 and 32-bit ARM `-buildmode=pie` needs external (cgo) linking and fails, so it is left out there.
- **Stripping:** done by the linker (`-ldflags="-s -w"`, which drops the symbol table and DWARF debug information; panics still show function names and lines). The `strip` command is not used: it cannot handle binaries for other architectures. A target where `-s -w` failed would be listed in `NOSTRIP_TARGETS` in the Makefiles; there are none.
- **GOARM:** `GOARM=6` and `7` use the hardware FPU; a board without one needs the `GOARM=5` build.

## Windows

```bash
make binary OS=windows      # in janeserver, tarzan or provisioner
```

janeserver, tarzan and jp build for Windows amd64 (#122). They are shipped as zip files, not packages, with the same programs and configuration files as the packages and a short `WINDOWS.txt`; there is no installer or service wrapper. On Windows tarzan opens the TPM through TPM Base Services; what Windows lets through is best effort (see [Running](running.md#tarzan)).

- `jane_<version>_windows-amd64.zip`: `janeserver.exe`, `addjaneuser` (Python, needs `argon2-cffi` and `pymongo`), `config.yaml`, `jane.env`, `REPLACE_ME.crt`/`.key`, `standardintents.json`, `standardobjects.json`.
- `tarzan_<version>_windows-amd64.zip`: `tarzan.exe`, `jp.exe`, `provisioning.yaml.example`, `skeletons\` (the four [jp skeletons](jp.md#skeletons)), `tarzan.env`.

## Version and build strings

janeserver, tarzan and jp show a version and build string at start-up (janeserver also in the Web UI). The Makefiles always set the build string (the date) and set the version when `VERSION` is given:

```bash
make build VERSION=1.5.6
```

or, building directly:

```bash
go build -ldflags="-X 'main.VERSION=1.5.6' -X 'main.BUILD=$(date)'" -o janeserver
```

## Tests

In each module:

```bash
go vet -composites=false ./...      # janeserver; plain go vet ./... for tarzan and provisioner (make vet does the right one)
go test ./...
```

and in `localnames/`, `go mod tidy && go test ./...`.

Tests that need a real TPM are skipped unless an environment variable names the device: `JANE_TEST_TPM` (janeserver), `TARZAN_TEST_TPM` (tarzan), `JP_TEST_TPM` (jp), e.g. `TARZAN_TEST_TPM=/dev/tpmrm0 go test ./tpm2`. They create only transient objects. Some jp tests check behaviour without root and are skipped when run as root.

## Building the packages

```bash
cd etc/debbuild
./debbuild.sh
```

Check first that the last line of `versions.txt` is the version you want (`1.5.6 dd mis yyyy`, the month in Welsh). The script runs `make libs fmt` once per module, builds every target with `make binary` into `<module>/dist/linux-<target>/`, and builds the packages in `/tmp/janedebbuild`, gzipped:

- `jane_<version>_<arch>.deb` for amd64, arm64, s390x;
- `tarzan_<version>_<arch>.deb` for amd64, arm64, s390x, ppc64el, riscv64, `armel-armv5`, `armhf-armv6`, `armhf-armv7`;
- the two [Windows](#windows) zips (not gzipped), made with Python's `zipfile` module, so no zip tool is needed.

The control files get their `Architecture:` from the target (`armhf` for both armhf packages). The script stops at the first error. `alien` (RPMs) and `lintian` are no longer run; the lines are left commented out in the script.

What the packages install and do, and which package suits which machine, is described in [Installing with dpkg/apt](deb.md).
