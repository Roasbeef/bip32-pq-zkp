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
- observed image IDs for that same public output:
  - sibling-layout build: `62b563ecceda688696ca9f9e2bb24c4b7e8987647a2d27a960e4d3376bd18082`
  - fresh-clone build: `61a39aca30f96db015a56ea08b6fba8f0cfd43eca4d148c50afa1de60ecb26de`
- observed release prove+verify times on this Mac:
  - latest sibling-layout rerun: `54.88s`
  - sibling-layout run: `65.24s`
  - fresh-clone run: `85.65s`

The output key is stable across both builds. The image ID is currently tied to
the exact built artifact: the guest ELF still embeds absolute build paths from
the linked `zkvm-platform` archive, so rebuilding the same source tree in a
different directory changes the image ID.

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
