# bip32-pq-zkp Docs

## Recommended Reading Order

1. `../README.md`
   - repo purpose, current state, and layout
2. `claim.md`
   - exact public claim, private witness, and optional policy knobs
3. `running.md`
   - build, execute, prove, and verify the current demo
4. `../progress.md`
   - full working log and investigation history

## Core Claim

The public output is only the final Taproot output key.

The private witness is:

- the seed
- the derivation path

Optional policy:

- require the path to satisfy the BIP-86 shape
- or leave the path unrestricted and prove only the general BIP-32 to Taproot
  derivation claim
