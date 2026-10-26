# Builds jp for one target. CGO is off, so any target builds on any host.
#
#   make                              linux/amd64, next to the source
#   make ARCH=arm64                   another architecture
#   make ARCH=arm GOARM=5             32-bit ARM: GOARM 5 (no FPU), 6 or 7
#   make OS=windows                   Windows amd64 (jp.exe)
#   make VERSION=1.5.6 OUT=dist/x     version string and output directory
#
# PIE and stripping are used only on targets that support them; see the
# architecture table in docs/compiling.md. etc/debbuild/debbuild.sh runs
# "make libs" once and then "make binary" for each target.

GO      ?= go
OS      ?= linux
ARCH    ?= amd64
GOARM   ?=
VERSION ?=
OUT     ?= .

BINARY_NAME = jp
EXE := $(if $(filter windows,$(OS)),.exe,)
BUILD_DATE := $(shell date)

# -buildmode=pie needs external (cgo) linking on the other targets
# (s390x, riscv64, 32-bit ARM), which is off here
PIE_TARGETS := linux/amd64 linux/arm64 linux/ppc64le windows/amd64
PIE := $(if $(filter $(OS)/$(ARCH),$(PIE_TARGETS)),-buildmode=pie,)

# stripped by the linker (-s -w): the strip command cannot handle binaries
# for other architectures. Targets where -s -w does not work go here.
NOSTRIP_TARGETS :=
STRIP := $(if $(filter $(OS)/$(ARCH),$(NOSTRIP_TARGETS)),,-s -w)

LDFLAGS := $(STRIP) -X 'main.BUILD=$(BUILD_DATE)'$(if $(VERSION), -X 'main.VERSION=$(VERSION)',)

.PHONY: build binary libs fmt vet clean

build: libs fmt binary

binary:
	mkdir -p $(OUT)
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) GOARM=$(GOARM) $(GO) build -ldflags="$(LDFLAGS)" $(PIE) -o $(OUT)/$(BINARY_NAME)$(EXE)
	ls -l $(OUT)/$(BINARY_NAME)$(EXE)

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

libs:
#	go get -u   # not part of a build: upgrade dependencies deliberately, test, then commit go.mod/go.sum
	$(GO) mod tidy -v
	$(GO) mod verify

clean:
	$(GO) clean
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe
