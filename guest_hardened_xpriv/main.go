// Package main is the TinyGo guest program for the reduced hardened-xpriv
// statement.
//
// It proves:
//
//	"I know a parent BIP-32 xpriv and chain code such that, after exactly
//	one hardened child derivation step, the resulting child xpriv and
//	chain code are exactly these public bytes."
package main

import (
	"github.com/roasbeef/bip32-pq-zkp/bip32"
	"github.com/roasbeef/go-zkvm/zkvm"
)

func main() {
	zkvm.Debug("hardened-xpriv: start\n")

	var parentPrivateKey [32]byte
	zkvm.Read(parentPrivateKey[:])

	var parentChainCode [32]byte
	zkvm.Read(parentChainCode[:])

	var childIndex uint32
	zkvm.ReadValue(&childIndex)

	parent, err := bip32.NewExtendedPrivateKey(
		parentPrivateKey, parentChainCode,
	)
	if err != nil {
		zkvm.Debug("NewExtendedPrivateKey failed\n")
		zkvm.Halt(1)
	}

	claim, err := bip32.DeriveHardenedXPrivClaim(
		parent, []uint32{childIndex},
	)
	if err != nil {
		zkvm.Debug("DeriveHardenedXPrivClaim failed\n")
		zkvm.Halt(1)
	}

	zkvm.Debug("hardened-xpriv: derived\n")
	zkvm.Commit(claim.Encode())
	zkvm.Debug("hardened-xpriv: committed\n")
	zkvm.Halt(0)
}
