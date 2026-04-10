// batch_nested_test.go validates the nested batch inclusion chain verifier
// and the bundled inclusion chain loader. The tests construct synthetic
// two-level hierarchies (parent batch over child batches over raw leaves)
// without invoking the actual prover, exercising only the host-side Merkle
// proof construction and chain verification logic.

package bip32pqzkp

import (
	"encoding/hex"
	"path/filepath"
	"testing"

	batch "github.com/roasbeef/bip32-pq-zkp/batchclaim"
)

func TestVerifyBatchInclusionChain(t *testing.T) {
	t.Parallel()

	rootClaim, proofs, expectedChildClaim := buildNestedBatchFixture(t)

	nestedClaims, err := VerifyBatchInclusionChain(rootClaim, proofs)
	if err != nil {
		t.Fatalf("VerifyBatchInclusionChain failed: %v", err)
	}
	if len(nestedClaims) != 1 {
		t.Fatalf("got %d nested claims, want 1", len(nestedClaims))
	}

	got := nestedClaims[0]
	if got.LeafClaimKind != expectedChildClaim.LeafClaimKind {
		t.Fatalf(
			"nested claim leaf kind = %d, want %d",
			got.LeafClaimKind,
			expectedChildClaim.LeafClaimKind,
		)
	}
	if got.LeafCount != expectedChildClaim.LeafCount {
		t.Fatalf(
			"nested claim leaf count = %d, want %d",
			got.LeafCount,
			expectedChildClaim.LeafCount,
		)
	}
	if got.MerkleRoot != expectedChildClaim.MerkleRoot {
		t.Fatalf(
			"nested claim Merkle root = %x, want %x",
			got.MerkleRoot,
			expectedChildClaim.MerkleRoot,
		)
	}
}

func TestVerifyBatchInclusionChainRejectsPrematureNonBatchLeaf(t *testing.T) {
	t.Parallel()

	rootClaim, proofs, _ := buildNestedBatchFixture(t)
	rootClaim.LeafClaimKind = BatchLeafKindHardenedXPriv

	if _, err := VerifyBatchInclusionChain(rootClaim, proofs); err == nil {
		t.Fatalf("expected premature non-batch leaf error")
	}
}

func TestVerifyBatchInclusionChainSupportsHeterogeneousParent(t *testing.T) {
	t.Parallel()

	rootClaim, proofs, expectedChildClaim :=
		buildNestedHeterogeneousFixture(t)

	nestedClaims, err := VerifyBatchInclusionChain(rootClaim, proofs)
	if err != nil {
		t.Fatalf("VerifyBatchInclusionChain failed: %v", err)
	}
	if len(nestedClaims) != 1 {
		t.Fatalf("got %d nested claims, want 1", len(nestedClaims))
	}
	if nestedClaims[0].MerkleRoot != expectedChildClaim.MerkleRoot {
		t.Fatalf(
			"nested claim Merkle root = %x, want %x",
			nestedClaims[0].MerkleRoot,
			expectedChildClaim.MerkleRoot,
		)
	}
}

func TestLoadBatchInclusionProofsFromBundledChain(t *testing.T) {
	t.Parallel()

	_, proofs, _ := buildNestedBatchFixture(t)
	chainPath := filepath.Join(t.TempDir(), "chain.json")
	err := WriteBatchInclusionChainFile(chainPath, BatchInclusionChainFile{
		SchemaVersion: 1,
		Proofs:        proofs,
	})
	if err != nil {
		t.Fatalf("WriteBatchInclusionChainFile failed: %v", err)
	}

	loaded, err := loadBatchInclusionProofs(nil, chainPath)
	if err != nil {
		t.Fatalf("loadBatchInclusionProofs failed: %v", err)
	}
	if len(loaded) != len(proofs) {
		t.Fatalf(
			"got %d proofs, want %d",
			len(loaded),
			len(proofs),
		)
	}
	if loaded[0].LeafJournalHex != proofs[0].LeafJournalHex {
		t.Fatalf("first bundled proof journal mismatch")
	}
}

func TestLoadBatchInclusionProofsRejectsMixedInputs(t *testing.T) {
	t.Parallel()

	chainPath := filepath.Join(t.TempDir(), "chain.json")
	err := WriteBatchInclusionChainFile(chainPath, BatchInclusionChainFile{
		SchemaVersion: 1,
		Proofs: []BatchInclusionProofFile{
			{SchemaVersion: 1},
		},
	})
	if err != nil {
		t.Fatalf("WriteBatchInclusionChainFile failed: %v", err)
	}

	_, err = loadBatchInclusionProofs([]string{"proof.json"}, chainPath)
	if err == nil {
		t.Fatalf("expected mixed inclusion input error")
	}
}

// buildNestedBatchFixture constructs a synthetic two-level batch hierarchy:
//
//	Parent = batch(childA, childB)
//	childA = batch(leafA0, leafA1)
//	childB = batch(leafB0, leafB1)
//
// It returns the parent claim, a two-element inclusion proof chain (parent
// level disclosing childB, then child level disclosing leafB0), and the
// expected childB claim for comparison.
func buildNestedBatchFixture(
	t *testing.T,
) (batch.Claim, []BatchInclusionProofFile, batch.Claim) {

	t.Helper()

	leafA0 := repeatedLeaf(0x11, 72)
	leafA1 := repeatedLeaf(0x22, 72)
	leafB0 := repeatedLeaf(0x33, 72)
	leafB1 := repeatedLeaf(0x44, 72)

	childLeafGuestImage := repeatedDigest(0xaa)
	parentLeafGuestImage := repeatedDigest(0xbb)

	childA := buildBatchClaim(
		t,
		batch.LeafKindHardenedXPriv,
		childLeafGuestImage,
		[][]byte{leafA0, leafA1},
	)
	childB := buildBatchClaim(
		t,
		batch.LeafKindHardenedXPriv,
		childLeafGuestImage,
		[][]byte{leafB0, leafB1},
	)

	childABytes := childA.Encode()
	childBBytes := childB.Encode()
	parentLeaves := [][]byte{childABytes[:], childBBytes[:]}
	parentClaim := buildBatchClaim(
		t,
		batch.LeafKindBatchClaimV1,
		parentLeafGuestImage,
		parentLeaves,
	)

	parentProof, _, err := batch.BuildProof(parentLeaves, 1, sumSHA256Host)
	if err != nil {
		t.Fatalf("BuildProof(parent) failed: %v", err)
	}
	childProof, _, err := batch.BuildProof(
		[][]byte{leafB0, leafB1}, 0, sumSHA256Host,
	)
	if err != nil {
		t.Fatalf("BuildProof(child) failed: %v", err)
	}

	proofs := []BatchInclusionProofFile{
		{
			SchemaVersion: 1,
			LeafClaimKind: batch.LeafKindBatchClaimV1,
			LeafClaimKindName: batch.LeafKindName(
				batch.LeafKindBatchClaimV1,
			),
			MerkleHashKind: batch.MerkleHashSHA256,
			MerkleHashKindName: batch.MerkleHashName(
				batch.MerkleHashSHA256,
			),
			LeafIndex: parentProof.LeafIndex,
			LeafCount: parentProof.LeafCount,
			LeafJournalHex: hex.EncodeToString(
				parentProof.LeafClaim,
			),
			Siblings: encodeDigestHexList(
				parentProof.Siblings,
			),
		},
		{
			SchemaVersion: 1,
			LeafClaimKind: batch.LeafKindHardenedXPriv,
			LeafClaimKindName: batch.LeafKindName(
				batch.LeafKindHardenedXPriv,
			),
			MerkleHashKind: batch.MerkleHashSHA256,
			MerkleHashKindName: batch.MerkleHashName(
				batch.MerkleHashSHA256,
			),
			LeafIndex: childProof.LeafIndex,
			LeafCount: childProof.LeafCount,
			LeafJournalHex: hex.EncodeToString(
				childProof.LeafClaim,
			),
			Siblings: encodeDigestHexList(childProof.Siblings),
		},
	}

	return parentClaim, proofs, childB
}

func buildNestedHeterogeneousFixture(
	t *testing.T,
) (batch.Claim, []BatchInclusionProofFile, batch.Claim) {

	t.Helper()

	rootClaim, proofs, expectedChildClaim := buildNestedBatchFixture(t)
	childEnvelope, err := batch.NewHeterogeneousEnvelopeV1(
		batch.LeafKindBatchClaimV1,
		repeatedDigest(0xbb),
		func() []byte {
			encoded := expectedChildClaim.Encode()
			return encoded[:]
		}(),
	)
	if err != nil {
		t.Fatalf("NewHeterogeneousEnvelopeV1 failed: %v", err)
	}
	rawEnvelope, err := batch.NewHeterogeneousEnvelopeV1(
		batch.LeafKindHardenedXPriv,
		repeatedDigest(0xaa),
		repeatedLeaf(0x55, 72),
	)
	if err != nil {
		t.Fatalf("NewHeterogeneousEnvelopeV1 failed: %v", err)
	}

	childEnvelopeBytes := childEnvelope.Encode()
	rawEnvelopeBytes := rawEnvelope.Encode()
	parentLeaves := [][]byte{rawEnvelopeBytes[:], childEnvelopeBytes[:]}
	parentRoot, err := batch.Root(parentLeaves, sumSHA256Host)
	if err != nil {
		t.Fatalf("Root failed: %v", err)
	}

	heterogeneousParent := batch.Claim{
		Version:          batch.VersionHeterogeneousParent,
		Flags:            batch.FlagsNone,
		LeafClaimKind:    batch.LeafKindHeterogeneousEnvelopeV1,
		MerkleHashKind:   batch.MerkleHashSHA256,
		LeafCount:        2,
		LeafGuestImageID: batch.HeterogeneousPolicyDigestV1(),
		MerkleRoot:       parentRoot,
	}

	parentProof, _, err := batch.BuildProof(parentLeaves, 1, sumSHA256Host)
	if err != nil {
		t.Fatalf("BuildProof(parent hetero) failed: %v", err)
	}

	proofs[0] = BatchInclusionProofFile{
		SchemaVersion: 1,
		LeafClaimKind: batch.LeafKindHeterogeneousEnvelopeV1,
		LeafClaimKindName: batch.LeafKindName(
			batch.LeafKindHeterogeneousEnvelopeV1,
		),
		MerkleHashKind: batch.MerkleHashSHA256,
		MerkleHashKindName: batch.MerkleHashName(
			batch.MerkleHashSHA256,
		),
		LeafIndex:      parentProof.LeafIndex,
		LeafCount:      parentProof.LeafCount,
		LeafJournalHex: hex.EncodeToString(parentProof.LeafClaim),
		DirectLeafKind: batch.LeafKindBatchClaimV1,
		DirectLeafKindName: batch.LeafKindName(
			batch.LeafKindBatchClaimV1,
		),
		DirectLeafImageID: hex.EncodeToString(
			childEnvelope.VerifyImageID[:],
		),
		Siblings: encodeDigestHexList(parentProof.Siblings),
	}

	_ = rootClaim
	return heterogeneousParent, proofs, expectedChildClaim
}

// buildBatchClaim is a test helper that constructs a batch.Claim from raw
// leaf journals using the host-side SHA-256 hash function.
func buildBatchClaim(
	t *testing.T,
	leafKind uint32,
	leafGuestImageID [32]byte,
	leaves [][]byte,
) batch.Claim {

	t.Helper()

	root, err := batch.Root(leaves, sumSHA256Host)
	if err != nil {
		t.Fatalf("Root failed: %v", err)
	}

	return batch.Claim{
		Version:          batch.Version,
		Flags:            batch.FlagsNone,
		LeafClaimKind:    leafKind,
		MerkleHashKind:   batch.MerkleHashSHA256,
		LeafCount:        uint32(len(leaves)),
		LeafGuestImageID: leafGuestImageID,
		MerkleRoot:       root,
	}
}

// repeatedLeaf returns a test leaf journal filled with a single byte value.
func repeatedLeaf(fill byte, size int) []byte {
	leaf := make([]byte, size)
	for i := range leaf {
		leaf[i] = fill
	}

	return leaf
}

// repeatedDigest returns a test 32-byte digest filled with a single byte value.
func repeatedDigest(fill byte) [32]byte {
	var digest [32]byte
	for i := range digest {
		digest[i] = fill
	}

	return digest
}
