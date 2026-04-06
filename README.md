# bip32-pq-zkp

`bip32-pq-zkp` is the end-to-end demo repo for proving, inside risc0, that a
public Taproot output key was derived from private BIP-32 witness material.

The witness is:

- the seed
- the derivation path

The verifier-facing public claim is:

- the final 32-byte Taproot output key
- a 32-byte commitment to the private derivation path
- claim version and policy flags

The canonical verifier artifact set is:

- a binary receipt file
- a human-readable `claim.json` file that names the public claim fields and the
  image ID they were proven against

The core public target is still the Taproot output key. The extra public fields
let the verifier bind the proof to a path commitment and an optional BIP-86
policy flag without revealing the private witness itself.

The intended default verifier flow is:

1. load the receipt
2. load `claim.json`
3. compute or pin the expected image ID for the exact guest artifact
4. verify the receipt against that image ID
5. compare the verified public journal output to `claim.json`

Direct `PUBKEY`, `PATH_COMMITMENT`, or `BIP32_PATH` checks are still supported,
but they are the advanced/manual path rather than the canonical one.

## Current Status

The current local lane works end-to-end on the updated stack:

- latest-upstream risc0 lane
- TinyGo `v0.40.1` fork with zkVM support
- deterministic `libzkvm_platform.a` from `risc0/examples/c-guest make platform-standalone`
- private witness input from the demo-specific Go host command backed by
  `go-zkvm/host`
- guest commits a structured 72-byte public claim
- local proving on Apple Silicon uses the Metal-enabled risc0 prover build by
  default unless `RISC0_FORCE_CPU_PROVER=1` is set

Current known-good vector result:

- Taproot output key:
  - `00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
- Path commitment:
  - `4c7de33d397de2c231e7c2a7f53e5b581ee3c20073ea79ee4afaab56de11f74b`
- Claim journal size:
  - `72` bytes
- latest measured proof seal size on this Mac:
  - `1797880` bytes
- current deterministic image ID:
  - `8a6a2c27dd54d8fa0f99a332b57cb105f88472d977c84bfac077cbe70907a690`
- observed release prove+verify times on this Mac:
  - split `make prove` run with explicit `PRIV_SEED_HEX` / `BIP32_PATH`:
    `54.38s`
  - deterministic standalone-archive run: `51.51s`
  - clean-room deterministic rerun: `58.93s`
  - earlier sibling-layout rerun: `54.88s`
  - earlier fresh-clone run: `85.65s`

Current reproducibility status:

- moving only the `bip32-pq-zkp` checkout to a different directory while
  reusing the same sibling `risc0`, `tinygo-zkvm`, and `go-zkvm` trees kept the
  image ID stable
- the older workspace-local `make platform` flow in `risc0/examples/c-guest`
  was the remaining source of checkout-path image-ID drift
- the published `make platform-standalone` path now removes that instability:
  the standalone-built archive, guest ELF, and packed guest `.bin` were all
  reproduced byte-for-byte across different `risc0` checkout directories

Strict BIP-86 path-shape checking is implemented as optional policy, not a hard
requirement for every proof.

## Scope

This repo is the concrete demo layer:

- minimal BIP-32 derivation helpers
- BIP-86 Taproot output-key derivation helpers
- TinyGo guest logic for the final proof claim
- host-side reference tests against btcd/txscript
- docs for the claim, policy options, and runbook
- the running project log in `progress.md`

The reusable guest/host plumbing lives in the sibling `go-zkvm` repo.

Fresh-clone setup notes:

- in sibling `tinygo-zkvm`, run `git submodule update --init --recursive`
- in sibling `risc0`, run `git lfs pull` before the local prover build
- `make execute`, `make prove`, and `make verify` automatically build the
  sibling `go-zkvm` `host-ffi` shared library if it is missing or stale

## Layout

- `bip32/`
  - minimal derivation helpers used by the guest and host-side tests
- `guest/`
  - TinyGo guest for the end-to-end proof
- `cmd/bip32-pq-zkp-host/`
  - demo-specific Go host command for `execute`, `prove`, and `verify`
  - built on top of `github.com/roasbeef/go-zkvm/host`
- `hostcheck/`
  - host-side correctness tests against btcd/txscript
- `docs/`
  - claim statement and runbooks
- `progress.md`
  - prototype log and major findings

## Documentation

Start with:

- `docs/README.md`
- `docs/claim.md`
- `docs/running.md`
- `progress.md`
