// batch_types.go defines the config, report, and claim-file types for the
// recursive batch aggregation lane. The batch lane aggregates N existing
// succinct leaf receipts into one final receipt whose public claim is a
// fixed 84-byte journal containing a Merkle root over the ordered leaf set.
//
// The key design property is that the batch claim size is constant
// regardless of N. Individual leaf details are disclosed via external
// Merkle inclusion proofs generated outside the guest, not by expanding
// the journal.
package bip32pqzkp

import (
	batch "github.com/roasbeef/bip32-pq-zkp/batchclaim"
	host "github.com/roasbeef/go-zkvm/host"
)

// DefaultBatchGuestPath is the packaged guest binary path used by the batch
// aggregation commands when the caller does not override it explicitly.
const DefaultBatchGuestPath = "./batch-platform-latest.bin"

const (
	// BatchLeafKindTaproot batches the original full Taproot leaf claim.
	BatchLeafKindTaproot = batch.LeafKindTaproot

	// BatchLeafKindHardenedXPriv batches the reduced hardened-xpriv
	// leaf claim.
	BatchLeafKindHardenedXPriv = batch.LeafKindHardenedXPriv
)

const (
	// BatchMerkleHashSHA256 selects the SHA-256 Merkle tree used
	// by the first aggregation implementation.
	BatchMerkleHashSHA256 = batch.MerkleHashSHA256
)

// BatchLeafInput identifies one existing leaf receipt plus its canonical
// claim.json artifact.
type BatchLeafInput struct {
	// ReceiptPath is the serialized leaf receipt to use as an assumption.
	ReceiptPath string

	// ClaimPath is the canonical leaf claim.json file used to recover the
	// pinned image ID and journal bytes.
	ClaimPath string
}

// BatchExecuteConfig describes one execute-only run of the batch aggregation
// guest.
type BatchExecuteConfig struct {
	// GuestPath points at the packaged batch guest `.bin` artifact to load.
	GuestPath string

	// LeafClaimKind identifies the schema shared by every leaf in
	// the batch.
	LeafClaimKind uint32

	// LeafInputs identifies the ordered leaves to aggregate.
	LeafInputs []BatchLeafInput
}

// BatchProveConfig describes one prove run of the batch guest plus artifact
// output paths.
type BatchProveConfig struct {
	// GuestPath points at the packaged batch guest `.bin` artifact to load.
	GuestPath string

	// LeafClaimKind identifies the schema shared by every
	// leaf in the batch.
	LeafClaimKind uint32

	// LeafInputs identifies the ordered leaves to aggregate.
	LeafInputs []BatchLeafInput

	// ReceiptKind selects the prove-time receipt compression level.
	ReceiptKind host.ReceiptKind

	// ReceiptOutputPath is where the final batch receipt should be written.
	ReceiptOutputPath string

	// ClaimOutputPath is where the canonical batch claim.json should go.
	ClaimOutputPath string
}

// BatchVerifyConfig describes one verification run against a stored batch
// receipt. It supports both batch-only and sparse-inclusion modes.
type BatchVerifyConfig struct {
	// GuestPath points at the packaged batch guest `.bin` artifact to load.
	GuestPath string

	// ReceiptInputPath is the serialized batch receipt to verify.
	ReceiptInputPath string

	// ClaimInputPath is an optional emitted batch claim.json file
	// to cross-check.
	ClaimInputPath string

	// InclusionProofInputPath optionally enables sparse
	// verification for one disclosed leaf via an ordinary
	// Merkle branch.
	InclusionProofInputPath string
}

// BatchDeriveInclusionConfig describes one host-side derivation of a sparse
// Merkle inclusion proof from an ordered leaf set.
type BatchDeriveInclusionConfig struct {
	// LeafClaimKind identifies the schema shared by every
	// leaf in the batch.
	LeafClaimKind uint32

	// LeafInputs identifies the ordered leaves used to derive the
	// batch root.
	LeafInputs []BatchLeafInput

	// LeafIndex is the disclosed leaf position.
	LeafIndex uint32

	// OutputPath is where the inclusion-proof artifact should be written.
	OutputPath string
}

// BatchClaimFile is the canonical human-readable verifier artifact written
// alongside one batch receipt.
type BatchClaimFile struct {
	// SchemaVersion identifies the JSON artifact schema.
	SchemaVersion uint32 `json:"schema_version"`

	// ImageID is the batch guest image ID used during prove and verify.
	ImageID string `json:"image_id"`

	// BatchVersion mirrors the journal claim version.
	BatchVersion uint32 `json:"batch_version"`

	// BatchFlags mirrors the journal policy flags.
	BatchFlags uint32 `json:"batch_flags"`

	// LeafClaimKind mirrors the pinned leaf schema identifier.
	LeafClaimKind uint32 `json:"leaf_claim_kind"`

	// LeafClaimKindName is the readable name of LeafClaimKind.
	LeafClaimKindName string `json:"leaf_claim_kind_name"`

	// MerkleHashKind mirrors the pinned Merkle-tree hash identifier.
	MerkleHashKind uint32 `json:"merkle_hash_kind"`

	// MerkleHashKindName is the readable name of MerkleHashKind.
	MerkleHashKindName string `json:"merkle_hash_kind_name"`

	// LeafCount is the number of leaves committed under the root.
	LeafCount uint32 `json:"leaf_count"`

	// LeafGuestImageID is the common leaf guest image ID as lowercase hex.
	LeafGuestImageID string `json:"leaf_guest_image_id"`

	// MerkleRoot is the committed batch root as lowercase hex.
	MerkleRoot string `json:"merkle_root"`

	// JournalHex is the raw committed journal as lowercase hex.
	JournalHex string `json:"journal_hex"`

	// JournalSizeBytes is the byte length of the committed journal.
	JournalSizeBytes int `json:"journal_size_bytes"`

	// ProofSealBytes is the proof seal size in bytes.
	ProofSealBytes uint64 `json:"proof_seal_bytes"`

	// ReceiptEncoding names the serialized receipt encoding
	// expected for the receipt artifact.
	ReceiptEncoding string `json:"receipt_encoding"`
}

// BatchInclusionProofFile stores one sparse-verification proof for a disclosed
// batch leaf.
type BatchInclusionProofFile struct {
	// SchemaVersion identifies the inclusion-proof JSON schema.
	SchemaVersion uint32 `json:"schema_version"`

	// LeafClaimKind mirrors the batch leaf schema identifier.
	LeafClaimKind uint32 `json:"leaf_claim_kind"`

	// LeafClaimKindName is the readable name of LeafClaimKind.
	LeafClaimKindName string `json:"leaf_claim_kind_name"`

	// MerkleHashKind mirrors the Merkle-tree hash identifier.
	MerkleHashKind uint32 `json:"merkle_hash_kind"`

	// MerkleHashKindName is the readable name of MerkleHashKind.
	MerkleHashKindName string `json:"merkle_hash_kind_name"`

	// LeafIndex is the disclosed leaf position.
	LeafIndex uint32 `json:"leaf_index"`

	// LeafCount is the total number of leaves in the batch.
	LeafCount uint32 `json:"leaf_count"`

	// LeafJournalHex is the disclosed leaf journal as lowercase hex.
	LeafJournalHex string `json:"leaf_journal_hex"`

	// Siblings are the sibling digests from leaf to root as lowercase hex.
	Siblings []string `json:"siblings"`
}

// BatchExecuteReport summarizes the public results of an execute-only run of
// the batch aggregation guest.
type BatchExecuteReport struct {
	// GuestPath is the batch guest artifact path used for the run.
	GuestPath string

	// GuestSize is the packaged guest size in bytes.
	GuestSize int

	// ImageID is the computed image ID for the loaded guest.
	ImageID string

	// LeafCount is the number of aggregated leaves.
	LeafCount uint32

	// Claim is the decoded batch claim committed by the guest.
	Claim batch.Claim

	// ExitCode is the guest exit code summary from execute-only mode.
	ExitCode string

	// JournalSize is the size of the committed journal in bytes.
	JournalSize int

	// SegmentCount is the number of zkVM segments executed.
	SegmentCount uint32

	// SessionRows is the total row count reported by the session.
	SessionRows uint64
}

// BatchProveReport summarizes one prove run and the generated batch artifacts.
type BatchProveReport struct {
	// GuestPath is the batch guest artifact path used for the proof.
	GuestPath string

	// GuestSize is the packaged guest size in bytes.
	GuestSize int

	// ImageID is the computed image ID for the loaded guest.
	ImageID string

	// LeafCount is the number of aggregated leaves.
	LeafCount uint32

	// Claim is the decoded public batch claim committed by the guest.
	Claim batch.Claim

	// ReceiptOutputPath is where the batch receipt artifact was written.
	ReceiptOutputPath string

	// ClaimOutputPath is where the readable batch claim artifact
	// was written.
	ClaimOutputPath string

	// JournalSize is the size of the committed journal in bytes.
	JournalSize int

	// ReceiptEncoding names the serialized receipt encoding.
	ReceiptEncoding string

	// ReceiptKind identifies the concrete receipt representation
	// returned by the prover.
	ReceiptKind host.ReceiptKind

	// ProverName identifies the selected proving backend.
	ProverName string

	// SealBytes is the proof seal size in bytes.
	SealBytes uint64
}

// BatchVerifyReport summarizes the result of verifying one stored batch
// receipt.
type BatchVerifyReport struct {
	// GuestPath is the batch guest artifact path used for verification.
	GuestPath string

	// GuestSize is the packaged guest size in bytes.
	GuestSize int

	// ImageID is the computed image ID for the loaded guest.
	ImageID string

	// Claim is the decoded public batch claim recovered from the receipt.
	Claim batch.Claim

	// ClaimInputPath is the optional batch claim.json used for comparison.
	ClaimInputPath string

	// ReceiptInputPath is the serialized batch receipt that was verified.
	ReceiptInputPath string

	// InclusionProofInputPath is the optional sparse-inclusion proof file.
	InclusionProofInputPath string

	// JournalSize is the size of the committed journal in bytes.
	JournalSize int

	// ReceiptEncoding names the serialized receipt encoding.
	ReceiptEncoding string

	// ReceiptKind identifies the concrete receipt representation that was
	// verified.
	ReceiptKind host.ReceiptKind

	// SealBytes is the proof seal size in bytes.
	SealBytes uint64
}

// BatchDeriveInclusionReport summarizes one derived sparse inclusion proof.
type BatchDeriveInclusionReport struct {
	// LeafClaimKind identifies the schema shared by every
	// leaf in the batch.
	LeafClaimKind uint32

	// LeafCount is the total number of leaves in the batch.
	LeafCount uint32

	// LeafIndex is the disclosed leaf position.
	LeafIndex uint32

	// OutputPath is where the inclusion-proof artifact was written.
	OutputPath string

	// MerkleRoot is the batch root the inclusion proof was derived against.
	MerkleRoot [32]byte
}
