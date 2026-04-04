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
- current sibling-layout image ID:
  - `e9177de911f48092749d50e17368e83a26207b016c3fe95a2efc49135e45c4eb`
- measured release prove+verify times on this Mac:
  - latest sibling-layout rerun: `54.88s`
  - sibling-layout run: `65.24s`
  - fresh-clone run: `85.65s`

The output key is stable across all of the checked runs.

Current image-ID caveat:

- changing only the `bip32-pq-zkp` checkout path while keeping the same sibling
  `risc0`, `tinygo-zkvm`, and `go-zkvm` trees did not change the image ID
- rebuilding the linked `libzkvm_platform.a` from a different `risc0` checkout
  path did change the image ID while preserving the same public output

So the remaining instability is specifically in the linked platform-archive
build, not just the demo checkout path.

## Remote Proving Note

The current verified lane is local proving.

Boundless is worth revisiting later as a remote/offload option:

- `https://github.com/boundless-xyz/boundless`

But the local/private path is the current source of truth.
