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

Fresh-clone setup notes:

- in `../tinygo-zkvm`, run `git submodule update --init --recursive`
- in `../risc0`, run `git lfs pull` before building the Rust host/prover path

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
6. execute or prove it with the Rust host harness from `go-zkvm`

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
- current deterministic image ID:
  - `b154913927df91257436ddb91567d46a28018c03bfb3848c3d7d7a774e840a79`
- measured release prove+verify times on this Mac:
  - deterministic standalone-archive run: `51.51s`
  - clean-room deterministic rerun: `58.93s`
  - earlier sibling-layout rerun: `54.88s`
  - earlier fresh-clone run: `85.65s`

The output key is stable across all of the checked runs.

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
