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

- image ID:
  - `62b563ecceda688696ca9f9e2bb24c4b7e8987647a2d27a960e4d3376bd18082`
- Taproot output key:
  - `00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
- most recent split-layout release prove+verify time on this Mac:
  - `65.24s`

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
