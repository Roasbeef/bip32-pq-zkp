# Running bip32-pq-zkp

This repo now carries its own demo-specific Go host command in
`cmd/bip32-pq-zkp-host/`. The sibling `go-zkvm` repo still provides the guest
packaging and proving engine boundary, but `bip32-pq-zkp` owns the
demo-specific `execute`, `prove`, and `verify` UX.

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
lowercase aliases you have been using interactively:

- `PRIV_SEED_HEX` or `priv_seed`
- `BIP32_PATH` or `bip_32_path`
- `PUBKEY` or `pubkey`
- `PATH_COMMITMENT` or `path_commitment`
- `REQUIRE_BIP86`
- `RECEIPT`
- `CLAIM`

`BIP32_PATH` accepts either slash form or comma form:

```text
m/86'/0'/0'/0/0
86',0',0',0,0
```

Default behavior:

- bare `make execute` or `make prove` uses the built-in BIP-32 test vector
- bare `make verify` expects the default receipt and claim artifacts from a
  prior `make prove`
- bare `make verify` also checks the default `require_bip86=true` policy bit
- if you set `PRIV_SEED_HEX`, you should also set `BIP32_PATH`

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

## Prove

Built-in test vector:

```bash
make prove GO_GOROOT=/path/to/go1.24.4
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
- readable claim metadata: `./artifacts/bip32-test-vector.claim.json`

## Verify

Verify against the emitted claim artifact:

```bash
make verify GO_GOROOT=/path/to/go1.24.4
```

Verify against both the claim artifact and explicit expected public material:

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
  - `1797880` bytes
- current deterministic image ID:
  - `8a6a2c27dd54d8fa0f99a332b57cb105f88472d977c84bfac077cbe70907a690`
- measured release prove times on this Mac:
  - split `make prove` with explicit `PRIV_SEED_HEX` / `BIP32_PATH`: `54.76s`
  - deterministic standalone-archive run: `51.51s`
  - clean-room deterministic rerun: `58.93s`
  - earlier sibling-layout rerun: `54.88s`
  - earlier fresh-clone run: `85.65s`

Current image-ID status:

- changing only the `bip32-pq-zkp` checkout path while keeping the same sibling
  `risc0`, `tinygo-zkvm`, and `go-zkvm` trees did not change the image ID
- the older workspace-local `make platform` flow in `risc0/examples/c-guest`
  did change the image ID when the `risc0` checkout path changed
- the new `make platform-standalone` flow produced a matching platform archive,
  guest ELF, and packed guest `.bin` across different `risc0` checkout paths

The deterministic image ID above comes from that standalone-archive flow.

## Remote Proving Note

The current verified lane is local proving.

Boundless is worth revisiting later as a remote/offload option:

- `https://github.com/boundless-xyz/boundless`

But the local/private path is the current source of truth.
