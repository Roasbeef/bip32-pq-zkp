// Package main is the TinyGo guest program that runs inside the risc0 zkVM.
//
// It proves the following statement without revealing the private witness:
//
//	"I know a BIP-32 seed and derivation path such that, following BIP-32
//	child key derivation and BIP-86 Taproot output-key construction, the
//	result is this specific 32-byte Taproot output key."
//
// The seed and derivation path are private witness data read from guest stdin;
// they never appear in the proof journal or receipt. The guest commits a
// 72-byte public claim containing the Taproot output key, a commitment to
// the derivation path, version, and policy flags.
package main

import (
	"github.com/roasbeef/bip32-pq-zkp/bip32"
	"github.com/roasbeef/go-zkvm/zkvm"
)

const (
	// maxSeedBytes is the maximum BIP-32 seed length supported by the
	// guest. The array is stack-allocated to avoid heap allocation in the
	// TinyGo guest environment.
	maxSeedBytes = 64

	// maxPathDepth is the maximum derivation path depth supported by the
	// guest. BIP-86 paths are 5 elements deep, so 16 provides generous
	// headroom. Stack-allocated for the same reason as maxSeedBytes.
	maxPathDepth = 16

	// flagRequireBIP86 is the witness flags bit that enables BIP-86
	// path-shape enforcement in the guest.
	flagRequireBIP86 = 1
)

func main() {
	zkvm.Debug("bip32: start\n")

	// Read the private witness from host stdin. The wire format is:
	//   [flags:u32_le] [seed_len:u32_le] [seed:bytes]
	//   [path_len:u32_le] [path:u32_le...]
	var flags uint32
	zkvm.ReadValue(&flags)

	// Read and validate the seed length.
	var seedLen uint32
	zkvm.ReadValue(&seedLen)
	if seedLen < 16 || seedLen > maxSeedBytes {
		zkvm.Debug("invalid seed length\n")
		zkvm.Halt(1)
	}

	// Read the raw seed bytes into a stack-allocated buffer.
	var seed [maxSeedBytes]byte
	zkvm.Read(seed[:int(seedLen)])

	// Read and validate the derivation path length.
	var pathLen uint32
	zkvm.ReadValue(&pathLen)
	if pathLen > maxPathDepth {
		zkvm.Debug("invalid path length\n")
		zkvm.Halt(1)
	}

	// Read each path component as a little-endian uint32.
	var path [maxPathDepth]uint32
	for i := uint32(0); i < pathLen; i++ {
		zkvm.ReadValue(&path[i])
	}

	pathSlice := path[:int(pathLen)]

	// Apply the BIP-86 path-shape check if the caller requested it.
	var opts []bip32.TaprootDeriveOption
	if flags&flagRequireBIP86 != 0 {
		opts = append(opts, bip32.WithBIP86PathVerification())
	}

	// This is the core proof computation: derive the BIP-32 child key,
	// compute the BIP-86 Taproot output key, and build the public claim.
	claim, err := bip32.DeriveTaprootClaim(
		seed[:int(seedLen)], pathSlice, opts...,
	)
	if err != nil {
		zkvm.Debug("DeriveTaprootClaim failed\n")
		zkvm.Halt(1)
	}

	// Commit the 72-byte public claim to the proof journal. This is the
	// only data that becomes visible to the verifier.
	zkvm.Debug("bip32: derived\n")
	zkvm.Commit(claim.Encode())
	zkvm.Debug("bip32: committed\n")
	zkvm.Halt(0)
}
