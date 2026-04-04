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
HOST_DIR := ./host
ARTIFACT_DIR ?= ./artifacts
RECEIPT ?= $(ARTIFACT_DIR)/bip32-test-vector.receipt
CLAIM ?= $(ARTIFACT_DIR)/bip32-test-vector.claim.json
PRIV_SEED_HEX ?= $(priv_seed)
BIP32_PATH ?= $(bip_32_path)
PUBKEY ?= $(pubkey)
PATH_COMMITMENT ?= $(path_commitment)
REQUIRE_BIP86 ?= 1

TRUTHY_VALUES := 1 true TRUE yes YES y Y on ON

.PHONY: all check-tools hostcheck platform-standalone bip32-platform-latest execute prove verify clean

all: bip32-platform-latest

platform-standalone:
	$(MAKE) -C $(RISC0_DIR)/examples/c-guest platform-standalone

check-tools:
	@test -x "$(TINYGO_BIN)" || (echo "missing TinyGo binary: $(TINYGO_BIN)" && exit 1)
	@test -f "$(PLATFORM_LIB)" || (echo "missing platform archive: $(PLATFORM_LIB)" && exit 1)
	@test -f "$(KERNEL)" || (echo "missing kernel ELF: $(KERNEL)" && exit 1)
	@test -d "$(HOST_DIR)" || (echo "missing local host crate: $(HOST_DIR)" && exit 1)

hostcheck:
	$(GO) test ./hostcheck -v

bip32-platform-latest: check-tools
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(TINYGO_BIN) build -target=zkvm-platform -scheduler=none -no-debug -ldflags='-extldflags=$(PLATFORM_LIB)' -o bip32-platform-latest.elf ./guest
	$(CONVERT) bip32-platform-latest.elf $(KERNEL) bip32-platform-latest.bin
	@echo "Built bip32-platform-latest.bin"

execute: bip32-platform-latest
	cd $(HOST_DIR) && cargo run --release -- execute --guest ../bip32-platform-latest.bin $(call witness_args)

prove: bip32-platform-latest
	cd $(HOST_DIR) && cargo run --release -- prove --guest ../bip32-platform-latest.bin $(call witness_args) --receipt-out ../$(RECEIPT) --claim-out ../$(CLAIM)

verify: bip32-platform-latest
	cd $(HOST_DIR) && cargo run --release -- verify --guest ../bip32-platform-latest.bin --receipt-in ../$(RECEIPT) $(if $(strip $(CLAIM)),--claim-in ../$(CLAIM),) $(call verify_args)

clean:
	rm -f *.elf *.bin
	rm -rf $(ARTIFACT_DIR)

define witness_args
$(if $(strip $(PRIV_SEED_HEX)),--seed-hex $(PRIV_SEED_HEX) --path "$(BIP32_PATH)",--use-test-vector) $(if $(filter $(REQUIRE_BIP86),$(TRUTHY_VALUES)),--require-bip86,)
endef

define verify_args
$(if $(strip $(PUBKEY)),--expected-pubkey $(PUBKEY),) $(if $(strip $(PATH_COMMITMENT)),--expected-path-commitment $(PATH_COMMITMENT),) $(if $(strip $(BIP32_PATH)),--expected-path "$(BIP32_PATH)",) --require-bip86 $(if $(filter $(REQUIRE_BIP86),$(TRUTHY_VALUES)),true,false)
endef
