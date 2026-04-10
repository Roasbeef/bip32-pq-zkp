// Package main is the generic TinyGo batch aggregation guest, compiled to a
// RISC-V ELF that runs inside the risc0 zkVM. This is the recursive
// composition entry point: it consumes N existing succinct leaf receipts as
// assumptions, verifies each one against a pinned leaf guest image ID,
// hashes the ordered leaf journals into a Merkle root, and commits only
// a fixed-size 84-byte batch claim to the journal.
//
// The critical architectural property is that the batch guest never sees
// the leaf private witnesses. It only sees the leaf public journals (which
// arrive via stdin) and the leaf receipts (which arrive as host-provided
// assumptions resolved by the risc0 recursion pipeline). This means the
// final batch receipt proves: "there exist N valid leaf receipts, all from
// the same guest image, whose ordered journals hash to this Merkle root."
//
// Wire format on stdin (private witness):
//
//	[leaf_claim_kind:u32_le] [merkle_hash_kind:u32_le]
//	[leaf_context_digest:32] [leaf_count:u32_le]
//	[leaf_record_0:*] [leaf_record_1:*] ... [leaf_record_N-1:*]
//
// Journal output (public claim, committed to the proof):
//
//	[version:u32_le] [flags:u32_le] [leaf_kind:u32_le]
//	[merkle_hash_kind:u32_le] [leaf_count:u32_le]
//	[leaf_context_digest:32] [merkle_root:32]
package main

import (
	"github.com/roasbeef/bip32-pq-zkp/batchclaim"
	"github.com/roasbeef/go-zkvm/zkvm"
)

func main() {
	zkvm.Debug("batch: start\n")

	// Step 1: Read the batch configuration from the private witness.
	// These parameters pin the leaf schema, hash algorithm, and the
	// exact leaf guest binary that produced the leaf receipts.
	var leafClaimKind uint32
	var merkleHashKind uint32
	var leafContextDigest [32]byte
	var leafCount uint32

	zkvm.ReadValue(&leafClaimKind)
	zkvm.ReadValue(&merkleHashKind)
	zkvm.ReadValue(&leafContextDigest)
	zkvm.ReadValue(&leafCount)

	// Step 2: Validate the batch configuration. The guest rejects
	// unsupported combinations early to avoid wasting prover work.
	if merkleHashKind != batchclaim.MerkleHashSHA256 {
		zkvm.Debug("batch: unsupported merkle hash kind\n")
		zkvm.Halt(1)
		return
	}
	if leafCount == 0 {
		zkvm.Debug("batch: leaf_count must be non-zero\n")
		zkvm.Halt(1)
		return
	}
	switch leafClaimKind {
	case batchclaim.LeafKindTaproot,
		batchclaim.LeafKindHardenedXPriv,
		batchclaim.LeafKindBatchClaimV1,
		batchclaim.LeafKindHeterogeneousEnvelopeV1:
	default:
		zkvm.Debug("batch: unsupported leaf claim kind\n")
		zkvm.Halt(1)
		return
	}

	leafClaimSize, ok := batchclaim.LeafClaimSize(leafClaimKind)
	if !ok {
		zkvm.Debug("batch: unsupported leaf claim size\n")
		zkvm.Halt(1)
		return
	}

	// Step 3: Read each leaf journal from stdin and verify it against
	// the pinned leaf guest image ID. zkvm.Verify adds an unresolved
	// assumption that the host-side recursion pipeline must resolve
	// against the corresponding succinct leaf receipt. If any leaf
	// receipt is invalid or missing, the batch proof will fail.
	leaves := make([][]byte, 0, leafCount)

	// When aggregating child batch claims, the guest enforces a
	// homogeneous subtree policy. The first child pins the expected
	// policy; every subsequent child must match exactly. This is
	// provably checked by the zkVM so verifiers can trust it
	// without seeing the individual child claims.
	var (
		childPolicySet      bool
		expectedChildPolicy childBatchPolicy
	)
	for i := uint32(0); i < leafCount; i++ {
		record := make([]byte, leafClaimSize)
		zkvm.Read(record)

		switch leafClaimKind {
		case batchclaim.LeafKindHeterogeneousEnvelopeV1:
			envelope, journal, err := decodeHeterogeneousEnvelope(
				record,
			)
			if err != nil {
				zkvm.Debug(
					"batch: decode heterogeneous envelope failed\n",
				)
				zkvm.Halt(1)
				return
			}
			if leafContextDigest != expectedHeterogeneousPolicyDigest() {
				zkvm.Debug(
					"batch: heterogeneous policy digest mismatch\n",
				)
				zkvm.Halt(1)
				return
			}

			// In heterogeneous mode each direct child carries its own
			// verify image ID inside the envelope.
			zkvm.Verify(envelope.VerifyImageID, journal)

		default:
			// This is the recursive composition call: it asserts that
			// a valid receipt exists for this (image_id, journal) pair.
			zkvm.Verify(leafContextDigest, record)

			if leafClaimKind == batchclaim.LeafKindBatchClaimV1 {
				childPolicy, err := decodeChildBatchPolicy(
					record,
				)
				if err != nil {
					zkvm.Debug(
						"batch: decode child batch claim failed\n",
					)
					zkvm.Halt(1)
					return
				}

				if !childPolicySet {
					expectedChildPolicy = childPolicy
					childPolicySet = true
				} else if !sameChildBatchPolicy(
					expectedChildPolicy, childPolicy,
				) {
					zkvm.Debug(
						"batch: child batch policy mismatch\n",
					)
					zkvm.Halt(1)
					return
				}
			}
		}

		leaves = append(leaves, record)
	}

	// Step 4: Build the Merkle root over the ordered, verified leaf
	// journals. The Merkle tree uses domain-separated SHA-256 with
	// leaf/interior node prefixes (see batchclaim/merkle.go).
	root, err := batchclaim.Root(leaves, zkvm.SumSHA256)
	if err != nil {
		zkvm.Debug("batch: merkle root failed\n")
		zkvm.Halt(1)
		return
	}

	// Step 5: Commit the 84-byte batch claim to the proof journal.
	// This is the only data the verifier sees from the batch receipt.
	claimVersion := uint32(batchclaim.Version)
	if leafClaimKind == batchclaim.LeafKindHeterogeneousEnvelopeV1 {
		claimVersion = batchclaim.VersionHeterogeneousParent
	}
	claim := batchclaim.Claim{
		Version:          claimVersion,
		Flags:            batchclaim.FlagsNone,
		LeafClaimKind:    leafClaimKind,
		MerkleHashKind:   merkleHashKind,
		LeafCount:        leafCount,
		LeafGuestImageID: leafContextDigest,
		MerkleRoot:       root,
	}
	encoded := claim.Encode()
	zkvm.Commit(encoded[:])
	zkvm.Debug("batch: committed\n")
	zkvm.Halt(0)
}

// childBatchPolicy captures the subset of a child batch claim that must be
// identical across all batch_claim_v1 leaves within one parent batch. This
// is the guest-side counterpart to the same-named struct in batch_support.go.
// Enforcing policy homogeneity inside the guest means the constraint is
// provably checked by the zkVM, not just trusted from the host.
type childBatchPolicy struct {
	version          uint32
	flags            uint32
	leafClaimKind    uint32
	merkleHashKind   uint32
	leafGuestImageID [32]byte
}

// decodeChildBatchPolicy parses a child batch claim journal into its
// policy-relevant fields. This is called inside the guest when the batch is
// aggregating batch_claim_v1 leaves to enforce that every child subtree
// shares the same version, flags, leaf kind, Merkle hash, and leaf guest
// image ID.
func decodeChildBatchPolicy(journal []byte) (childBatchPolicy, error) {
	claim, err := batchclaim.Decode(journal)
	if err != nil {
		return childBatchPolicy{}, err
	}

	return childBatchPolicy{
		version:          claim.Version,
		flags:            claim.Flags,
		leafClaimKind:    claim.LeafClaimKind,
		merkleHashKind:   claim.MerkleHashKind,
		leafGuestImageID: claim.LeafGuestImageID,
	}, nil
}

// sameChildBatchPolicy returns true iff both policies are identical across
// all five pinned fields. This is the guest-side enforcement point for the
// homogeneous child subtree invariant.
func sameChildBatchPolicy(a, b childBatchPolicy) bool {
	return a.version == b.version &&
		a.flags == b.flags &&
		a.leafClaimKind == b.leafClaimKind &&
		a.merkleHashKind == b.merkleHashKind &&
		a.leafGuestImageID == b.leafGuestImageID
}

// decodeHeterogeneousEnvelope parses one fixed-size heterogeneous direct-child
// record and returns both the decoded envelope metadata and the unpadded
// direct-child journal bytes.
func decodeHeterogeneousEnvelope(
	record []byte,
) (batchclaim.HeterogeneousEnvelopeV1, []byte, error) {
	envelope, err := batchclaim.DecodeHeterogeneousEnvelopeV1(record)
	if err != nil {
		return batchclaim.HeterogeneousEnvelopeV1{}, nil, err
	}

	return envelope, envelope.JournalBytes(), nil
}

// expectedHeterogeneousPolicyDigest returns the pinned policy digest for the
// first mixed direct-child parent mode.
func expectedHeterogeneousPolicyDigest() [32]byte {
	return batchclaim.HeterogeneousPolicyDigestV1()
}
