/*
Package bip32pqzkp contains the demo-specific host-side logic for the
`bip32-pq-zkp` repository.

The sibling `github.com/roasbeef/go-zkvm/host` package provides the generic
guest execution, proving, and verification boundary. This package sits one
level above that boundary: it knows how to build private BIP-32 witnesses,
decode public claim journals, write demo receipt and claim artifacts, and
enforce the optional BIP-86 policy bit.

The package supports three proof lanes:

  - Full Taproot: proves seed + full path to BIP-86 Taproot output key
    (72-byte claim: version, flags, output key, path commitment).
  - Hardened xpub: proves parent xpriv to child compressed pubkey via
    hardened-only derivation (73-byte claim: version, flags, compressed
    pubkey, chain code).
  - Hardened xpriv: proves a single hardened CKDpriv step from parent
    xpriv to child xpriv (72-byte claim: version, flags, child private
    key, chain code). This is the fastest variant (~2s prove time) since
    the guest avoids EC point multiplication entirely.

Each lane has its own Runner methods (Execute, Prove, Verify), witness
builder, claim decoder, and claim-file I/O helpers.

Keeping that logic here lets `cmd/bip32-pq-zkp-host` stay small while still
giving the repo a normal Go package that can be imported, tested, and read
through `go doc`.
*/
package bip32pqzkp
