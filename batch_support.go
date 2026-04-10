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

	var imageIDHex string
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

func batchLeafJournalSize(kind uint32) (int, error) {
	switch kind {
	case BatchLeafKindTaproot, BatchLeafKindHardenedXPriv:
		return 72, nil
	default:
		return 0, fmt.Errorf("unsupported batch leaf kind %d", kind)
	}
}

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

func sumSHA256Host(data []byte) [32]byte {
	return sha256.Sum256(data)
}
