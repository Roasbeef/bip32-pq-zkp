# bip32-pq-zkp Docs

## Recommended Reading Order

Start with `../doc.go` for the package-level overview, then:

1. `../README.md`
   - repo purpose, canonical artifacts, and quick-start flow
2. `claim.md`
   - exact public claim, private witness, compatibility contract, and policy
3. `running.md`
   - build, execute, prove, and verify the current demo through the Go host
     CLI
4. `reduced-variants.md`
   - side-by-side comparison of the full Taproot proof and the two reduced
     hardened-derivation variants
5. `batch-aggregation.md`
   - how the v1 batch lane works, batch claim schema, Merkle tree
     construction, verifier flow, and scaling results
6. `nested-batching.md`
   - the implemented first hierarchical batch-of-batches extension,
     bundled inclusion-chain verifier path, enforced child-batch
     homogeneity, and current limitations
7. `mmr-accumulator-sketch.md`
   - shorter sketch of the append-only / flat-root-preserving accumulator
     direction if we later need stronger incremental semantics
8. `batch-future-work.md`
   - broader future directions beyond the implemented nested layer:
     heterogeneous parent leaves, accumulator alternatives, and multi-UTXO
     authorization
9. `code-format.md`
   - the commenting, stanza, and readability rules used for the Go code in
     this repo
10. `../progress.md`
   - repo-local working log and major findings

## Core Claim

The verifier-facing public claim is:

- final Taproot output key
- path commitment
- policy/version flags

The private witness is:

- the seed
- the derivation path

Optional policy:

- require the path to satisfy the BIP-86 shape
- or leave the path unrestricted and prove only the general BIP-32 to Taproot
  derivation claim

## Canonical Verifier Path

The default verifier UX for this repo is:

- receipt file
- `claim.json`

Direct verification against explicit `PUBKEY`, `PATH_COMMITMENT`, or
`BIP32_PATH` expectations is still supported, but it is the advanced/manual
path rather than the primary contract.
