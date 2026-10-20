# Compiling

janeserver, tarzan and the provisioner jp are separate Go modules in `janeserver/`, `tarzan/` and `provisioner/`. Each needs Go 1.26 or later (the `go` line in its `go.mod`) and nothing else: no C libraries, no SGX or other platform SDKs. The Makefiles expect Go at `/usr/local/go/bin/go`. Builds are tested on Debian 13 on amd64.

- [janeserver](#janeserver)
- [tarzan](#tarzan)
- [jp](#jp)
- [Version and build strings](#version-and-build-strings)
- [Tests](#tests)
- [Building the packages](#building-the-packages)

## janeserver

```bash
cd janeserver
make build
```

This runs `go mod tidy`, `go fmt` and builds a position-independent executable `janeserver` for linux/amd64. `Makefile.arm64` builds for 64-bit ARM. For other systems change `OS` and `ARCH` in the Makefile, or build directly (see below).

To run it from the source tree, copy and edit `config.yaml` and see [Running](running.md#janeserver).

## tarzan

```bash
cd tarzan
make build
```

builds `tarzan` for linux/amd64 (`Makefile.arm64` and `Makefile.arm6` for ARM). tarzan runs on many systems; `go tool dist list` lists the possible `GOOS`/`GOARCH` pairs. For example:

```bash
GOOS=linux   GOARCH=arm64 go build -o tarzan_arm64     # Raspberry Pi 4/5, 64-bit
GOOS=linux   GOARCH=arm   go build -o tarzan_arm       # Raspberry Pi 3
GOOS=windows GOARCH=amd64 go build -o tarzan.exe       # Windows 10/11
GOOS=openbsd GOARCH=amd64 go build -o tarzan_openbsd   # also netbsd, freebsd
GOOS=linux   GOARCH=s390x go build -o tarzan_s390x     # IBM Z, or Hercules
```

What each system can offer differs: Windows controls the TPM itself and has no UEFI or IMA logs; systems without a TPM can still answer `sys/info`.

## jp

```bash
cd provisioner
make build
```

builds and strips `jp` for linux/amd64 (`Makefile.arm64` for ARM). jp also builds for Windows (`GOOS=windows`).

## Version and build strings

janeserver and tarzan show a version and build string at start-up and in the Web UI. The Makefiles set the build string from the date and `VERSION` (default `locally_compiled`):

```bash
make build VERSION=1.5.4
```

or, building directly:

```bash
go build -ldflags="-X 'main.VERSION=1.5.4' -X 'main.BUILD=$(date)'" -o janeserver
```

## Tests

In each module:

```bash
go vet -composites=false ./...      # janeserver; plain go vet ./... for tarzan and provisioner
go test ./...
```

Tests that need a real TPM are skipped unless an environment variable names the device: `JANE_TEST_TPM` (janeserver), `TARZAN_TEST_TPM` (tarzan), `JP_TEST_TPM` (jp), e.g. `TARZAN_TEST_TPM=/dev/tpmrm0 go test ./tpm2`. They create only transient objects. Some jp tests check behaviour without root and are skipped when run as root.

## Building the packages

```bash
cd etc/debbuild
./debbuild.sh
```

Check first that the last line of `versions.txt` is the version you want (`1.5.4 dd mis yyyy`, the month in Welsh). The script compiles janeserver, tarzan and jp, builds the two deb packages in `/tmp/janedebbuild` (`jane_<version>_amd64.deb`, `tarzan_<version>_amd64.deb`), sets the file modes inside them, converts them to RPMs with `alien`, lints them with `lintian`, and gzips the results.

`alien` and `lintian` are optional: without them those steps print errors, but the deb files are still built. The RPMs are not tested (the install scripts assume Debian tools and systemd).

What the packages install and do is described in [Installing with dpkg/apt](deb.md).
