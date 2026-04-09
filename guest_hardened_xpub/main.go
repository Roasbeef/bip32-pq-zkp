// Package main is the TinyGo guest program for the reduced hardened-xpub
// statement.
//
// It proves:
//
//	"I know a parent BIP-32 xpriv and chain code such that, after one or
//	more hardened child derivation steps, the resulting child xpub and
//	chain code are exactly these public bytes."
package main

import (
	"github.com/roasbeef/bip32-pq-zkp/bip32"
	"github.com/roasbeef/go-zkvm/zkvm"
)

const (
	// maxHardenedPathDepth is generous headroom for the reduced hardened path.
	maxHardenedPathDepth = 16
)

func main() {
	zkvm.Debug("hardened-xpub: start\n")

	var parentPrivateKey [32]byte
	zkvm.Read(parentPrivateKey[:])

	var parentChainCode [32]byte
	zkvm.Read(parentChainCode[:])

	var pathLen uint32
	zkvm.ReadValue(&pathLen)
	if pathLen > maxHardenedPathDepth {
		zkvm.Debug("invalid hardened path length\n")
		zkvm.Halt(1)
	}

	var path [maxHardenedPathDepth]uint32
	for i := uint32(0); i < pathLen; i++ {
		zkvm.ReadValue(&path[i])
	}

	parent, err := bip32.NewExtendedPrivateKey(
		parentPrivateKey, parentChainCode,
	)
	if err != nil {
		zkvm.Debug("NewExtendedPrivateKey failed\n")
		zkvm.Halt(1)
	}

	claim, err := bip32.DeriveHardenedXPubClaim(
		parent, path[:int(pathLen)],
	)
	if err != nil {
		zkvm.Debug("DeriveHardenedXPubClaim failed\n")
		zkvm.Halt(1)
	}

	zkvm.Debug("hardened-xpub: derived\n")
	zkvm.Commit(claim.Encode())
	zkvm.Debug("hardened-xpub: committed\n")
	zkvm.Halt(0)
}
