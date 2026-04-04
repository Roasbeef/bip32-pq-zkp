# Running bip32-pq-zkp

This repo currently uses the reusable host harness from the sibling
`go-zkvm` repo.

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

If your default `go` is newer than the TinyGo lane supports, export:

```bash
export GO_GOROOT=/path/to/go1.24.4
```

## Current Flow

1. build the TinyGo fork
2. build the risc0 platform archive
3. build the `bip32` guest with TinyGo target `zkvm-platform`
4. package it with `v1compat.elf`
5. run host-side reference tests
6. execute or prove it with the Rust host harness from `go-zkvm`

## Reference Check

The host-side reference test compares the local helper code against
`btcd/txscript.ComputeTaprootKeyNoScript`.

Run:

```bash
go test ./hostcheck -v
```

## Execute-Only Run

From `../go-zkvm/go-guest-host`:

```bash
cargo run --release -- ../bip32-pq-zkp/bip32-platform-latest.bin \
  --raw-journal \
  --execute-only \
  --use-test-vector
```

## Prove And Verify

From `../go-zkvm/go-guest-host`:

```bash
cargo run --release -- ../bip32-pq-zkp/bip32-platform-latest.bin \
  --raw-journal \
  --use-test-vector \
  --require-bip86
```

## Current Known-Good Result

Current built-in vector result:

- journal/output key:
  - `00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
- latest measured proof seal size:
  - `1797880` bytes
- observed image IDs:
  - sibling-layout build: `62b563ecceda688696ca9f9e2bb24c4b7e8987647a2d27a960e4d3376bd18082`
  - fresh-clone build: `61a39aca30f96db015a56ea08b6fba8f0cfd43eca4d148c50afa1de60ecb26de`
- measured release prove+verify times on this Mac:
  - latest sibling-layout rerun: `54.88s`
  - sibling-layout run: `65.24s`
  - fresh-clone run: `85.65s`

The output key is stable across both runs. The image ID currently depends on
the exact built artifact because absolute build paths from `zkvm-platform` are
embedded into the guest ELF.

## Remote Proving Note

The current verified lane is local proving.

Boundless is worth revisiting later as a remote/offload option:

- `https://github.com/boundless-xyz/boundless`

But the local/private path is the current source of truth.
