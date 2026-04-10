// batch_support.go provides the leaf loading, witness building, claim file
// I/O, and verification helpers for the batch aggregation lane. The most
// important helper is loadBatchLeaves, which:
//
//  1. Reads each leaf's claim.json to recover the journal bytes and image ID.
//  2. Verifies each leaf receipt against its image ID and expected journal
//     (this is a host-side check, separate from the guest-side verification).
//  3. Enforces that all leaf receipts are succinct (a risc0 requirement:
//     only succinct receipts can be supplied as host-side assumptions).
//  4. Collects the receipt bytes as AssumptionReceipt values for the
//     go-zkvm host API.
//
// The sparse inclusion proof verification in verifyBatchInclusionProof
// runs entirely outside the zkVM. It recomputes the Merkle root from the
// disclosed leaf journal and the sibling hashes, then checks that root
// against the one committed in the verified batch claim.
package bip32pqzkp

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	batch "github.com/roasbeef/bip32-pq-zkp/batchclaim"
	zkvmhost "github.com/roasbeef/go-zkvm/host"
)

// genericLeafClaimFile is a minimal claim.json reader that extracts only the
// fields needed by the batch loader: image_id and journal_hex. This avoids
// importing every leaf-specific claim type.
type genericLeafClaimFile struct {
	ImageID         string `json:"image_id"`
	JournalHex      string `json:"journal_hex"`
	JournalSize     int    `json:"journal_size_bytes"`
	ReceiptEncoding string `json:"receipt_encoding"`
}

// loadedBatchLeaves is the resolved result of loading all leaf inputs.
// The journals are passed to the batch guest via stdin, and the assumptions
// are passed to the go-zkvm host API so the risc0 recursion pipeline can
// resolve the guest-side zkvm.Verify calls.
type loadedBatchLeaves struct {
	// leafGuestImageID is the common image ID shared by all leaf receipts.
	leafGuestImageID [32]byte

	// journals holds the raw leaf journal bytes in batch order.
	journals [][]byte

	// assumptions holds the serialized succinct leaf receipts that the
	// host supplies to the prover for recursive assumption resolution.
	assumptions []zkvmhost.AssumptionReceipt
}

// childBatchPolicy captures the subset of a child batch claim that must be
// identical across all batch_claim_v1 leaves within one parent batch. The
// parent verifier enforces this homogeneity so that a single parent receipt
// implicitly guarantees that every child subtree was produced with the same
// guest binary, leaf schema, and Merkle hash construction.
type childBatchPolicy struct {
	version          uint32
	flags            uint32
	leafClaimKind    uint32
	merkleHashKind   uint32
	leafGuestImageID [32]byte
}

// DecodeBatchClaim decodes the fixed-size public batch claim committed by the
// aggregation guest.
func DecodeBatchClaim(journal []byte) (batch.Claim, error) {
	return batch.Decode(journal)
}

// NewBatchClaimFile converts the verified batch journal into the canonical
// human-readable batch claim.json artifact.
func NewBatchClaimFile(
	imageID string, claim batch.Claim, journal []byte, sealBytes uint64,
	receiptEncoding string,
) BatchClaimFile {

	return BatchClaimFile{
		SchemaVersion:     1,
		ImageID:           imageID,
		BatchVersion:      claim.Version,
		BatchFlags:        claim.Flags,
		LeafClaimKind:     claim.LeafClaimKind,
		LeafClaimKindName: batch.LeafKindName(claim.LeafClaimKind),
		MerkleHashKind:    claim.MerkleHashKind,
		MerkleHashKindName: batch.MerkleHashName(
			claim.MerkleHashKind,
		),
		LeafCount:        claim.LeafCount,
		LeafGuestImageID: hex.EncodeToString(claim.LeafGuestImageID[:]),
		MerkleRoot:       hex.EncodeToString(claim.MerkleRoot[:]),
		JournalHex:       hex.EncodeToString(journal),
		JournalSizeBytes: len(journal),
		ProofSealBytes:   sealBytes,
		ReceiptEncoding:  receiptEncoding,
	}
}

// ReadBatchClaimFile loads a previously written batch claim.json artifact.
func ReadBatchClaimFile(path string) (BatchClaimFile, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return BatchClaimFile{}, fmt.Errorf(
			"read claim `%s`: %w", path, err,
		)
	}

	var claim BatchClaimFile
	if err := json.Unmarshal(bytes, &claim); err != nil {
		return BatchClaimFile{}, fmt.Errorf(
			"deserialize batch claim JSON: %w", err,
		)
	}

	return claim, nil
}

// WriteBatchClaimFile writes the canonical human-readable batch claim.json
// artifact to disk.
func WriteBatchClaimFile(path string, claim BatchClaimFile) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create `%s`: %w", path, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(claim); err != nil {
		return fmt.Errorf("serialize batch claim JSON: %w", err)
	}

	return nil
}

// ReadBatchInclusionProofFile loads a previously written sparse inclusion
// proof file.
func ReadBatchInclusionProofFile(path string) (BatchInclusionProofFile, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return BatchInclusionProofFile{}, fmt.Errorf(
			"read inclusion proof `%s`: %w", path, err,
		)
	}

	var proof BatchInclusionProofFile
	if err := json.Unmarshal(bytes, &proof); err != nil {
		return BatchInclusionProofFile{}, fmt.Errorf(
			"deserialize batch inclusion proof JSON: %w", err,
		)
	}

	return proof, nil
}

// WriteBatchInclusionProofFile writes a sparse batch inclusion proof to disk.
func WriteBatchInclusionProofFile(
	path string, proof BatchInclusionProofFile,
) error {

	if err := ensureParentDir(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create `%s`: %w", path, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(proof); err != nil {
		return fmt.Errorf(
			"serialize batch inclusion proof JSON: %w", err,
		)
	}

	return nil
}

// ReadBatchInclusionChainFile loads a previously written bundled inclusion
// chain artifact.
func ReadBatchInclusionChainFile(path string) (BatchInclusionChainFile, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return BatchInclusionChainFile{}, fmt.Errorf(
			"read inclusion chain `%s`: %w", path, err,
		)
	}

	var chain BatchInclusionChainFile
	if err := json.Unmarshal(bytes, &chain); err != nil {
		return BatchInclusionChainFile{}, fmt.Errorf(
			"deserialize batch inclusion chain JSON: %w", err,
		)
	}

	return chain, nil
}

// WriteBatchInclusionChainFile writes a bundled inclusion chain to disk.
func WriteBatchInclusionChainFile(
	path string, chain BatchInclusionChainFile,
) error {

	if err := ensureParentDir(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create `%s`: %w", path, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(chain); err != nil {
		return fmt.Errorf(
			"serialize batch inclusion chain JSON: %w", err,
		)
	}

	return nil
}

// verifyBatchClaimFileMatches compares an on-disk claim.json artifact against
// the claim decoded from a verified receipt. This detects tampering or version
// drift. ProofSealBytes is intentionally excluded because the seal size can
// change between re-proofing runs without affecting the committed claim.
func verifyBatchClaimFileMatches(
	expected BatchClaimFile, verified BatchClaimFile,
) error {

	matches := expected.SchemaVersion == verified.SchemaVersion &&
		expected.ImageID == verified.ImageID &&
		expected.BatchVersion == verified.BatchVersion &&
		expected.BatchFlags == verified.BatchFlags &&
		expected.LeafClaimKind == verified.LeafClaimKind &&
		expected.LeafClaimKindName == verified.LeafClaimKindName &&
		expected.MerkleHashKind == verified.MerkleHashKind &&
		expected.MerkleHashKindName == verified.MerkleHashKindName &&
		expected.LeafCount == verified.LeafCount &&
		expected.LeafGuestImageID == verified.LeafGuestImageID &&
		expected.MerkleRoot == verified.MerkleRoot &&
		expected.JournalHex == verified.JournalHex &&
		expected.JournalSizeBytes == verified.JournalSizeBytes &&
		expected.ReceiptEncoding == verified.ReceiptEncoding

	if !matches {
		return errors.New(
			"batch claim file does not match the verified " +
				"public receipt output",
		)
	}

	return nil
}

// buildBatchWitnessStdin serializes the private witness stream that the batch
// guest reads from stdin. The wire format is:
//
//	[leaf_claim_kind:u32_le] [merkle_hash_kind:u32_le]
//	[leaf_guest_image_id:32] [leaf_count:u32_le]
//	[leaf_journal_0:*] [leaf_journal_1:*] ... [leaf_journal_N-1:*]
//
// Each leaf journal must be exactly the fixed size for the given leaf kind.
// The host passes these journals as private witness data; they never appear
// in the batch receipt itself. The guest re-verifies each journal against its
// corresponding succinct leaf receipt via zkvm.Verify.
func buildBatchWitnessStdin(
	leafClaimKind uint32, leafGuestImageID [32]byte, journals [][]byte,
) ([]byte, error) {

	if len(journals) == 0 {
		return nil, errors.New(
			"batch witness requires at least one leaf",
		)
	}

	expectedLeafSize, err := batchLeafJournalSize(leafClaimKind)
	if err != nil {
		return nil, err
	}

	totalBytes := 4 + 4 + 32 + 4 + len(journals)*expectedLeafSize
	var stdin bytes.Buffer
	stdin.Grow(totalBytes)

	for _, word := range []uint32{
		leafClaimKind,
		batch.MerkleHashSHA256,
	} {
		if err := binary.Write(
			&stdin, binary.LittleEndian, word,
		); err != nil {
			return nil, fmt.Errorf(
				"write batch witness header: %w", err,
			)
		}
	}

	if _, err := stdin.Write(leafGuestImageID[:]); err != nil {
		return nil, fmt.Errorf("write leaf guest image ID: %w", err)
	}
	if err := binary.Write(
		&stdin, binary.LittleEndian, uint32(len(journals)),
	); err != nil {
		return nil, fmt.Errorf("write batch leaf count: %w", err)
	}

	for idx, journal := range journals {
		if len(journal) != expectedLeafSize {
			return nil, fmt.Errorf(
				"leaf %d journal size mismatch: got %d, want "+
					"%d",
				idx,
				len(journal),
				expectedLeafSize,
			)
		}
		if _, err := stdin.Write(journal); err != nil {
			return nil, fmt.Errorf(
				"write leaf %d journal: %w", idx, err,
			)
		}
	}

	return stdin.Bytes(), nil
}

// loadBatchLeaves reads and validates all leaf inputs for a batch aggregation
// run. For each leaf it:
//
//  1. Reads the claim.json to recover the journal bytes and image ID.
//  2. Verifies the leaf receipt against its image ID and expected journal
//     (host-side sanity check before the guest sees the receipt).
//  3. Enforces that the receipt is succinct, since risc0 requires succinct
//     receipts for host-side assumption resolution during composition.
//  4. When the leaf kind is batch_claim_v1, validates that all child batch
//     claims share the same subtree policy (version, flags, leaf kind,
//     Merkle hash kind, and leaf guest image ID).
//
// The returned loadedBatchLeaves contains the common leaf guest image ID,
// the ordered journals for the witness stdin, and the serialized succinct
// receipts as AssumptionReceipt values for the go-zkvm host API.
func (r *Runner) loadBatchLeaves(
	leafClaimKind uint32, inputs []BatchLeafInput,
) (*loadedBatchLeaves, error) {

	if len(inputs) == 0 {
		return nil, errors.New(
			"batch proof requires at least one leaf input",
		)
	}

	expectedLeafSize, err := batchLeafJournalSize(leafClaimKind)
	if err != nil {
		return nil, err
	}

	result := &loadedBatchLeaves{
		journals:    make([][]byte, 0, len(inputs)),
		assumptions: make([]zkvmhost.AssumptionReceipt, 0, len(inputs)),
	}

	// imageIDHex is the first leaf's image ID; all subsequent leaves
	// must share the same value.
	var imageIDHex string

	// For batch_claim_v1 leaves, we enforce that every child batch
	// claim shares the same subtree policy. The first leaf pins the
	// expected policy; subsequent leaves must match exactly.
	var (
		childPolicySet      bool
		expectedChildPolicy childBatchPolicy
	)
	for idx, input := range inputs {
		if input.ReceiptPath == "" {
			return nil, fmt.Errorf(
				"leaf %d missing receipt path", idx,
			)
		}
		if input.ClaimPath == "" {
			return nil, fmt.Errorf(
				"leaf %d missing claim path", idx,
			)
		}

		claimFile, err := readGenericLeafClaimFile(input.ClaimPath)
		if err != nil {
			return nil, err
		}

		journal, err := hex.DecodeString(claimFile.JournalHex)
		if err != nil {
			return nil, fmt.Errorf(
				"decode leaf %d journal hex: %w", idx, err,
			)
		}
		if len(journal) != expectedLeafSize {
			return nil, fmt.Errorf(
				"leaf %d journal size mismatch: got %d, want "+
					"%d",
				idx,
				len(journal),
				expectedLeafSize,
			)
		}

		// When batching child batch claims (nested hierarchy), enforce
		// that every child was built with the same guest binary, leaf
		// schema, hash algorithm, and policy flags. Without this check
		// the parent could silently aggregate heterogeneous subtrees.
		if leafClaimKind == BatchLeafKindBatchClaimV1 {
			childPolicy, err := decodeChildBatchPolicyHost(
				journal,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"decode child batch claim %d: %w",
					idx,
					err,
				)
			}

			if !childPolicySet {
				expectedChildPolicy = childPolicy
				childPolicySet = true
			} else {
				samePolicy := sameChildBatchPolicy(
					expectedChildPolicy,
					childPolicy,
				)
				if !samePolicy {
					return nil, fmt.Errorf(
						"child batch claim %d does "+
							"not match the pinned "+
							"child subtree "+
							"policy",
						idx,
					)
				}
			}
		}

		receiptBytes, err := os.ReadFile(input.ReceiptPath)
		if err != nil {
			return nil, fmt.Errorf(
				"read leaf receipt `%s`: %w",
				input.ReceiptPath, err,
			)
		}

		if imageIDHex == "" {
			imageIDHex = claimFile.ImageID
			decodedImageID, err := decodeHexArray32(
				"leaf image ID", imageIDHex,
			)
			if err != nil {
				return nil, err
			}
			result.leafGuestImageID = decodedImageID
		} else if claimFile.ImageID != imageIDHex {
			return nil, fmt.Errorf(
				"leaf %d image ID mismatch: got %s, want %s",
				idx, claimFile.ImageID, imageIDHex,
			)
		}

		// Host-side receipt verification: confirm the leaf receipt is
		// valid for this image ID and journal before we pass it to the
		// guest as an assumption. This catches corrupt or mismatched
		// receipts early rather than failing inside the prover.
		verifyResult, err := r.client.Verify(zkvmhost.VerifyRequest{
			Receipt:         receiptBytes,
			ImageID:         imageIDHex,
			ExpectedJournal: journal,
		})
		if err != nil {
			return nil, fmt.Errorf(
				"verify leaf receipt %d against claim: %w",
				idx, err,
			)
		}
		// Only succinct receipts can serve as assumptions in the risc0
		// recursion pipeline. Composite receipts cannot be supplied as
		// host-side assumptions, so we reject them at load time.
		if verifyResult.ReceiptKind != zkvmhost.ReceiptKindSuccinct {
			return nil, fmt.Errorf(
				"leaf %d receipt kind must be succinct, got %s",
				idx, verifyResult.ReceiptKind,
			)
		}

		result.journals = append(result.journals, journal)
		result.assumptions = append(
			result.assumptions,
			zkvmhost.AssumptionReceipt(receiptBytes),
		)
	}

	return result, nil
}

// readGenericLeafClaimFile loads a claim.json file using only the fields that
// the batch loader needs (image_id, journal_hex). This avoids importing the
// full leaf-specific claim type, which makes the batch loader agnostic to the
// concrete leaf schema.
func readGenericLeafClaimFile(path string) (genericLeafClaimFile, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return genericLeafClaimFile{}, fmt.Errorf(
			"read claim `%s`: %w", path, err,
		)
	}

	var claim genericLeafClaimFile
	if err := json.Unmarshal(bytes, &claim); err != nil {
		return genericLeafClaimFile{}, fmt.Errorf(
			"deserialize generic leaf claim JSON: %w", err,
		)
	}
	if claim.ImageID == "" {
		return genericLeafClaimFile{}, errors.New(
			"generic leaf claim is missing image_id",
		)
	}
	if claim.JournalHex == "" {
		return genericLeafClaimFile{}, errors.New(
			"generic leaf claim is missing journal_hex",
		)
	}

	return claim, nil
}

// batchLeafJournalSize returns the fixed journal byte size for the given leaf
// kind. Every leaf in a batch must have the same journal size, so this is
// checked once during leaf loading and again during witness construction.
func batchLeafJournalSize(kind uint32) (int, error) {
	size, ok := batch.LeafClaimSize(kind)
	if !ok {
		return 0, fmt.Errorf("unsupported batch leaf kind %d", kind)
	}

	return size, nil
}

// verifyBatchInclusionProof checks one level-local sparse inclusion proof
// against a batch claim. It decodes the disclosed leaf journal and sibling
// hashes from the proof file, recomputes the Merkle root from leaf to root,
// and verifies the result matches the claim's committed Merkle root. This
// runs entirely outside the zkVM as a host-side post-verification step.
func verifyBatchInclusionProof(
	claim batch.Claim, inclusion BatchInclusionProofFile,
) error {

	if inclusion.LeafClaimKind != claim.LeafClaimKind {
		return fmt.Errorf(
			"inclusion proof leaf kind mismatch: got %d, want %d",
			inclusion.LeafClaimKind, claim.LeafClaimKind,
		)
	}
	if inclusion.MerkleHashKind != claim.MerkleHashKind {
		return fmt.Errorf(
			"inclusion proof Merkle hash mismatch: got %d, want %d",
			inclusion.MerkleHashKind, claim.MerkleHashKind,
		)
	}
	if inclusion.LeafCount != claim.LeafCount {
		return fmt.Errorf(
			"inclusion proof leaf count mismatch: got %d, want %d",
			inclusion.LeafCount, claim.LeafCount,
		)
	}

	leafJournal, err := hex.DecodeString(inclusion.LeafJournalHex)
	if err != nil {
		return fmt.Errorf("decode inclusion leaf journal: %w", err)
	}

	siblings := make([][32]byte, 0, len(inclusion.Siblings))
	for idx, siblingHex := range inclusion.Siblings {
		sibling, err := decodeHexArray32(
			fmt.Sprintf("inclusion sibling %d", idx), siblingHex,
		)
		if err != nil {
			return err
		}
		siblings = append(siblings, sibling)
	}

	ok := batch.VerifyProof(
		claim.MerkleRoot,
		batch.Proof{
			LeafIndex: inclusion.LeafIndex,
			LeafCount: inclusion.LeafCount,
			LeafClaim: leafJournal,
			Siblings:  siblings,
		},
		sumSHA256Host,
	)
	if !ok {
		return errors.New("batch inclusion proof did not verify")
	}

	return nil
}

// VerifyBatchInclusionChain checks one or more sparse inclusion proofs
// against a top-level batch claim. Each non-final proof must disclose one
// child batch claim, which then becomes the claim verified at the next
// level. The final proof may disclose any supported non-batch leaf claim.
func VerifyBatchInclusionChain(
	rootClaim batch.Claim, inclusions []BatchInclusionProofFile,
) ([]batch.Claim, error) {

	if len(inclusions) == 0 {
		return nil, errors.New(
			"nested batch verification requires at least one " +
				"inclusion proof",
		)
	}

	currentClaim := rootClaim
	nestedClaims := make([]batch.Claim, 0, max(0, len(inclusions)-1))
	for idx, inclusion := range inclusions {
		if err := verifyBatchInclusionProof(
			currentClaim, inclusion,
		); err != nil {
			return nil, fmt.Errorf(
				"verify inclusion proof %d: %w", idx, err,
			)
		}

		if idx == len(inclusions)-1 {
			break
		}

		if currentClaim.LeafClaimKind != BatchLeafKindBatchClaimV1 {
			return nil, fmt.Errorf(
				"inclusion proof %d reached non-batch leaf "+
					"kind %s before the final level",
				idx,
				batch.LeafKindName(currentClaim.LeafClaimKind),
			)
		}

		childClaim, err := decodeNestedBatchClaim(
			inclusion.LeafJournalHex,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"decode nested batch claim from inclusion "+
					"%d: %w",
				idx, err,
			)
		}

		nestedClaims = append(nestedClaims, childClaim)
		currentClaim = childClaim
	}

	return nestedClaims, nil
}

// loadBatchInclusionProofs loads inclusion proofs from one of two mutually
// exclusive sources: either a list of individual per-level proof files
// (specified via repeated --inclusion-in flags), or a single bundled
// inclusion chain file (specified via --inclusion-chain-in). Providing both
// is an error. Returns nil if neither source is provided.
func loadBatchInclusionProofs(
	inclusionPaths []string, inclusionChainPath string,
) ([]BatchInclusionProofFile, error) {

	if len(inclusionPaths) != 0 && inclusionChainPath != "" {
		return nil, errors.New(
			"provide either repeated inclusion proof files or " +
				"one inclusion chain file, not both",
		)
	}

	if inclusionChainPath != "" {
		chain, err := ReadBatchInclusionChainFile(
			inclusionChainPath,
		)
		if err != nil {
			return nil, err
		}
		if len(chain.Proofs) == 0 {
			return nil, errors.New(
				"inclusion chain must contain at " +
					"least one proof",
			)
		}

		return append(
			[]BatchInclusionProofFile(nil),
			chain.Proofs...,
		), nil
	}

	if len(inclusionPaths) == 0 {
		return nil, nil
	}

	inclusionProofs := make(
		[]BatchInclusionProofFile, 0, len(inclusionPaths),
	)
	for _, path := range inclusionPaths {
		inclusionProof, err := ReadBatchInclusionProofFile(path)
		if err != nil {
			return nil, err
		}
		inclusionProofs = append(inclusionProofs, inclusionProof)
	}

	return inclusionProofs, nil
}

// decodeNestedBatchClaim hex-decodes a disclosed child batch journal and
// parses it as a batch.Claim. This is used during nested inclusion chain
// verification: when a non-final proof discloses a batch_claim_v1 leaf, the
// verifier decodes it into a Claim so it can serve as the claim for the next
// level's inclusion proof.
func decodeNestedBatchClaim(journalHex string) (batch.Claim, error) {
	journal, err := hex.DecodeString(journalHex)
	if err != nil {
		return batch.Claim{}, fmt.Errorf(
			"decode nested batch journal hex: %w", err,
		)
	}

	claim, err := DecodeBatchClaim(journal)
	if err != nil {
		return batch.Claim{}, err
	}

	return claim, nil
}

// decodeChildBatchPolicyHost extracts the policy-relevant fields from a
// serialized child batch claim on the host side. This mirrors the guest-side
// decodeChildBatchPolicy but operates on raw journal bytes rather than zkVM
// stdin. The host uses this during leaf loading to enforce that all
// batch_claim_v1 leaves share the same subtree policy before sending them to
// the guest.
func decodeChildBatchPolicyHost(journal []byte) (childBatchPolicy, error) {
	claim, err := DecodeBatchClaim(journal)
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

// sameChildBatchPolicy returns true iff both policies agree on every field.
// This enforces homogeneous child subtrees: a parent batch must not silently
// mix child batches built from different leaf schemas, hash algorithms, or
// guest binaries.
func sameChildBatchPolicy(a, b childBatchPolicy) bool {
	return a.version == b.version &&
		a.flags == b.flags &&
		a.leafClaimKind == b.leafClaimKind &&
		a.merkleHashKind == b.merkleHashKind &&
		a.leafGuestImageID == b.leafGuestImageID
}

// sumSHA256Host computes a plain SHA-256 digest on the host side. This is the
// host-side counterpart to zkvm.SumSHA256 used inside the guest. Both must
// produce identical digests for the same input; the host uses crypto/sha256
// while the guest uses the zkVM SHA acceleration syscalls.
func sumSHA256Host(data []byte) [32]byte {
	return sha256.Sum256(data)
}
