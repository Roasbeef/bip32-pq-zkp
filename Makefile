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
BATCH_RECEIPT ?= $(ARTIFACT_DIR)/hardened-xpriv-batch.receipt
BATCH_CLAIM ?= $(ARTIFACT_DIR)/hardened-xpriv-batch.claim.json
BATCH_INCLUSION ?= $(batch_inclusion)
BATCH_INCLUSIONS ?= $(if $(strip $(batch_inclusions)),$(batch_inclusions),$(BATCH_INCLUSION))
BATCH_INCLUSION_CHAIN ?= $(if $(strip $(batch_inclusion_chain)),$(batch_inclusion_chain),$(ARTIFACT_DIR)/batch.inclusion-chain.json)
BATCH_INCLUSION_OUT ?= $(if $(strip $(batch_inclusion_out)),$(batch_inclusion_out),$(ARTIFACT_DIR)/hardened-xpriv-batch.inclusion.json)
PARENT_BATCH_RECEIPT ?= $(if $(strip $(parent_batch_receipt)),$(parent_batch_receipt),$(ARTIFACT_DIR)/parent-batch.receipt)
PARENT_BATCH_CLAIM ?= $(if $(strip $(parent_batch_claim)),$(parent_batch_claim),$(ARTIFACT_DIR)/parent-batch.claim.json)
PARENT_BATCH_INCLUSION_OUT ?= $(if $(strip $(parent_batch_inclusion_out)),$(parent_batch_inclusion_out),$(ARTIFACT_DIR)/parent-batch.inclusion.json)
PARENT_BATCH_LEAF_INDEX ?= $(if $(strip $(parent_batch_leaf_index)),$(parent_batch_leaf_index),0)
PARENT_BATCH_CHILD_CLAIMS ?= $(if $(strip $(parent_batch_child_claims)),$(parent_batch_child_claims),$(ARTIFACT_DIR)/child-a.claim.json $(ARTIFACT_DIR)/child-b.claim.json)
PARENT_BATCH_CHILD_RECEIPTS ?= $(if $(strip $(parent_batch_child_receipts)),$(parent_batch_child_receipts),$(ARTIFACT_DIR)/child-a.receipt $(ARTIFACT_DIR)/child-b.receipt)
HARDENED_XPUB_RECEIPT ?= $(ARTIFACT_DIR)/hardened-xpub-test-vector.receipt
HARDENED_XPUB_CLAIM ?= $(ARTIFACT_DIR)/hardened-xpub-test-vector.claim.json
HARDENED_XPRIV_RECEIPT ?= $(ARTIFACT_DIR)/hardened-xpriv-test-vector.receipt
HARDENED_XPRIV_CLAIM ?= $(ARTIFACT_DIR)/hardened-xpriv-test-vector.claim.json
PRIV_SEED_HEX ?= $(priv_seed)
BIP32_PATH ?= $(bip_32_path)
PUBKEY ?= $(pubkey)
PATH_COMMITMENT ?= $(path_commitment)
PARENT_XPRIV_HEX ?= $(parent_xpriv)
PARENT_CHAIN_CODE_HEX ?= $(parent_chain_code)
HARDENED_PATH ?= $(hardened_path)
EXPECTED_COMPRESSED_PUBKEY ?= $(expected_compressed_pubkey)
EXPECTED_CHILD_PRIVATE_KEY ?= $(expected_child_private_key)
EXPECTED_CHAIN_CODE ?= $(expected_chain_code)
BATCH_LEAF_KIND ?= $(if $(strip $(leaf_kind)),$(leaf_kind),hardened-xpriv)
BATCH_LEAF_CLAIMS ?= $(if $(strip $(leaf_claims)),$(leaf_claims),$(ARTIFACT_DIR)/hardened-xpriv-succinct.claim.json $(ARTIFACT_DIR)/hardened-xpriv-succinct.claim.json)
BATCH_LEAF_RECEIPTS ?= $(if $(strip $(leaf_receipts)),$(leaf_receipts),$(ARTIFACT_DIR)/hardened-xpriv-succinct.receipt $(ARTIFACT_DIR)/hardened-xpriv-succinct.receipt)
BATCH_LEAF_INDEX ?= $(if $(strip $(leaf_index)),$(leaf_index),0)
REQUIRE_BIP86 ?= 1
RECEIPT_KIND ?= $(if $(strip $(receipt_kind)),$(receipt_kind),composite)
GOFMT_FILES := $(shell find . -type f -name '*.go' -not -path './host/*' -not -path './vendor/*')
NATIVE_GO_PKGS := . ./batchclaim ./bip32 ./cmd/bip32-pq-zkp-host ./hostcheck
NATIVE_TEST_PKGS := . ./batchclaim ./cmd/bip32-pq-zkp-host ./hostcheck

TRUTHY_VALUES := 1 true TRUE yes YES y Y on ON

GREEN := "\\033[0;32m"
NC := "\\033[0m"
define print
	@echo $(GREEN)$1$(NC)
endef

.PHONY: all check-tools hostcheck platform-standalone host-ffi bip32-platform-latest batch-platform-latest hardened-xpub-platform-latest hardened-xpriv-platform-latest execute prove verify execute-batch prove-batch verify-batch derive-batch-inclusion bundle-batch-inclusion-chain prove-parent-batch derive-parent-batch-inclusion verify-nested-batch execute-hardened-xpub prove-hardened-xpub verify-hardened-xpub execute-hardened-xpriv prove-hardened-xpriv verify-hardened-xpriv fmt fmt-check tidy tidy-check local-custom-gcl install-custom-gcl build-native-linter lint-native lint native-check clean

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
	$(GO) test $(NATIVE_TEST_PKGS) -v

bip32-platform-latest: check-tools
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(TINYGO_BIN) build -target=zkvm-platform -scheduler=none -no-debug -ldflags='-extldflags=$(PLATFORM_LIB)' -o bip32-platform-latest.elf ./guest
	$(CONVERT) bip32-platform-latest.elf $(KERNEL) bip32-platform-latest.bin
	@echo "Built bip32-platform-latest.bin"

hardened-xpub-platform-latest: check-tools
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(TINYGO_BIN) build -target=zkvm-platform -scheduler=none -no-debug -ldflags='-extldflags=$(PLATFORM_LIB)' -o hardened-xpub-platform-latest.elf ./guest_hardened_xpub
	$(CONVERT) hardened-xpub-platform-latest.elf $(KERNEL) hardened-xpub-platform-latest.bin
	@echo "Built hardened-xpub-platform-latest.bin"

hardened-xpriv-platform-latest: check-tools
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(TINYGO_BIN) build -target=zkvm-platform -scheduler=none -no-debug -ldflags='-extldflags=$(PLATFORM_LIB)' -o hardened-xpriv-platform-latest.elf ./guest_hardened_xpriv
	$(CONVERT) hardened-xpriv-platform-latest.elf $(KERNEL) hardened-xpriv-platform-latest.bin
	@echo "Built hardened-xpriv-platform-latest.bin"

batch-platform-latest: check-tools
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(TINYGO_BIN) build -target=zkvm-platform -scheduler=none -no-debug -ldflags='-extldflags=$(PLATFORM_LIB)' -o batch-platform-latest.elf ./guest_batch
	$(CONVERT) batch-platform-latest.elf $(KERNEL) batch-platform-latest.bin
	@echo "Built batch-platform-latest.bin"

execute: host-ffi bip32-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) execute --guest ./bip32-platform-latest.bin $(call witness_args)

prove: host-ffi bip32-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) prove --guest ./bip32-platform-latest.bin $(call witness_args) --receipt-kind $(RECEIPT_KIND) --receipt-out $(RECEIPT) --claim-out $(CLAIM)

verify: host-ffi bip32-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) verify --guest ./bip32-platform-latest.bin --receipt-in $(RECEIPT) $(if $(strip $(CLAIM)),--claim-in $(CLAIM),) $(call verify_args)

execute-batch: host-ffi batch-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) execute-batch --guest ./batch-platform-latest.bin --leaf-kind $(BATCH_LEAF_KIND) $(call batch_leaf_args)

prove-batch: host-ffi batch-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) prove-batch --guest ./batch-platform-latest.bin --leaf-kind $(BATCH_LEAF_KIND) $(call batch_leaf_args) --receipt-kind $(RECEIPT_KIND) --receipt-out $(BATCH_RECEIPT) --claim-out $(BATCH_CLAIM)

verify-batch: host-ffi batch-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) verify-batch --guest ./batch-platform-latest.bin --receipt-in $(BATCH_RECEIPT) $(if $(strip $(BATCH_CLAIM)),--claim-in $(BATCH_CLAIM),) $(call batch_inclusion_args) $(if $(strip $(BATCH_INCLUSION_CHAIN)),--inclusion-chain-in $(BATCH_INCLUSION_CHAIN),)

derive-batch-inclusion: host-ffi
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) derive-batch-inclusion --leaf-kind $(BATCH_LEAF_KIND) $(call batch_leaf_args) --leaf-index $(BATCH_LEAF_INDEX) --proof-out $(BATCH_INCLUSION_OUT)

bundle-batch-inclusion-chain: host-ffi
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) bundle-batch-inclusion-chain $(call batch_inclusion_args) --chain-out $(BATCH_INCLUSION_CHAIN)

prove-parent-batch: host-ffi batch-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) prove-batch --guest ./batch-platform-latest.bin --leaf-kind batch-claim-v1 $(call parent_batch_leaf_args) --receipt-kind $(RECEIPT_KIND) --receipt-out $(PARENT_BATCH_RECEIPT) --claim-out $(PARENT_BATCH_CLAIM)

derive-parent-batch-inclusion: host-ffi
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) derive-batch-inclusion --leaf-kind batch-claim-v1 $(call parent_batch_leaf_args) --leaf-index $(PARENT_BATCH_LEAF_INDEX) --proof-out $(PARENT_BATCH_INCLUSION_OUT)

verify-nested-batch: host-ffi batch-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) verify-batch --guest ./batch-platform-latest.bin --receipt-in $(PARENT_BATCH_RECEIPT) $(if $(strip $(PARENT_BATCH_CLAIM)),--claim-in $(PARENT_BATCH_CLAIM),) $(if $(strip $(BATCH_INCLUSION_CHAIN)),--inclusion-chain-in $(BATCH_INCLUSION_CHAIN),)

execute-hardened-xpub: host-ffi hardened-xpub-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) execute-hardened-xpub --guest ./hardened-xpub-platform-latest.bin $(call hardened_xpub_witness_args)

prove-hardened-xpub: host-ffi hardened-xpub-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) prove-hardened-xpub --guest ./hardened-xpub-platform-latest.bin $(call hardened_xpub_witness_args) --receipt-kind $(RECEIPT_KIND) --receipt-out $(HARDENED_XPUB_RECEIPT) --claim-out $(HARDENED_XPUB_CLAIM)

verify-hardened-xpub: host-ffi hardened-xpub-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) verify-hardened-xpub --guest ./hardened-xpub-platform-latest.bin --receipt-in $(HARDENED_XPUB_RECEIPT) $(if $(strip $(HARDENED_XPUB_CLAIM)),--claim-in $(HARDENED_XPUB_CLAIM),) $(call hardened_xpub_verify_args)

execute-hardened-xpriv: host-ffi hardened-xpriv-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) execute-hardened-xpriv --guest ./hardened-xpriv-platform-latest.bin $(call hardened_xpriv_witness_args)

prove-hardened-xpriv: host-ffi hardened-xpriv-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) prove-hardened-xpriv --guest ./hardened-xpriv-platform-latest.bin $(call hardened_xpriv_witness_args) --receipt-kind $(RECEIPT_KIND) --receipt-out $(HARDENED_XPRIV_RECEIPT) --claim-out $(HARDENED_XPRIV_CLAIM)

verify-hardened-xpriv: host-ffi hardened-xpriv-platform-latest
	PATH=$(GO_GOROOT)/bin:$$PATH GOROOT=$(GO_GOROOT) $(GO) run $(HOST_CMD) verify-hardened-xpriv --guest ./hardened-xpriv-platform-latest.bin --receipt-in $(HARDENED_XPRIV_RECEIPT) $(if $(strip $(HARDENED_XPRIV_CLAIM)),--claim-in $(HARDENED_XPRIV_CLAIM),) $(call hardened_xpriv_verify_args)

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

define hardened_xpub_witness_args
$(if $(strip $(PARENT_XPRIV_HEX)),--parent-xpriv-hex $(PARENT_XPRIV_HEX) --parent-chain-code-hex $(PARENT_CHAIN_CODE_HEX) --path "$(HARDENED_PATH)",--use-test-vector)
endef

define hardened_xpub_verify_args
$(if $(strip $(EXPECTED_COMPRESSED_PUBKEY)),--expected-compressed-pubkey $(EXPECTED_COMPRESSED_PUBKEY),) $(if $(strip $(EXPECTED_CHAIN_CODE)),--expected-chain-code $(EXPECTED_CHAIN_CODE),)
endef

define hardened_xpriv_witness_args
$(if $(strip $(PARENT_XPRIV_HEX)),--parent-xpriv-hex $(PARENT_XPRIV_HEX) --parent-chain-code-hex $(PARENT_CHAIN_CODE_HEX) --path "$(HARDENED_PATH)",--use-test-vector)
endef

define hardened_xpriv_verify_args
$(if $(strip $(EXPECTED_CHILD_PRIVATE_KEY)),--expected-child-private-key $(EXPECTED_CHILD_PRIVATE_KEY),) $(if $(strip $(EXPECTED_CHAIN_CODE)),--expected-chain-code $(EXPECTED_CHAIN_CODE),)
endef

define batch_leaf_args
$(foreach claim,$(BATCH_LEAF_CLAIMS),--leaf-claim $(claim)) $(foreach receipt,$(BATCH_LEAF_RECEIPTS),--leaf-receipt $(receipt))
endef

define batch_inclusion_args
$(foreach proof,$(BATCH_INCLUSIONS),--inclusion-in $(proof))
endef

define parent_batch_leaf_args
$(foreach claim,$(PARENT_BATCH_CHILD_CLAIMS),--leaf-claim $(claim)) $(foreach receipt,$(PARENT_BATCH_CHILD_RECEIPTS),--leaf-receipt $(receipt))
endef
