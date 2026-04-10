// Package batchclaim defines the fixed-size public claim and Merkle tree
// structures used by the recursive batch aggregation guest. This package
// is imported by both the TinyGo guest (inside the zkVM) and the host-side
// Go code, so it must remain compatible with TinyGo's minimal runtime.
//
// The batch claim design keeps the final journal fixed-size (84 bytes)
// regardless of how many leaves are aggregated. The fan-out is captured
// by the Merkle root, not by enumerating leaves in the journal. This
// ensures that a succinct batch receipt stays near the ~222 KB floor
// even as N grows, while total prover work scales linearly with N.
package batchclaim

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

const (
	// Version is the current serialized batch-claim format version.
	Version = 1

	// VersionHeterogeneousParent is the serialized batch-claim version used
	// when the direct children are heterogeneous envelopes rather than one
	// homogeneous raw journal kind.
	VersionHeterogeneousParent = 2

	// FlagsNone is the zero-value batch policy bitfield for the
	// first version.
	FlagsNone = 0

	// MerkleHashSHA256 identifies the SHA-256 Merkle-tree construction.
	// SHA-256 was chosen over Poseidon2 for the first design because it
	// is easier for external verifiers and aligns with Bitcoin-adjacent
	// tooling. The merkle_hash_kind field in the claim makes the hash
	// choice explicit so it can be upgraded later.
	MerkleHashSHA256 = 1

	// LeafKindTaproot identifies the original full Taproot claim journal
	// (72 bytes: version, flags, taproot output key, path commitment).
	LeafKindTaproot = 1

	// LeafKindHardenedXPriv identifies the reduced hardened-xpriv claim
	// journal (72 bytes: version, flags, child private key, chain code).
	LeafKindHardenedXPriv = 2

	// LeafKindBatchClaimV1 identifies one serialized v1 batch claim used as
	// a leaf inside a higher-level parent batch.
	LeafKindBatchClaimV1 = 3

	// LeafKindHeterogeneousEnvelopeV1 identifies one fixed-size
	// direct-child
	// envelope that can mix raw leaves and nested batch claims
	// within the same parent batch.
	LeafKindHeterogeneousEnvelopeV1 = 4

	// PublicClaimSize is the serialized size of Claim in bytes:
	// 4 (version) + 4 (flags) + 4 (leaf kind) + 4 (merkle hash kind) +
	// 4 (leaf count) + 32 (leaf guest image ID) + 32 (merkle root) = 84.
	PublicClaimSize = 84
)

// Claim is the fixed-size public batch claim committed by the aggregation
// guest. It pins the leaf schema, the leaf guest image, and the Merkle root
// over the ordered set of verified leaf journals. This is the only data the
// verifier sees from the batch receipt; individual leaf details require a
// separate Merkle inclusion proof.
type Claim struct {
	// Version identifies the serialized claim format.
	Version uint32

	// Flags carries batch-level verifier-visible policy bits.
	Flags uint32

	// LeafClaimKind identifies the schema used by every leaf in the batch.
	LeafClaimKind uint32

	// MerkleHashKind identifies the Merkle-tree hash construction.
	MerkleHashKind uint32

	// LeafCount is the number of leaves committed under the Merkle root.
	LeafCount uint32

	// LeafGuestImageID is the common leaf guest image ID pinned for
	// homogeneous v1 batches. For heterogeneous parent batches
	// (`VersionHeterogeneousParent` +
	// `LeafKindHeterogeneousEnvelopeV1`) the same 32-byte slot
	// carries the pinned policy digest instead.
	LeafGuestImageID [32]byte

	// MerkleRoot commits to the ordered batch leaf set.
	MerkleRoot [32]byte
}

// Encode serializes the batch claim into the fixed journal layout.
func (c Claim) Encode() [PublicClaimSize]byte {
	var out [PublicClaimSize]byte
	binary.LittleEndian.PutUint32(out[0:4], c.Version)
	binary.LittleEndian.PutUint32(out[4:8], c.Flags)
	binary.LittleEndian.PutUint32(out[8:12], c.LeafClaimKind)
	binary.LittleEndian.PutUint32(out[12:16], c.MerkleHashKind)
	binary.LittleEndian.PutUint32(out[16:20], c.LeafCount)
	copy(out[20:52], c.LeafGuestImageID[:])
	copy(out[52:84], c.MerkleRoot[:])
	return out
}

// Decode parses one fixed-size batch claim journal.
func Decode(journal []byte) (Claim, error) {
	if len(journal) != PublicClaimSize {
		return Claim{}, fmt.Errorf(
			"unexpected batch claim size: got %d bytes, want %d",
			len(journal), PublicClaimSize,
		)
	}

	var claim Claim
	claim.Version = binary.LittleEndian.Uint32(journal[0:4])
	claim.Flags = binary.LittleEndian.Uint32(journal[4:8])
	claim.LeafClaimKind = binary.LittleEndian.Uint32(journal[8:12])
	claim.MerkleHashKind = binary.LittleEndian.Uint32(journal[12:16])
	claim.LeafCount = binary.LittleEndian.Uint32(journal[16:20])
	copy(claim.LeafGuestImageID[:], journal[20:52])
	copy(claim.MerkleRoot[:], journal[52:84])
	return claim, nil
}

// ValidateVersion reports whether the claim version matches the expected
// schema for its direct leaf mode.
func (c Claim) ValidateVersion() error {
	expected := uint32(Version)
	if c.UsesPolicyDigest() {
		expected = uint32(VersionHeterogeneousParent)
	}
	if c.Version != expected {
		return fmt.Errorf(
			"unexpected batch claim version: got %d, want %d",
			c.Version,
			expected,
		)
	}

	return nil
}

// LeafKindName returns a readable label for the leaf claim kind.
func LeafKindName(kind uint32) string {
	switch kind {
	case LeafKindTaproot:
		return "taproot"
	case LeafKindHardenedXPriv:
		return "hardened_xpriv"
	case LeafKindBatchClaimV1:
		return "batch_claim_v1"

	case LeafKindHeterogeneousEnvelopeV1:
		return "heterogeneous_envelope_v1"
	default:
		return "unknown"
	}
}

// LeafClaimSize returns the fixed journal size for one supported leaf kind.
func LeafClaimSize(kind uint32) (int, bool) {
	switch kind {
	case LeafKindTaproot, LeafKindHardenedXPriv:
		return 72, true

	case LeafKindBatchClaimV1:
		return PublicClaimSize, true

	case LeafKindHeterogeneousEnvelopeV1:
		return HeterogeneousEnvelopeSizeV1, true

	default:
		return 0, false
	}
}

// MerkleHashName returns a readable label for the Merkle hash kind.
func MerkleHashName(kind uint32) string {
	switch kind {
	case MerkleHashSHA256:
		return "sha256"
	default:
		return "unknown"
	}
}

// UsesPolicyDigest reports whether the 32-byte context field in the claim is
// interpreted as a policy digest rather than one shared direct-leaf image ID.
func (c Claim) UsesPolicyDigest() bool {
	return c.Version == VersionHeterogeneousParent &&
		c.LeafClaimKind == LeafKindHeterogeneousEnvelopeV1
}

// HeterogeneousPolicyDigestV1 returns the pinned policy digest for the first
// mixed-direct-child parent mode. The digest commits to the envelope schema,
// envelope size, maximum embedded journal size, and the allowed direct child
// kinds so the parent claim has a stable verifier-visible policy anchor.
func HeterogeneousPolicyDigestV1() [32]byte {
	data := make([]byte, 0, 64)
	data = append(data, "bip32-pq-zkp:heterogeneous-parent-policy:v1"...)
	for _, word := range []uint32{
		HeterogeneousEnvelopeVersionV1,
		HeterogeneousEnvelopeSizeV1,
		HeterogeneousEnvelopeMaxJournalSizeV1,
		LeafKindTaproot,
		LeafKindHardenedXPriv,
		LeafKindBatchClaimV1,
	} {
		data = binary.LittleEndian.AppendUint32(data, word)
	}

	return sha256.Sum256(data)
}
