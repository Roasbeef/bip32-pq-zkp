GO ?= go
SIBLING_ROOT ?= $(abspath ..)
RISC0_DIR ?= $(SIBLING_ROOT)/risc0
TINYGO_ZKVM_DIR ?= $(SIBLING_ROOT)/tinygo-zkvm
GO_ZKVM_DIR ?= $(SIBLING_ROOT)/go-zkvm
TINYGO_BIN ?= $(TINYGO_ZKVM_DIR)/build/tinygo
GO_GOROOT ?= $(shell $(GO) env GOROOT)
PLATFORM_LIB ?= $(RISC0_DIR)/examples/c-guest/guest/out/platform/riscv32im-risc0-zkvm-elf/release/libzkvm_platform.a
KERNEL ?= $(RISC0_DIR)/risc0/zkos/v1compat/elfs/v1compat.elf
CONVERT := $(GO) run $(GO_ZKVM_DIR)/convert_to_r0bf.go

.PHONY: all check-tools hostcheck platform-standalone bip32-platform-latest execute prove clean

all: bip32-platform-latest

platform-standalone:
	$(MAKE) -C $(RISC0_DIR)/examples/c-guest platform-standalone

check-tools:
	@test -x "$(TINYGO_BIN)" || (echo "missing TinyGo binary: $(TINYGO_BIN)" && exit 1)
	@test -f "$(PLATFORM_LIB)" || (echo "missing platform archive: $(PLATFORM_LIB)" && exit 1)
	@test -f "$(KERNEL)" || (echo "missing kernel ELF: $(KERNEL)" && exit 1)
	@test -d "$(GO_ZKVM_DIR)/go-guest-host" || (echo "missing sibling go-zkvm repo: $(GO_ZKVM_DIR)" && exit 1)

hostcheck:
	$(GO) test ./hostcheck -v

bip32-platform-latest: check-tools
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(TINYGO_BIN) build -target=zkvm-platform -scheduler=none -no-debug -ldflags='-extldflags=$(PLATFORM_LIB)' -o bip32-platform-latest.elf ./guest
	$(CONVERT) bip32-platform-latest.elf $(KERNEL) bip32-platform-latest.bin
	@echo "Built bip32-platform-latest.bin"

execute: bip32-platform-latest
	cd $(GO_ZKVM_DIR)/go-guest-host && cargo run --release -- ../../bip32-pq-zkp/bip32-platform-latest.bin --raw-journal --execute-only --use-test-vector

prove: bip32-platform-latest
	cd $(GO_ZKVM_DIR)/go-guest-host && cargo run --release -- ../../bip32-pq-zkp/bip32-platform-latest.bin --raw-journal --use-test-vector --require-bip86

clean:
	rm -f *.elf *.bin
