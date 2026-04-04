# bip32-pq-zkp

`bip32-pq-zkp` is the end-to-end demo repo for proving, inside risc0, that a
public Taproot output key was derived from private BIP-32 witness material.

The witness is:

- the seed
- the derivation path

The public output is:

- the final 32-byte Taproot output key only

This keeps the proof aligned with the actual claim we care about: knowledge of
private seed/path material that derives to a specific Taproot key, without
revealing that witness.

## Current Status

The current local lane works end-to-end on the updated stack:

- latest-upstream risc0 lane
- TinyGo `v0.40.1` fork with zkVM support
- deterministic `libzkvm_platform.a` from `risc0/examples/c-guest make platform-standalone`
- private witness input from the Rust host
- guest commits only the final Taproot output key

Current known-good vector result:

- Taproot output key:
  - `00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
- latest measured proof seal size on this Mac:
  - `1797880` bytes
- current deterministic image ID:
  - `b154913927df91257436ddb91567d46a28018c03bfb3848c3d7d7a774e840a79`
- observed release prove+verify times on this Mac:
  - deterministic standalone-archive run: `51.51s`
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

## Layout

- `bip32/`
  - minimal derivation helpers used by the guest and host-side tests
- `guest/`
  - TinyGo guest for the end-to-end proof
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
