package bip32pqzkp

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	localbip32 "github.com/roasbeef/bip32-pq-zkp/bip32"
)

// DecodeHardenedXPubClaim decodes the structured journal format committed by
// the hardened-xpub guest.
func DecodeHardenedXPubClaim(
	journal []byte,
) (HardenedXPubClaim, error) {

	claim, err := localbip32.DecodeHardenedXPubClaim(journal)
	if err != nil {
		return HardenedXPubClaim{}, fmt.Errorf(
			"decode hardened xpub claim: %w", err,
		)
	}

	return claim, nil
}

// NewHardenedXPubClaimFile converts the verified journal into the canonical
// human-readable `claim.json` verifier artifact for the hardened-xpub guest.
func NewHardenedXPubClaimFile(
	imageID string, claim HardenedXPubClaim, journal []byte,
	sealBytes uint64, receiptEncoding string,
) HardenedXPubClaimFile {

	return HardenedXPubClaimFile{
		SchemaVersion:    1,
		ImageID:          imageID,
		ClaimVersion:     claim.Version,
		ClaimFlags:       claim.Flags,
		CompressedPubKey: claim.CompressedPubKeyHex(),
		ChainCode:        claim.ChainCodeHex(),
		JournalHex:       hex.EncodeToString(journal),
		JournalSizeBytes: len(journal),
		ProofSealBytes:   sealBytes,
		ReceiptEncoding:  receiptEncoding,
	}
}

// ReadHardenedXPubClaimFile loads a previously written hardened-xpub
// `claim.json` verifier artifact.
func ReadHardenedXPubClaimFile(path string) (HardenedXPubClaimFile, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return HardenedXPubClaimFile{}, fmt.Errorf(
			"read claim `%s`: %w", path, err,
		)
	}

	var claim HardenedXPubClaimFile
	if err := json.Unmarshal(bytes, &claim); err != nil {
		return HardenedXPubClaimFile{}, fmt.Errorf(
			"deserialize hardened xpub claim JSON: %w", err,
		)
	}

	return claim, nil
}

// WriteHardenedXPubClaimFile writes the hardened-xpub verifier artifact to
// disk.
func WriteHardenedXPubClaimFile(
	path string, claim HardenedXPubClaimFile,
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
			"serialize hardened xpub claim JSON: %w", err,
		)
	}

	return nil
}

func verifyHardenedXPubClaimFileMatches(
	expected HardenedXPubClaimFile, verified HardenedXPubClaimFile,
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
		expected.CompressedPubKey == verified.CompressedPubKey &&
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

func verifyHardenedXPubExpectations(
	expectations HardenedXPubVerifyExpectations, claim HardenedXPubClaim,
) error {

	if expectations.CompressedPubKeyHex != "" {
		expected, err := decodeHexArray33(
			"--expected-compressed-pubkey",
			expectations.CompressedPubKeyHex,
		)
		if err != nil {
			return err
		}

		if claim.CompressedPubKey != expected {
			return fmt.Errorf(
				"compressed pubkey mismatch: "+
					"receipt has %s, expected %s",
				claim.CompressedPubKeyHex(),
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
