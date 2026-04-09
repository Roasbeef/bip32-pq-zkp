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
