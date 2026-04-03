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

- image ID:
  - `62b563ecceda688696ca9f9e2bb24c4b7e8987647a2d27a960e4d3376bd18082`
- journal/output key:
  - `00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
- measured split-layout release prove+verify time on this Mac:
  - `65.24s`

## Remote Proving Note

The current verified lane is local proving.

Boundless is worth revisiting later as a remote/offload option:

- `https://github.com/boundless-xyz/boundless`

But the local/private path is the current source of truth.
