# Running bip32-pq-zkp

This repo carries its own demo-specific Go host command in
`cmd/bip32-pq-zkp-host/`. The sibling `go-zkvm` repo still provides the guest
packaging and proving engine boundary, but `bip32-pq-zkp` owns the
demo-specific UX across all three proof lanes:

- Full Taproot: `execute`, `prove`, `verify`
- Hardened xpub: `execute-hardened-xpub`, `prove-hardened-xpub`,
  `verify-hardened-xpub`
- Hardened xpriv: `execute-hardened-xpriv`, `prove-hardened-xpriv`,
  `verify-hardened-xpriv`
- Batch aggregation: `execute-batch`, `prove-batch`, `verify-batch`,
  `derive-batch-inclusion`, `bundle-batch-inclusion-chain`,
  `run-nested-batch-plan`

## Expected Sibling Layout

```text
github.com/roasbeef/
├── risc0
├── tinygo-zkvm
├── go-zkvm
└── bip32-pq-zkp
```

## Dependencies

- `roasbeef/risc0`
- `roasbeef/tinygo-zkvm`
- `roasbeef/go-zkvm`

Fresh-clone setup notes:

- in `../tinygo-zkvm`, run `git submodule update --init --recursive`
- in `../risc0`, run `git lfs pull` before building the local prover path

If your default `go` is newer than the TinyGo lane supports, export:

```bash
export GO_GOROOT=/path/to/go1.24.4
```

## Current Flow

1. build the TinyGo fork
2. build the risc0 platform archive with `make platform-standalone`
3. build the `bip32` guest with TinyGo target `zkvm-platform`
4. package it with `v1compat.elf`
5. run host-side reference tests
6. execute, prove, or verify with the demo-specific Go host command

From this repo root, the archive step can be proxied with:

```bash
make platform-standalone
```

## Reference Check

The host-side reference test compares the local helper code against
`btcd/txscript.ComputeTaprootKeyNoScript`.

Run:

```bash
go test ./hostcheck -v
```

## Make Variables

The top-level Makefile supports both explicit uppercase variables and the
lowercase aliases you have been using interactively. Lowercase aliases are
variable aliases that map to the uppercase forms.

- `PRIV_SEED_HEX` or `priv_seed`
- `BIP32_PATH` or `bip_32_path`
- `PUBKEY` or `pubkey`
- `PATH_COMMITMENT` or `path_commitment`
- `REQUIRE_BIP86`
- `RECEIPT_KIND` or `receipt_kind`
- `RECEIPT`
- `CLAIM`
- `BATCH_LEAF_KIND`
- `BATCH_LEAF_CLAIMS`
- `BATCH_LEAF_RECEIPTS`
- `BATCH_DIRECT_LEAF_KINDS`
- `BATCH_RECEIPT`
- `BATCH_CLAIM`
- `BATCH_INCLUSION`
- `BATCH_INCLUSION_OUT`
- `BATCH_LEAF_INDEX`
- `NESTED_BATCH_PLAN`

Variable roles:

- prove/execute witness inputs:
  - `PRIV_SEED_HEX`
  - `BIP32_PATH`
- `REQUIRE_BIP86`
- `RECEIPT_KIND`
- verify-time expectations:
  - `CLAIM`
  - `PUBKEY`
  - `PATH_COMMITMENT`
  - `BIP32_PATH`
  - `REQUIRE_BIP86`

Important distinction:

- `PUBKEY` and `PATH_COMMITMENT` are verifier-side checks only
- they are never private proving inputs
- for `make verify`, `BIP32_PATH` means "public path supplied to the verifier
  so it can recompute the expected path commitment"
- for `make prove`, `BIP32_PATH` is private witness input sent to the guest

`BIP32_PATH` accepts either slash form or comma form:

```text
m/86'/0'/0'/0/0
86',0',0',0,0
```

Default behavior:

- bare `make execute` or `make prove` uses the built-in BIP-32 test vector
- bare `make verify` expects the default receipt and claim artifacts from a
  prior `make prove`
- bare `make execute`, `make prove`, and `make verify` all default to
  `require_bip86=true`
- bare `make prove` defaults to `RECEIPT_KIND=composite`
- if you set `PRIV_SEED_HEX`, you should also set `BIP32_PATH`

Canonical verifier path:

- `make prove` emits both:
  - a receipt
  - a canonical `claim.json`
- `make verify` is intended to verify that pair by default
- the documented demo lane assumes `REQUIRE_BIP86=1` unless you opt out
- direct `PUBKEY`, `PATH_COMMITMENT`, or `BIP32_PATH` checks are optional
  tighter/manual checks, not the primary verifier UX

## Execute

Built-in test vector:

```bash
make execute GO_GOROOT=/path/to/go1.24.4
```

Explicit private witness:

```bash
make execute GO_GOROOT=/path/to/go1.24.4 \
  PRIV_SEED_HEX=000102030405060708090a0b0c0d0e0f \
  BIP32_PATH="86',0',0',0,0" \
  REQUIRE_BIP86=1
```

Opt out of BIP-86 for a non-BIP-86 derivation:

```bash
make prove GO_GOROOT=/path/to/go1.24.4 \
  PRIV_SEED_HEX=000102030405060708090a0b0c0d0e0f \
  BIP32_PATH="44',0',0',0,0" \
  REQUIRE_BIP86=0
```

## Prove

Built-in test vector:

```bash
make prove GO_GOROOT=/path/to/go1.24.4
```

Built-in test vector with a recursively compressed receipt:

```bash
make prove GO_GOROOT=/path/to/go1.24.4 RECEIPT_KIND=succinct
```

Explicit private witness:

```bash
make prove GO_GOROOT=/path/to/go1.24.4 \
  PRIV_SEED_HEX=000102030405060708090a0b0c0d0e0f \
  BIP32_PATH="86',0',0',0,0" \
  REQUIRE_BIP86=1
```

This writes:

- receipt artifact: `./artifacts/bip32-test-vector.receipt`
- canonical claim artifact: `./artifacts/bip32-test-vector.claim.json`

## Verify

Verify against the emitted canonical claim artifact:

```bash
make verify GO_GOROOT=/path/to/go1.24.4
```

Verify against both the canonical claim artifact and explicit expected public
material:

```bash
make verify GO_GOROOT=/path/to/go1.24.4 \
  PUBKEY=00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6 \
  BIP32_PATH="86',0',0',0,0" \
  REQUIRE_BIP86=1
```

Verify using only direct public expectations and no claim JSON:

```bash
make verify GO_GOROOT=/path/to/go1.24.4 \
  CLAIM= \
  PUBKEY=00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6 \
  BIP32_PATH="86',0',0',0,0" \
  REQUIRE_BIP86=1
```

If the verifier knows the path commitment directly instead of the path itself,
replace `BIP32_PATH=...` with `PATH_COMMITMENT=<hex>`.

Policy note:

- the demo keeps a single guest image for both modes
- `require_bip86` is carried as a verifier-visible public claim flag
- opting out changes the public claim policy bit, not the host/image model
- `RECEIPT_KIND` changes only the receipt representation, not the public claim
  semantics

## Batch Aggregation

The batch guest consumes existing succinct leaf receipts as assumptions,
verifies them inside one aggregation guest, and commits only a fixed-size
batch claim:

- `leaf_claim_kind`
- `leaf_guest_image_id`
- `leaf_count`
- `merkle_root`

The default batch Makefile lane uses the hardened-xpriv leaf schema and two
copies of the existing succinct hardened-xpriv leaf artifacts:

```bash
make execute-batch GO_GOROOT=/path/to/go1.24.4
make prove-batch GO_GOROOT=/path/to/go1.24.4
make derive-batch-inclusion GO_GOROOT=/path/to/go1.24.4
make verify-batch GO_GOROOT=/path/to/go1.24.4 \
  BATCH_INCLUSION=./artifacts/hardened-xpriv-batch.inclusion.json
```

That writes:

- batch receipt: `./artifacts/hardened-xpriv-batch.receipt`
- batch claim: `./artifacts/hardened-xpriv-batch.claim.json`
- sparse inclusion proof:
  `./artifacts/hardened-xpriv-batch.inclusion.json`

To request a succinct final batch receipt instead:

```bash
make prove-batch GO_GOROOT=/path/to/go1.24.4 \
  RECEIPT_KIND=succinct \
  BATCH_RECEIPT=./artifacts/hardened-xpriv-batch-succinct.receipt \
  BATCH_CLAIM=./artifacts/hardened-xpriv-batch-succinct.claim.json
```

To reuse the same batch guest with the original full Taproot leaf claim,
override the leaf inputs with existing succinct Taproot leaf artifacts:

```bash
make prove-batch GO_GOROOT=/path/to/go1.24.4 \
  BATCH_LEAF_KIND=taproot \
  BATCH_LEAF_CLAIMS="./artifacts/bip32-succinct.claim.json ./artifacts/bip32-succinct.claim.json" \
  BATCH_LEAF_RECEIPTS="./artifacts/bip32-succinct.receipt ./artifacts/bip32-succinct.receipt" \
  BATCH_RECEIPT=./artifacts/taproot-batch.receipt \
  BATCH_CLAIM=./artifacts/taproot-batch.claim.json
```

Current validated hardened-xpriv batch scaling matrix:

| N | Kind | Receipt bytes | Seal bytes | Claim JSON | Inclusion JSON | Prove sec | Verify sec |
|---|------|---------------|------------|------------|----------------|-----------|------------|
| 2 | composite | 681,214 | 679,904 | 755 | 456 | 2.06 | 0.04 |
| 2 | succinct  | 223,343 | 222,668 | 755 | 456 | 5.35 | 0.02 |
| 4 | composite | 1,138,062 | 1,135,864 | 756 | 528 | 3.66 | 0.06 |
| 4 | succinct  | 223,343 | 222,668 | 755 | 528 | 9.44 | 0.02 |
| 8 | composite | 2,042,158 | 2,038,184 | 756 | 600 | 7.27 | 0.12 |
| 8 | succinct  | 223,343 | 222,668 | 755 | 600 | 17.74 | 0.04 |
| 16 | composite | 4,072,409 | 4,064,720 | 757 | 673 | 11.24 | 0.22 |
| 16 | succinct  | 223,343 | 222,668 | 756 | 673 | 33.80 | 0.04 |

Current validated batch identity values:

- hardened-xpriv leaf guest image ID:
  - `8401a36e4f54cb2beaf9ac7677603806cf9d775e90ef5a70168045a3c0df0849`
- current two-leaf hardened-xpriv batch Merkle root:
  - `0a0a1d7c7baf543b60321fb0303a4a70d46a6ba8371399110d1affb43efc03c0`
- current validated three-level nested top root:
  - `8fabf9c04a03e18f47ef37fe23c3bdbfb9984767d77b055b53a5ae10e4d7aaf3`

The batch guest image ID is expected to change whenever the batch guest code
or batch-claim helpers change, so the docs keep the stable Merkle-root
examples here rather than pinning one transient batch guest image ID.

Smaller confirmation matrix for the original full Taproot leaf schema:

| N | Kind | Receipt bytes | Claim JSON | Inclusion JSON | Prove sec |
|---|------|---------------|------------|----------------|-----------|
| 2 | composite | 681,214 | 748 | 449 | 2.64 |
| 2 | succinct  | 223,343 | 748 | 449 | 6.43 |
| 8 | composite | 2,042,158 | 749 | 593 | 10.35 |
| 8 | succinct  | 223,343 | 748 | 593 | 21.22 |

The key measured takeaway is that the final succinct batch receipt stays flat
at ~223 KB across the current matrix, while the batch fan-out shows up in the
Merkle inclusion artifact rather than the final receipt itself.

### Nested Batch Claims

The first hierarchical batch layer is implemented with the same batch guest.
Parent batches use child batch `claim.json` artifacts as 84-byte leaves, and
the canonical verifier path now uses one bundled inclusion-chain JSON file:

- child batches are still built with `prove-batch`
- parent batches switch to `BATCH_LEAF_KIND=batch-claim-v1`
- `bundle-batch-inclusion-chain` combines one proof per level into one
  verifier artifact
- `verify-nested-batch` consumes that bundled chain directly

A minimal two-level example now looks like:

```bash
make prove-parent-batch \
  GO_GOROOT=/path/to/go1.24.4 \
  RECEIPT_KIND=succinct \
  PARENT_BATCH_RECEIPT=/tmp/parent.receipt \
  PARENT_BATCH_CLAIM=/tmp/parent.claim.json \
  PARENT_BATCH_CHILD_CLAIMS="/tmp/child-a.claim.json /tmp/child-b.claim.json" \
  PARENT_BATCH_CHILD_RECEIPTS="/tmp/child-a.receipt /tmp/child-b.receipt"

make derive-parent-batch-inclusion \
  GO_GOROOT=/path/to/go1.24.4 \
  PARENT_BATCH_INCLUSION_OUT=/tmp/parent.inclusion.json \
  PARENT_BATCH_LEAF_INDEX=1 \
  PARENT_BATCH_CHILD_CLAIMS="/tmp/child-a.claim.json /tmp/child-b.claim.json" \
  PARENT_BATCH_CHILD_RECEIPTS="/tmp/child-a.receipt /tmp/child-b.receipt"

make bundle-batch-inclusion-chain \
  GO_GOROOT=/path/to/go1.24.4 \
  BATCH_INCLUSIONS="/tmp/parent.inclusion.json /tmp/child-b.inclusion.json" \
  BATCH_INCLUSION_CHAIN=/tmp/parent.chain.json

make verify-nested-batch \
  GO_GOROOT=/path/to/go1.24.4 \
  PARENT_BATCH_RECEIPT=/tmp/parent.receipt \
  PARENT_BATCH_CLAIM=/tmp/parent.claim.json \
  BATCH_INCLUSION_CHAIN=/tmp/parent.chain.json
```

The current implementation keeps each batch level homogeneous in its direct
leaf kind:

- base batch: `taproot` or `hardened-xpriv`
- parent batch: `batch-claim-v1`
- all child batch claims under one parent must agree on:
  - child batch version and flags
  - child leaf kind
  - child Merkle hash kind
  - child leaf guest image ID

So the verifier receives:

1. one final parent receipt
2. one bundled inclusion-chain JSON artifact
3. one disclosed original leaf journal at the bottom

The lower-level interface still supports repeated `--inclusion-in` flags on
`verify-batch`, but the bundled chain artifact is now the primary nested
verifier path.

### Heterogeneous Parent Batches

The mixed direct-child parent mode is now implemented too. It keeps the same
84-byte batch claim size, but changes the meaning of the 32-byte context slot:

- `batch_version = 1`
  - homogeneous parent
  - `leaf_guest_image_id` pins one shared direct-leaf image ID
- `batch_version = 2`
  - heterogeneous parent
  - the same 32-byte slot becomes a pinned `policy_digest`

Current heterogeneous parent mode uses:

- `BATCH_LEAF_KIND=heterogeneous-envelope-v1`
- one repeated `BATCH_DIRECT_LEAF_KINDS` entry per direct child
- fixed-size 128-byte direct-child envelopes carrying:
  - direct child kind
  - per-child verify image ID
  - exact child journal bytes padded to the current maximum

The supported direct child kinds today are:

- `hardened-xpriv`
- `taproot`
- `batch-claim-v1`

A direct mixed parent over one raw hardened-xpriv leaf and one child batch
claim now looks like:

```bash
make prove-batch GO_GOROOT=/path/to/go1.24.4 \
  BATCH_LEAF_KIND=heterogeneous-envelope-v1 \
  BATCH_LEAF_CLAIMS="./artifacts/hardened-xpriv-succinct.claim.json /tmp/child.claim.json" \
  BATCH_LEAF_RECEIPTS="./artifacts/hardened-xpriv-succinct.receipt /tmp/child.receipt" \
  BATCH_DIRECT_LEAF_KINDS="hardened-xpriv batch-claim-v1" \
  RECEIPT_KIND=succinct \
  BATCH_RECEIPT=/tmp/parent.receipt \
  BATCH_CLAIM=/tmp/parent.claim.json

make derive-batch-inclusion GO_GOROOT=/path/to/go1.24.4 \
  BATCH_LEAF_KIND=heterogeneous-envelope-v1 \
  BATCH_LEAF_CLAIMS="./artifacts/hardened-xpriv-succinct.claim.json /tmp/child.claim.json" \
  BATCH_LEAF_RECEIPTS="./artifacts/hardened-xpriv-succinct.receipt /tmp/child.receipt" \
  BATCH_DIRECT_LEAF_KINDS="hardened-xpriv batch-claim-v1" \
  BATCH_LEAF_INDEX=1 \
  BATCH_INCLUSION_OUT=/tmp/parent.inclusion.json
```

If the disclosed heterogeneous child is itself a `batch-claim-v1`, sparse
verification still becomes a chained proof:

1. verify the top-level heterogeneous parent receipt
2. verify inclusion of the disclosed mixed direct-child envelope
3. decode the embedded child batch claim
4. continue with the next inclusion proof level

### One-Shot Nested Wrapper

The repo now also has a manifest-driven one-shot wrapper:

```bash
make run-nested-batch-plan GO_GOROOT=/path/to/go1.24.4 \
  NESTED_BATCH_PLAN=/tmp/nested-plan.json
```

That manifest is consumed by `run-nested-batch-plan`, which:

1. proves any inline child batches bottom-up
2. proves the final top-level batch
3. optionally derives one bundled inclusion chain from `disclosure_path`
4. optionally verifies the final receipt plus that bundled chain

This wrapper is the ergonomic path. The lower-level subcommands remain the
debugging surface.

Current validated flat-vs-nested comparison on the hardened-xpriv lane:

| N | Final kind | Flat prove | Flat peak RSS | Nested total prove | Nested peak RSS | Flat verifier artifact | Nested verifier artifact |
|---|------------|------------|---------------|--------------------|-----------------|------------------------|--------------------------|
| 8 | composite | 7.27s | 11.21 GiB | 24.79s | 5.75 GiB | 2,043,514 B | 1,139,980 B |
| 8 | succinct | 17.74s | 11.20 GiB | 30.69s | 5.74 GiB | 224,698 B | 225,260 B |
| 16 | composite | 11.24s | 11.25 GiB | 45.25s | 5.75 GiB | 4,073,839 B | 1,140,056 B |
| 16 | succinct | 33.80s | 11.26 GiB | 51.82s | 5.75 GiB | 224,772 B | 225,337 B |

Compatibility notes for verifiers:

- the primary compatibility surface is `claim.json` plus the receipt file
- the current documented receipt encoding is `borsh`
- `proof_seal_bytes` is informative, not a stability guarantee
- image IDs are expected to change when the guest artifact changes, but not
  when only the private witness changes

## Metal Note

On Apple Silicon, guest compilation is still normal CPU work. Metal applies to
the local proving backend, not to TinyGo compilation. The current Go-host prove
path is backed by `go-zkvm/host-ffi`, and the live proof process was confirmed
to map `Metal.framework` plus the Metal Performance Shaders frameworks during
proof generation. Local proving still falls back to CPU if
`RISC0_FORCE_CPU_PROVER=1` is set.

## Current Known-Good Result

Current built-in vector result:

- taproot output key:
  - `00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
- path commitment:
  - `4c7de33d397de2c231e7c2a7f53e5b581ee3c20073ea79ee4afaab56de11f74b`
- journal size:
  - `72` bytes
- latest measured proof seal size:
  - composite: `1797880` bytes
  - succinct: `222668` bytes
- current deterministic image ID:
  - `8a6a2c27dd54d8fa0f99a332b57cb105f88472d977c84bfac077cbe70907a690`
- measured release prove times on this Mac:
  - composite: `49.32s`
  - succinct: `64.30s`
- measured release verify times on this Mac:
  - composite: `0.10s`
  - succinct: `0.03s`
- measured receipt sizes on disk:
  - composite: `1799256` bytes
  - succinct: `223319` bytes
- measured peak RSS:
  - composite: `11.91 GB`
  - succinct: `11.93 GB`

Current image-ID status:

- changing only the `bip32-pq-zkp` checkout path while keeping the same sibling
  `risc0`, `tinygo-zkvm`, and `go-zkvm` trees did not change the image ID
- the older workspace-local `make platform` flow in `risc0/examples/c-guest`
  did change the image ID when the `risc0` checkout path changed
- the new `make platform-standalone` flow produced a matching platform archive,
  guest ELF, and packed guest `.bin` across different `risc0` checkout paths

The deterministic image ID above comes from that standalone-archive flow.

## Remote Proving Note

The current validated lane in this repo is local proving. The host stack uses
the normal risc0 backend selection path, so prover choice still depends on the
surrounding risc0 environment and configuration. Remote proving is therefore
not documented or validated here; see `claim.md` for the privacy implications
of witness handling.
