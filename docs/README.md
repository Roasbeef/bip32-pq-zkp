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
5. `recursive-proving.md`
   - accessible guide to how recursive composition works: the two-sided
     dependency model, what `zkvm.Verify` actually does, the assumptions
     digest chain, and the risc0 recursion pipeline (lift/join/resolve)
6. `batch-merkle-system.md`
   - detailed guide to the batch Merkle tree, claim schemas (Taproot,
     hardened-xpriv, batch-claim-v1, heterogeneous-envelope-v1), domain
     separation, sparse verification flows, and verifier artifact sets
7. `batch-aggregation.md`
   - how the current batch lane works, including homogeneous batches,
     heterogeneous parents, the batch claim schema, verifier flow, and
     scaling results
8. `recursion-batch-walkthrough.md`
   - reference-style walkthrough of how `batch_runner`, `guest_batch`,
     `zkvm.Verify(...)`, and the host-side assumption receipts fit together
9. `nested-batching.md`
   - the implemented first hierarchical batch-of-batches extension,
     bundled inclusion-chain verifier path, enforced child-batch
     homogeneity, and current limitations
10. `mmr-accumulator-sketch.md`
    - shorter sketch of the append-only / flat-root-preserving accumulator
      direction if we later need stronger incremental semantics
11. `batch-future-work.md`
    - broader future directions beyond the implemented nested and
      heterogeneous-parent layers: accumulator alternatives, larger studies,
      and multi-UTXO authorization
12. `heterogeneous-parent-plan.md`
    - design note that led to the current mixed direct parent mode for
      `{batch_claim_v1, raw_leaf_1, raw_leaf_2}`
13. `nested-wrapper-plan.md`
    - design note behind the current one-shot nested-batch orchestration
      command
14. `code-format.md`
    - the commenting, stanza, and readability rules used for the Go code in
      this repo
15. `../progress.md`
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
