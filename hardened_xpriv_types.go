// hardened_xpriv_types.go defines the config, report, and claim-file types
// for the reduced hardened-xpriv proof lane. The type hierarchy mirrors the
// full Taproot lane (types.go) but with xpriv-specific fields: the witness
// uses a parent xpriv + chain code + single hardened child index, and the
// public claim reveals the derived child private key and chain code.
package bip32pqzkp

import (
	localbip32 "github.com/roasbeef/bip32-pq-zkp/bip32"
	host "github.com/roasbeef/go-zkvm/host"
)

// HardenedXPrivClaim is the verifier-visible public claim committed by the
// reduced hardened-xpriv guest. It is a type alias to avoid duplicating the
// bip32 package's claim struct at the root package level.
type HardenedXPrivClaim = localbip32.HardenedXPrivClaim

// HardenedXPrivWitnessConfig describes the private witness material fed into
// the hardened-xpriv guest.
type HardenedXPrivWitnessConfig struct {
	// ParentPrivateKeyHex is the serialized parent xpriv scalar.
	ParentPrivateKeyHex string

	// ParentChainCodeHex is the parent BIP-32 chain code.
	ParentChainCodeHex string

	// Path is the single hardened child step to derive.
	Path string

	// UseTestVector selects the built-in reduced-variant witness instead of
	// explicit input.
	UseTestVector bool
}

// HardenedXPrivExecuteConfig describes one execute-only run of the reduced
// hardened-xpriv guest.
type HardenedXPrivExecuteConfig struct {
	// GuestPath points at the packaged guest `.bin` artifact to load.
	GuestPath string

	// Witness carries the private input to feed into guest stdin.
	Witness HardenedXPrivWitnessConfig
}

// HardenedXPrivProveConfig describes one prove run plus the artifact paths to
// write for the reduced hardened-xpriv guest.
type HardenedXPrivProveConfig struct {
	// GuestPath points at the packaged guest `.bin` artifact to load.
	GuestPath string

	// Witness carries the private input to feed into guest stdin.
	Witness HardenedXPrivWitnessConfig

	// ReceiptKind selects the prove-time receipt compression level.
	ReceiptKind host.ReceiptKind

	// ReceiptOutputPath is where the serialized receipt should be written.
	ReceiptOutputPath string

	// ClaimOutputPath is where the human-readable claim artifact should go.
	ClaimOutputPath string
}

// HardenedXPrivVerifyExpectations describes optional verifier-side checks for
// the reduced hardened-xpriv claim.
type HardenedXPrivVerifyExpectations struct {
	// ChildPrivateKeyHex is the expected serialized child xpriv.
	ChildPrivateKeyHex string

	// ChainCodeHex is the expected child chain code.
	ChainCodeHex string
}

// HardenedXPrivVerifyConfig describes one verification run against a stored
// hardened-xpriv receipt.
type HardenedXPrivVerifyConfig struct {
	// GuestPath points at the packaged guest `.bin` artifact to load.
	GuestPath string

	// ReceiptInputPath is the serialized receipt to verify.
	ReceiptInputPath string

	// ClaimInputPath is an optional emitted `claim.json` to cross-check.
	ClaimInputPath string

	// Expectations holds optional direct public checks.
	Expectations HardenedXPrivVerifyExpectations
}

// HardenedXPrivClaimFile is the canonical human-readable verifier artifact
// written alongside a hardened-xpriv receipt.
type HardenedXPrivClaimFile struct {
	// SchemaVersion identifies the JSON artifact schema.
	SchemaVersion uint32 `json:"schema_version"`

	// ImageID is the guest image ID used during prove and verify.
	ImageID string `json:"image_id"`

	// ClaimVersion mirrors the journal claim version.
	ClaimVersion uint32 `json:"claim_version"`

	// ClaimFlags mirrors the journal policy flags.
	ClaimFlags uint32 `json:"claim_flags"`

	// ChildPrivateKey is the final child xpriv as lowercase hex.
	ChildPrivateKey string `json:"child_private_key"`

	// ChainCode is the child chain code as lowercase hex.
	ChainCode string `json:"chain_code"`

	// JournalHex is the raw committed journal as lowercase hex.
	JournalHex string `json:"journal_hex"`

	// JournalSizeBytes is the byte length of the committed journal.
	JournalSizeBytes int `json:"journal_size_bytes"`

	// ProofSealBytes is the size of the proof seal in bytes. It is
	// informative metadata rather than a compatibility guarantee.
	ProofSealBytes uint64 `json:"proof_seal_bytes"`

	// ReceiptEncoding names the serialized receipt encoding expected for
	// the receipt artifact.
	ReceiptEncoding string `json:"receipt_encoding"`
}

// HardenedXPrivExecuteReport summarizes the public results of an execute-only
// run of the reduced hardened-xpriv guest.
type HardenedXPrivExecuteReport struct {
	// GuestPath is the guest artifact path used for the run.
	GuestPath string

	// GuestSize is the packaged guest size in bytes.
	GuestSize int

	// ImageID is the computed image ID for the loaded guest.
	ImageID string

	// UsingTestVector reports whether the built-in witness was used.
	UsingTestVector bool

	// Claim is the decoded public claim committed by the guest.
	Claim HardenedXPrivClaim

	// ExitCode is the guest exit code summary from execute-only mode.
	ExitCode string

	// JournalSize is the size of the committed journal in bytes.
	JournalSize int

	// SegmentCount is the number of zkVM segments executed.
	SegmentCount uint32

	// SessionRows is the total row count reported by the session.
	SessionRows uint64
}

// HardenedXPrivProveReport summarizes a prove run and the generated artifacts
// for the reduced hardened-xpriv guest.
type HardenedXPrivProveReport struct {
	// GuestPath is the guest artifact path used for the proof.
	GuestPath string

	// GuestSize is the packaged guest size in bytes.
	GuestSize int

	// ImageID is the computed image ID for the loaded guest.
	ImageID string

	// UsingTestVector reports whether the built-in witness was used.
	UsingTestVector bool

	// Claim is the decoded public claim committed by the guest.
	Claim HardenedXPrivClaim

	// ReceiptOutputPath is where the receipt artifact was written.
	ReceiptOutputPath string

	// ClaimOutputPath is where the readable claim artifact was written.
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

// HardenedXPrivVerifyReport summarizes the result of verifying a stored
// hardened-xpriv receipt.
type HardenedXPrivVerifyReport struct {
	// GuestPath is the guest artifact path used for verification.
	GuestPath string

	// GuestSize is the packaged guest size in bytes.
	GuestSize int

	// ImageID is the computed image ID for the loaded guest.
	ImageID string

	// Claim is the decoded public claim recovered from the receipt.
	Claim HardenedXPrivClaim

	// ClaimInputPath is the optional `claim.json` used for comparison.
	ClaimInputPath string

	// ReceiptInputPath is the serialized receipt that was verified.
	ReceiptInputPath string

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
