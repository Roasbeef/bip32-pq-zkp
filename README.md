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
- upstream `libzkvm_platform.a`
- private witness input from the Rust host
- guest commits only the final Taproot output key

Current known-good vector result:

- Taproot output key:
  - `00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
- latest measured proof seal size on this Mac:
  - `1797880` bytes
- current sibling-layout image ID:
  - `e9177de911f48092749d50e17368e83a26207b016c3fe95a2efc49135e45c4eb`
- observed release prove+verify times on this Mac:
  - latest sibling-layout rerun: `54.88s`
  - sibling-layout run: `65.24s`
  - fresh-clone run: `85.65s`

Current reproducibility caveat:

- moving only the `bip32-pq-zkp` checkout to a different directory while
  reusing the same sibling `risc0`, `tinygo-zkvm`, and `go-zkvm` trees kept the
  image ID stable
- rebuilding the linked `libzkvm_platform.a` from a different `risc0` checkout
  path changed the image ID while keeping the public Taproot output key the same

So the current instability is specifically tied to the linked platform-archive
build, not just the demo repo checkout path.

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
