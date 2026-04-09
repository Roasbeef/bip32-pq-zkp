// hardened_xpriv_claim_support.go provides journal decoding, claim.json
// I/O, and field-level verification helpers for the reduced hardened-xpriv
// lane. The verification model is the same as the full Taproot lane: the
// verifier first checks the receipt against the guest image ID, then
// compares the verified journal output to either a stored claim.json
// artifact or explicit field expectations.
//
// NOTE: Like the full Taproot lane, ProofSealBytes is excluded from the
// semantic claim comparison because changing RECEIPT_KIND changes the seal
// size without changing the public claim semantics.
package bip32pqzkp

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	localbip32 "github.com/roasbeef/bip32-pq-zkp/bip32"
)

// DecodeHardenedXPrivClaim decodes the structured journal format committed by
// the hardened-xpriv guest.
func DecodeHardenedXPrivClaim(
	journal []byte,
) (HardenedXPrivClaim, error) {

	claim, err := localbip32.DecodeHardenedXPrivClaim(journal)
	if err != nil {
		return HardenedXPrivClaim{}, fmt.Errorf(
			"decode hardened xpriv claim: %w", err,
		)
	}

	return claim, nil
}

// NewHardenedXPrivClaimFile converts the verified journal into the canonical
// human-readable `claim.json` verifier artifact for the hardened-xpriv guest.
func NewHardenedXPrivClaimFile(
	imageID string, claim HardenedXPrivClaim, journal []byte,
	sealBytes uint64, receiptEncoding string,
) HardenedXPrivClaimFile {

	return HardenedXPrivClaimFile{
		SchemaVersion:    1,
		ImageID:          imageID,
		ClaimVersion:     claim.Version,
		ClaimFlags:       claim.Flags,
		ChildPrivateKey:  claim.ChildPrivateKeyHex(),
		ChainCode:        claim.ChainCodeHex(),
		JournalHex:       hex.EncodeToString(journal),
		JournalSizeBytes: len(journal),
		ProofSealBytes:   sealBytes,
		ReceiptEncoding:  receiptEncoding,
	}
}

// ReadHardenedXPrivClaimFile loads a previously written hardened-xpriv
// `claim.json` verifier artifact.
func ReadHardenedXPrivClaimFile(path string) (HardenedXPrivClaimFile, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return HardenedXPrivClaimFile{}, fmt.Errorf(
			"read claim `%s`: %w", path, err,
		)
	}

	var claim HardenedXPrivClaimFile
	if err := json.Unmarshal(bytes, &claim); err != nil {
		return HardenedXPrivClaimFile{}, fmt.Errorf(
			"deserialize hardened xpriv claim JSON: %w", err,
		)
	}

	return claim, nil
}

// WriteHardenedXPrivClaimFile writes the hardened-xpriv verifier artifact to
// disk.
func WriteHardenedXPrivClaimFile(
	path string, claim HardenedXPrivClaimFile,
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
	if err := encoder.Encode(claim); err != nil {
		return fmt.Errorf(
			"serialize hardened xpriv claim JSON: %w", err,
		)
	}

	return nil
}

func verifyHardenedXPrivClaimFileMatches(
	expected HardenedXPrivClaimFile, verified HardenedXPrivClaimFile,
) error {

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
		expected.ChildPrivateKey == verified.ChildPrivateKey &&
		expected.ChainCode == verified.ChainCode &&
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

func verifyHardenedXPrivExpectations(
	expectations HardenedXPrivVerifyExpectations, claim HardenedXPrivClaim,
) error {

	if expectations.ChildPrivateKeyHex != "" {
		expected, err := decodeHexArray32(
			"--expected-child-private-key",
			expectations.ChildPrivateKeyHex,
		)
		if err != nil {
			return err
		}

		if claim.ChildPrivateKey != expected {
			return fmt.Errorf(
				"child private key mismatch: "+
					"receipt has %s, expected %s",
				claim.ChildPrivateKeyHex(),
				hex.EncodeToString(expected[:]),
			)
		}
	}

	if expectations.ChainCodeHex != "" {
		expected, err := decodeHexArray32(
			"--expected-chain-code", expectations.ChainCodeHex,
		)
		if err != nil {
			return err
		}

		if claim.ChainCode != expected {
			return fmt.Errorf(
				"chain code mismatch: "+
					"receipt has %s, expected %s",
				claim.ChainCodeHex(),
				hex.EncodeToString(expected[:]),
			)
		}
	}

	return nil
}
