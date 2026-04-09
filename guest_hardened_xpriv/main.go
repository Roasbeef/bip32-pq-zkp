// Package main is the TinyGo guest program for the reduced hardened-xpriv
// proof statement, compiled to a RISC-V ELF that runs inside the risc0 zkVM.
//
// It proves:
//
//	"I know a parent BIP-32 xpriv and chain code such that, after exactly
//	one hardened child derivation step, the resulting child xpriv and
//	chain code are exactly these public bytes."
//
// This is the fastest of the three proof variants because the guest performs
// only HMAC-SHA512 plus scalar addition -- no elliptic curve point
// multiplication. The execution trace is roughly 134k rows / 1 segment,
// compared to ~1.8M rows / 2+ segments for the full Taproot lane.
//
// Wire format on stdin (private witness, never leaves the prover):
//
//	[parent_xpriv:32] [parent_chain_code:32] [child_index:u32_le]
//
// Journal output (public claim, committed to the proof):
//
//	[version:u32_le] [flags:u32_le] [child_xpriv:32] [chain_code:32]
package main

import (
	"github.com/roasbeef/bip32-pq-zkp/bip32"
	"github.com/roasbeef/go-zkvm/zkvm"
)

func main() {
	zkvm.Debug("hardened-xpriv: start\n")

	// Step 1: Read the parent extended private key from the private
	// witness stream. These bytes never appear in the proof output.
	var parentPrivateKey [32]byte
	zkvm.Read(parentPrivateKey[:])

	var parentChainCode [32]byte
	zkvm.Read(parentChainCode[:])

	// Step 2: Read the single hardened child index. The host-side
	// witness builder already validated that this is >= 0x80000000.
	var childIndex uint32
	zkvm.ReadValue(&childIndex)

	// Step 3: Reconstruct the parent ExtendedPrivateKey from the raw
	// bytes. This validates the scalar is in range.
	parent, err := bip32.NewExtendedPrivateKey(
		parentPrivateKey, parentChainCode,
	)
	if err != nil {
		zkvm.Debug("NewExtendedPrivateKey failed\n")
		zkvm.Halt(1)
	}

	// Step 4: Derive exactly one hardened child step. This is the core
	// computation: HMAC-SHA512(parent_chain_code, 0x00 || parent_key ||
	// child_index), then split into child_key (left 32) and
	// child_chain_code (right 32), then child_key += parent_key mod n.
	claim, err := bip32.DeriveHardenedXPrivClaim(
		parent, []uint32{childIndex},
	)
	if err != nil {
		zkvm.Debug("DeriveHardenedXPrivClaim failed\n")
		zkvm.Halt(1)
	}

	// Step 5: Commit the 72-byte public claim to the proof journal.
	// This is the only data the verifier sees.
	zkvm.Debug("hardened-xpriv: derived\n")
	zkvm.Commit(claim.Encode())
	zkvm.Debug("hardened-xpriv: committed\n")
	zkvm.Halt(0)
}
