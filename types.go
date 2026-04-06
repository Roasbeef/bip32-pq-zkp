package bip32pqzkp

import zkvmhost "github.com/roasbeef/go-zkvm/host"

// DefaultGuestPath is the packaged guest binary path used by the demo commands
// when the caller does not override it explicitly.
const DefaultGuestPath = "./bip32-platform-latest.bin"

const (
	bip32HardenedKeyStart   = 0x8000_0000
	bip86Purpose            = bip32HardenedKeyStart + 86
	witnessFlagRequireBIP86 = 1
	publicClaimSize         = 72
)

// PublicClaimVersion is the current demo-specific journal schema version.
const PublicClaimVersion = 1

var defaultBIP32Seed = []byte{
	0x00, 0x01, 0x02, 0x03,
	0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b,
	0x0c, 0x0d, 0x0e, 0x0f,
}

var defaultBIP32Path = []uint32{
	bip32HardenedKeyStart + 86,
	bip32HardenedKeyStart,
	bip32HardenedKeyStart,
	0,
	0,
}

// Runner owns the demo-specific host flow on top of the reusable go-zkvm host
// package.
type Runner struct {
	client *zkvmhost.Client
}

// WitnessConfig describes the private witness material fed into the guest.
type WitnessConfig struct {
	// SeedHex is the private seed encoded as hex.
	SeedHex string

	// Path is the private derivation path in slash or comma form.
	Path string

	// UseTestVector selects the built-in demo witness instead of
	// explicit input.
	UseTestVector bool

	// RequireBIP86 requests BIP-86 path-shape enforcement inside the claim.
	RequireBIP86 bool
}

// ExecuteConfig describes one execute-only run of the guest.
type ExecuteConfig struct {
	// GuestPath points at the packaged guest `.bin` artifact to load.
	GuestPath string

	// Witness carries the private input to feed into guest stdin.
	Witness WitnessConfig
}

// ProveConfig describes one prove run plus the artifact paths to write.
type ProveConfig struct {
	// GuestPath points at the packaged guest `.bin` artifact to load.
	GuestPath string

	// Witness carries the private input to feed into guest stdin.
	Witness WitnessConfig

	// ReceiptOutputPath is where the serialized receipt should be written.
	ReceiptOutputPath string

	// ClaimOutputPath is where the human-readable claim artifact should go.
	ClaimOutputPath string
}

// VerifyExpectations describes optional verifier-side checks beyond receipt
// verification against the image ID.
type VerifyExpectations struct {
	// PubKeyHex is the expected Taproot output key as lowercase hex.
	PubKeyHex string

	// PathCommitmentHex is the expected path commitment as lowercase hex.
	PathCommitmentHex string

	// Path is the expected private derivation path if known directly.
	Path string

	// RequireBIP86 checks the verifier-visible BIP-86 policy bit when set.
	RequireBIP86 *bool
}

// VerifyConfig describes one verification run against a stored receipt.
type VerifyConfig struct {
	// GuestPath points at the packaged guest `.bin` artifact to load.
	GuestPath string

	// ReceiptInputPath is the serialized receipt to verify.
	ReceiptInputPath string

	// ClaimInputPath is an optional emitted `claim.json` to cross-check.
	ClaimInputPath string

	// Expectations holds optional direct public checks.
	Expectations VerifyExpectations
}

// PublicClaim is the structured 72-byte journal committed by the guest.
type PublicClaim struct {
	// Version identifies the serialized claim format.
	Version uint32

	// Flags records verifier-visible policy bits.
	Flags uint32

	// TaprootOutputKey is the final x-only Taproot output key.
	TaprootOutputKey [32]byte

	// PathCommitment commits to the private derivation path.
	PathCommitment [32]byte
}

// ClaimFile is the human-readable verifier artifact written alongside the
// receipt.
type ClaimFile struct {
	// SchemaVersion identifies the JSON artifact schema.
	SchemaVersion uint32 `json:"schema_version"`
	// ImageID is the guest image ID used during prove and verify.
	ImageID string `json:"image_id"`

	// ClaimVersion mirrors the journal claim version.
	ClaimVersion uint32 `json:"claim_version"`

	// ClaimFlags mirrors the journal policy flags.
	ClaimFlags uint32 `json:"claim_flags"`

	// RequireBIP86 mirrors the verifier-visible BIP-86 policy bit.
	RequireBIP86 bool `json:"require_bip86"`

	// TaprootOutputKey is the final x-only output key as lowercase hex.
	TaprootOutputKey string `json:"taproot_output_key"`

	// PathCommitment is the committed path digest as lowercase hex.
	PathCommitment string `json:"path_commitment"`

	// JournalHex is the raw committed journal as lowercase hex.
	JournalHex string `json:"journal_hex"`

	// JournalSizeBytes is the byte length of the committed journal.
	JournalSizeBytes int `json:"journal_size_bytes"`

	// ProofSealBytes is the size of the proof seal in bytes.
	ProofSealBytes uint64 `json:"proof_seal_bytes"`

	// ReceiptEncoding names the serialized receipt encoding.
	ReceiptEncoding string `json:"receipt_encoding"`
}

// ExecuteReport summarizes the public results of an execute-only run.
type ExecuteReport struct {
	// GuestPath is the guest artifact path used for the run.
	GuestPath string
	// GuestSize is the packaged guest size in bytes.
	GuestSize int

	// ImageID is the computed image ID for the loaded guest.
	ImageID string

	// UsingTestVector reports whether the built-in witness was used.
	UsingTestVector bool

	// Claim is the decoded public claim committed by the guest.
	Claim PublicClaim

	// ExitCode is the guest exit code summary from execute-only mode.
	ExitCode string

	// JournalSize is the size of the committed journal in bytes.
	JournalSize int

	// SegmentCount is the number of zkVM segments executed.
	SegmentCount uint32

	// SessionRows is the total row count reported by the session.
	SessionRows uint64
}

// ProveReport summarizes a prove run and the generated artifacts.
type ProveReport struct {
	// GuestPath is the guest artifact path used for the proof.
	GuestPath string

	// GuestSize is the packaged guest size in bytes.
	GuestSize int

	// ImageID is the computed image ID for the loaded guest.
	ImageID string

	// UsingTestVector reports whether the built-in witness was used.
	UsingTestVector bool

	// Claim is the decoded public claim committed by the guest.
	Claim PublicClaim

	// ReceiptOutputPath is where the receipt artifact was written.
	ReceiptOutputPath string

	// ClaimOutputPath is where the readable claim artifact was written.
	ClaimOutputPath string

	// JournalSize is the size of the committed journal in bytes.
	JournalSize int

	// ReceiptEncoding names the serialized receipt encoding.
	ReceiptEncoding string

	// ProverName identifies the selected proving backend.
	ProverName string

	// SealBytes is the proof seal size in bytes.
	SealBytes uint64
}

// VerifyReport summarizes the result of verifying a stored receipt.
type VerifyReport struct {
	// GuestPath is the guest artifact path used for verification.
	GuestPath string

	// GuestSize is the packaged guest size in bytes.
	GuestSize int

	// ImageID is the computed image ID for the loaded guest.
	ImageID string

	// Claim is the decoded public claim recovered from the receipt.
	Claim PublicClaim

	// ClaimInputPath is the optional `claim.json` used for comparison.
	ClaimInputPath string

	// ReceiptInputPath is the serialized receipt that was verified.
	ReceiptInputPath string

	// JournalSize is the size of the committed journal in bytes.
	JournalSize int

	// ReceiptEncoding names the serialized receipt encoding.
	ReceiptEncoding string

	// SealBytes is the proof seal size in bytes.
	SealBytes uint64
}
