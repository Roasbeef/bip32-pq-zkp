# bip32-pq-zkp Docs

## Recommended Reading Order

1. `../README.md`
   - repo purpose, current state, and layout
2. `claim.md`
   - exact public claim, private witness, and optional policy knobs
3. `running.md`
   - build, execute, prove, and verify the current demo through the Go host
     wrapper
4. `code-format.md`
   - the commenting, stanza, and readability rules used for the Go code in
     this repo
5. `../progress.md`
   - full working log and investigation history

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
