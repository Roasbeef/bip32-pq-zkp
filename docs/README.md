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
4. `code-format.md`
   - the commenting, stanza, and readability rules used for the Go code in
     this repo
5. `../progress.md`
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
