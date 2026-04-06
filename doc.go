/*
Package bip32pqzkp contains the demo-specific host-side logic for the
`bip32-pq-zkp` repository.

The sibling `github.com/roasbeef/go-zkvm/host` package provides the generic
guest execution, proving, and verification boundary. This package sits one
level above that boundary: it knows how to build the private BIP-32 witness,
decode the 72-byte public claim journal, write the demo receipt and claim
artifacts, and enforce the optional BIP-86 policy bit.

Keeping that logic here lets `cmd/bip32-pq-zkp-host` stay small while still
giving the repo a normal Go package that can be imported, tested, and read
through `go doc`.
*/
package bip32pqzkp
