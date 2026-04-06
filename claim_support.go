package bip32pqzkp

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DecodePublicClaim decodes the structured 72-byte journal format committed by
// the guest.
func DecodePublicClaim(journal []byte) (PublicClaim, error) {
	if len(journal) != publicClaimSize {
		return PublicClaim{}, fmt.Errorf(
			"unexpected journal size: got %d bytes, want %d",
			len(journal), publicClaimSize,
		)
	}

	var claim PublicClaim
	claim.Version = binary.LittleEndian.Uint32(journal[0:4])
	claim.Flags = binary.LittleEndian.Uint32(journal[4:8])
	copy(claim.TaprootOutputKey[:], journal[8:40])
	copy(claim.PathCommitment[:], journal[40:72])

	return claim, nil
}

// RequireBIP86 reports whether the claim's public policy flag requires the
// witness path to satisfy the BIP-86 shape.
func (c PublicClaim) RequireBIP86() bool {
	return c.Flags&witnessFlagRequireBIP86 != 0
}

// TaprootOutputKeyHex returns the x-only Taproot output key as lowercase hex.
func (c PublicClaim) TaprootOutputKeyHex() string {
	return hex.EncodeToString(c.TaprootOutputKey[:])
}

// PathCommitmentHex returns the public path commitment as lowercase hex.
func (c PublicClaim) PathCommitmentHex() string {
	return hex.EncodeToString(c.PathCommitment[:])
}

// NewClaimFile converts the verified journal into the canonical
// human-readable `claim.json` verifier artifact.
func NewClaimFile(imageID string, claim PublicClaim, journal []byte,
	sealBytes uint64, receiptEncoding string) ClaimFile {

	return ClaimFile{
		SchemaVersion:    1,
		ImageID:          imageID,
		ClaimVersion:     claim.Version,
		ClaimFlags:       claim.Flags,
		RequireBIP86:     claim.RequireBIP86(),
		TaprootOutputKey: claim.TaprootOutputKeyHex(),
		PathCommitment:   claim.PathCommitmentHex(),
		JournalHex:       hex.EncodeToString(journal),
		JournalSizeBytes: len(journal),
		ProofSealBytes:   sealBytes,
		ReceiptEncoding:  receiptEncoding,
	}
}

// ReadClaimFile loads a previously written canonical `claim.json` verifier
// artifact.
func ReadClaimFile(path string) (ClaimFile, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return ClaimFile{}, fmt.Errorf("read claim `%s`: %w", path, err)
	}

	var claim ClaimFile
	if err := json.Unmarshal(bytes, &claim); err != nil {
		return ClaimFile{}, fmt.Errorf(
			"deserialize claim JSON: %w", err,
		)
	}

	return claim, nil
}

// WriteClaimFile writes the canonical human-readable `claim.json`
// verifier artifact to disk.
func WriteClaimFile(path string, claim ClaimFile) error {
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
		return fmt.Errorf("serialize claim JSON: %w", err)
	}

	return nil
}

// writeReceipt writes the raw serialized receipt bytes to disk, creating
// parent directories as needed.
func writeReceipt(path string, receipt []byte) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}

	return os.WriteFile(path, receipt, 0o600)
}

// ensureParentDir creates the parent directory of the given path if it does
// not already exist.
func ensureParentDir(path string) error {
	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		return nil
	}

	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf(
			"create parent directory `%s`: %w", parent, err,
		)
	}

	return nil
}

// verifyClaimFileMatches checks that a stored claim.json artifact matches the
// claim decoded from the verified receipt journal. The comparison is semantic:
// it checks all public claim fields plus the journal and receipt encoding, but
// intentionally excludes ProofSealBytes because the seal size may vary across
// prover implementations for the same valid proof.
//
// NOTE: ProofSealBytes is intentionally excluded from the semantic comparison.
func verifyClaimFileMatches(expected ClaimFile, verified ClaimFile) error {
	if expected.ImageID != verified.ImageID {
		return fmt.Errorf(
			"claim image ID mismatch: "+
				"claim says %s, guest computes %s",
			expected.ImageID, verified.ImageID,
		)
	}

	matches := expected.SchemaVersion == verified.SchemaVersion &&
		expected.ImageID == verified.ImageID &&
		expected.ClaimVersion == verified.ClaimVersion &&
		expected.ClaimFlags == verified.ClaimFlags &&
		expected.RequireBIP86 == verified.RequireBIP86 &&
		expected.TaprootOutputKey == verified.TaprootOutputKey &&
		expected.PathCommitment == verified.PathCommitment &&
		expected.JournalHex == verified.JournalHex &&
		expected.JournalSizeBytes == verified.JournalSizeBytes &&
		expected.ReceiptEncoding == verified.ReceiptEncoding

	if !matches {
		return errors.New(
			"claim file does not match the verified public " +
				"receipt output",
		)
	}

	return nil
}

// verifyClaimExpectations checks the decoded public claim against explicit
// per-field expectations provided by the caller via CLI flags.
func verifyClaimExpectations(expectations VerifyExpectations,
	claim PublicClaim) error {

	if expectations.PathCommitmentHex != "" && expectations.Path != "" {
		return errors.New(
			"--expected-path and --expected-path-commitment are " +
				"mutually exclusive",
		)
	}

	if expectations.PubKeyHex != "" {
		expected, err := decodeHexArray32(
			"--expected-pubkey", expectations.PubKeyHex,
		)
		if err != nil {
			return err
		}

		if claim.TaprootOutputKey != expected {
			return fmt.Errorf(
				"taproot output key mismatch: receipt has %s, "+
					"expected %s",
				claim.TaprootOutputKeyHex(),
				hex.EncodeToString(expected[:]),
			)
		}
	}

	if expectations.PathCommitmentHex != "" {
		expected, err := decodeHexArray32(
			"--expected-path-commitment",
			expectations.PathCommitmentHex,
		)
		if err != nil {
			return err
		}

		if claim.PathCommitment != expected {
			return fmt.Errorf(
				"path commitment mismatch: "+
					"receipt has %s, expected "+
					"%s",
				claim.PathCommitmentHex(),
				hex.EncodeToString(expected[:]),
			)
		}
	}

	if expectations.Path != "" {
		path, err := ParseBIP32Path(expectations.Path)
		if err != nil {
			return fmt.Errorf("parse --expected-path: %w", err)
		}

		expected := PathCommitmentFromPath(path)
		if claim.PathCommitment != expected {
			return fmt.Errorf(
				"path commitment mismatch: "+
					"receipt has %s, expected "+
					"commitment from path %s",
				claim.PathCommitmentHex(),
				hex.EncodeToString(expected[:]),
			)
		}
	}

	if expectations.RequireBIP86 != nil &&
		claim.RequireBIP86() != *expectations.RequireBIP86 {

		return fmt.Errorf(
			"claim policy mismatch: "+
				"receipt says require_bip86=%v, "+
				"expected %v",
			claim.RequireBIP86(), *expectations.RequireBIP86,
		)
	}

	return nil
}
