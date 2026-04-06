GO ?= go
GOCC ?= $(GO)
TOOLS_DIR := tools
LOCAL_CUSTOM_GCL := $(CURDIR)/$(TOOLS_DIR)/custom-gcl
GOLANGCI_LINT_VERSION := v1.64.8
SIBLING_ROOT ?= $(abspath ..)
RISC0_DIR ?= $(SIBLING_ROOT)/risc0
TINYGO_ZKVM_DIR ?= $(SIBLING_ROOT)/tinygo-zkvm
GO_ZKVM_DIR ?= $(SIBLING_ROOT)/go-zkvm
TINYGO_BIN ?= $(TINYGO_ZKVM_DIR)/build/tinygo
GO_GOROOT ?= $(shell $(GO) env GOROOT)
PLATFORM_LIB ?= $(RISC0_DIR)/examples/c-guest/guest/out/platform/riscv32im-risc0-zkvm-elf/release/libzkvm_platform.a
KERNEL ?= $(RISC0_DIR)/risc0/zkos/v1compat/elfs/v1compat.elf
CONVERT := $(GO) run $(GO_ZKVM_DIR)/convert_to_r0bf.go
HOST_CMD := ./cmd/bip32-pq-zkp-host
ARTIFACT_DIR ?= ./artifacts
RECEIPT ?= $(ARTIFACT_DIR)/bip32-test-vector.receipt
CLAIM ?= $(ARTIFACT_DIR)/bip32-test-vector.claim.json
PRIV_SEED_HEX ?= $(priv_seed)
BIP32_PATH ?= $(bip_32_path)
PUBKEY ?= $(pubkey)
PATH_COMMITMENT ?= $(path_commitment)
REQUIRE_BIP86 ?= 1
GOFMT_FILES := $(shell find . -type f -name '*.go' -not -path './host/*' -not -path './vendor/*')
NATIVE_GO_PKGS := . ./bip32 ./cmd/bip32-pq-zkp-host ./hostcheck

TRUTHY_VALUES := 1 true TRUE yes YES y Y on ON

GREEN := "\\033[0;32m"
NC := "\\033[0m"
define print
	@echo $(GREEN)$1$(NC)
endef

.PHONY: all check-tools hostcheck platform-standalone host-ffi bip32-platform-latest execute prove verify fmt fmt-check tidy tidy-check local-custom-gcl install-custom-gcl build-native-linter lint-native lint native-check clean

all: bip32-platform-latest

platform-standalone:
	$(MAKE) -C $(RISC0_DIR)/examples/c-guest platform-standalone

check-tools:
	@test -x "$(TINYGO_BIN)" || (echo "missing TinyGo binary: $(TINYGO_BIN)" && exit 1)
	@test -f "$(PLATFORM_LIB)" || (echo "missing platform archive: $(PLATFORM_LIB)" && exit 1)
	@test -f "$(KERNEL)" || (echo "missing kernel ELF: $(KERNEL)" && exit 1)

host-ffi:
	$(MAKE) -C $(GO_ZKVM_DIR) host-ffi

hostcheck:
	$(GO) test ./hostcheck -v

fmt:
	gofmt -w $(GOFMT_FILES)

fmt-check:
	@test -z "$$(gofmt -l $(GOFMT_FILES))" || (gofmt -l $(GOFMT_FILES) && exit 1)

tidy:
	$(GO) mod tidy

tidy-check: tidy
	@test -z "$$(git status --porcelain -- go.mod go.sum)" || (git status --short -- go.mod go.sum && exit 1)

local-custom-gcl:
	@./scripts/local-custom-gcl.sh "$(LOCAL_CUSTOM_GCL)" "$(GOLANGCI_LINT_VERSION)"

install-custom-gcl:
	@./scripts/install-custom-gcl.sh "$(if $(dest),$(dest),$(LOCAL_CUSTOM_GCL))" "$(GOLANGCI_LINT_VERSION)"

build-native-linter:
	@$(call print, "Building native linter binary.")
	@./scripts/install-custom-gcl.sh "$(LOCAL_CUSTOM_GCL)" "$(GOLANGCI_LINT_VERSION)"

lint-native: build-native-linter
	@$(call print, "Linting source (native).")
	GOWORK=off $(LOCAL_CUSTOM_GCL) run -v --timeout=10m $(NATIVE_GO_PKGS)

lint: lint-native

native-check:
	$(GO) build . ./cmd/bip32-pq-zkp-host
	$(GO) test ./hostcheck -v

bip32-platform-latest: check-tools
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(TINYGO_BIN) build -target=zkvm-platform -scheduler=none -no-debug -ldflags='-extldflags=$(PLATFORM_LIB)' -o bip32-platform-latest.elf ./guest
	$(CONVERT) bip32-platform-latest.elf $(KERNEL) bip32-platform-latest.bin
	@echo "Built bip32-platform-latest.bin"

execute: host-ffi bip32-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) execute --guest ./bip32-platform-latest.bin $(call witness_args)

prove: host-ffi bip32-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) prove --guest ./bip32-platform-latest.bin $(call witness_args) --receipt-out ./$(RECEIPT) --claim-out ./$(CLAIM)

verify: host-ffi bip32-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) verify --guest ./bip32-platform-latest.bin --receipt-in ./$(RECEIPT) $(if $(strip $(CLAIM)),--claim-in ./$(CLAIM),) $(call verify_args)

clean:
	rm -f *.elf *.bin
	rm -f bip32-pq-zkp-host
	rm -rf $(ARTIFACT_DIR)

define witness_args
$(if $(strip $(PRIV_SEED_HEX)),--seed-hex $(PRIV_SEED_HEX) --path "$(BIP32_PATH)",--use-test-vector) $(if $(filter $(REQUIRE_BIP86),$(TRUTHY_VALUES)),--require-bip86,)
endef

define verify_args
$(if $(strip $(PUBKEY)),--expected-pubkey $(PUBKEY),) $(if $(strip $(PATH_COMMITMENT)),--expected-path-commitment $(PATH_COMMITMENT),) $(if $(strip $(BIP32_PATH)),--expected-path "$(BIP32_PATH)",) --require-bip86 $(if $(filter $(REQUIRE_BIP86),$(TRUTHY_VALUES)),true,false)
endef
