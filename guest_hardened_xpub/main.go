// Package main is the TinyGo guest program for the reduced hardened-xpub
// proof statement, compiled to a RISC-V ELF that runs inside the risc0 zkVM.
//
// It proves:
//
//	"I know a parent BIP-32 xpriv and chain code such that, after one or
//	more hardened child derivation steps, the resulting child xpub and
//	chain code are exactly these public bytes."
//
// This is the middle-ground variant: it still requires one EC point
// multiplication (private key -> public key) but avoids the full Taproot
// tweak. The execution trace is roughly 1.8M rows / 2 segments. The
// resulting compressed public key is 33 bytes (SEC format with parity).
//
// Wire format on stdin (private witness, never leaves the prover):
//
//	[parent_xpriv:32] [parent_chain_code:32]
//	[path_len:u32_le] [path_component:u32_le...]
//
// Journal output (public claim, committed to the proof):
//
//	[version:u32_le] [flags:u32_le] [compressed_pubkey:33] [chain_code:32]
package main

import (
	"github.com/roasbeef/bip32-pq-zkp/bip32"
	"github.com/roasbeef/go-zkvm/zkvm"
)

const (
	// maxHardenedPathDepth caps the hardened path length to prevent
	// unbounded stack allocation inside the guest. 16 levels is generous
	// headroom for any practical BIP-32 derivation.
	//
	// NOTE: The path array is stack-allocated (fixed-size [16]uint32) to
	// avoid heap allocation in TinyGo's minimal runtime. This is the same
	// pattern used by the full Taproot guest.
	maxHardenedPathDepth = 16
)

func main() {
	zkvm.Debug("hardened-xpub: start\n")

	// Step 1: Read the parent extended private key from the private
	// witness stream.
	var parentPrivateKey [32]byte
	zkvm.Read(parentPrivateKey[:])

	var parentChainCode [32]byte
	zkvm.Read(parentChainCode[:])

	// Step 2: Read the variable-length hardened path. The length prefix
	// is validated against the stack-allocated maximum.
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

	// Step 3: Reconstruct the parent ExtendedPrivateKey.
	parent, err := bip32.NewExtendedPrivateKey(
		parentPrivateKey, parentChainCode,
	)
	if err != nil {
		zkvm.Debug("NewExtendedPrivateKey failed\n")
		zkvm.Halt(1)
	}

	// Step 4: Derive the hardened child path and compute the claim.
	// DeriveHardenedXPubClaim validates all indices are hardened, walks
	// the derivation chain, then performs the EC point multiplication
	// to produce the compressed public key.
	claim, err := bip32.DeriveHardenedXPubClaim(
		parent, path[:int(pathLen)],
	)
	if err != nil {
		zkvm.Debug("DeriveHardenedXPubClaim failed\n")
		zkvm.Halt(1)
	}

	// Step 5: Commit the 73-byte public claim to the proof journal.
	zkvm.Debug("hardened-xpub: derived\n")
	zkvm.Commit(claim.Encode())
	zkvm.Debug("hardened-xpub: committed\n")
	zkvm.Halt(0)
}
