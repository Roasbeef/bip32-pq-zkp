package bip32pqzkp

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func testClaimFixture(t *testing.T) (PublicClaim, []byte) {
	t.Helper()

	taprootKey, err := decodeHexArray32(
		"taproot_output_key",
		"00324bf6fa47a8d70cb5519957dd54a02"+
			"b385c0ead8e4f92f9f07f992b288ee6",
	)
	if err != nil {
		t.Fatalf("decode taproot output key: %v", err)
	}

	pathCommitment, err := decodeHexArray32(
		"path_commitment",
		"4c7de33d397de2c231e7c2a7f53e5b581"+
			"ee3c20073ea79ee4afaab56de11f74b",
	)
	if err != nil {
		t.Fatalf("decode path commitment: %v", err)
	}

	claim := PublicClaim{
		Version:          PublicClaimVersion,
		Flags:            witnessFlagRequireBIP86,
		TaprootOutputKey: taprootKey,
		PathCommitment:   pathCommitment,
	}

	journal := make([]byte, publicClaimSize)
	binary.LittleEndian.PutUint32(journal[0:4], claim.Version)
	binary.LittleEndian.PutUint32(journal[4:8], claim.Flags)
	copy(journal[8:40], claim.TaprootOutputKey[:])
	copy(journal[40:72], claim.PathCommitment[:])

	return claim, journal
}

func TestNewClaimFileSchemaV1(t *testing.T) {
	claim, journal := testClaimFixture(t)

	claimFile := NewClaimFile(
		"8a6a2c27dd54d8fa0f99a332b57cb105"+
			"f88472d977c84bfac077cbe70907a690",
		claim,
		journal,
		1797880,
		"borsh",
	)

	if claimFile.SchemaVersion != 1 {
		t.Fatalf("unexpected schema version: got %d want 1",
			claimFile.SchemaVersion)
	}
	if claimFile.ClaimVersion != PublicClaimVersion {
		t.Fatalf("unexpected claim version: got %d want %d",
			claimFile.ClaimVersion, PublicClaimVersion)
	}
	if claimFile.ClaimFlags != witnessFlagRequireBIP86 {
		t.Fatalf("unexpected claim flags: got %d want %d",
			claimFile.ClaimFlags, witnessFlagRequireBIP86)
	}
	if !claimFile.RequireBIP86 {
		t.Fatal("expected require_bip86=true")
	}
	if claimFile.TaprootOutputKey !=
		hex.EncodeToString(claim.TaprootOutputKey[:]) {

		t.Fatalf("unexpected taproot output key: got %s want %s",
			claimFile.TaprootOutputKey,
			hex.EncodeToString(claim.TaprootOutputKey[:]))
	}
	if claimFile.PathCommitment !=
		hex.EncodeToString(claim.PathCommitment[:]) {

		t.Fatalf("unexpected path commitment: got %s want %s",
			claimFile.PathCommitment,
			hex.EncodeToString(claim.PathCommitment[:]))
	}
	if claimFile.JournalHex != hex.EncodeToString(journal) {
		t.Fatalf("unexpected journal hex: got %s want %s",
			claimFile.JournalHex, hex.EncodeToString(journal))
	}
	if claimFile.JournalSizeBytes != len(journal) {
		t.Fatalf("unexpected journal size: got %d want %d",
			claimFile.JournalSizeBytes, len(journal))
	}
	if claimFile.ReceiptEncoding != "borsh" {
		t.Fatalf("unexpected receipt encoding: got %s want borsh",
			claimFile.ReceiptEncoding)
	}

	encoded, err := json.Marshal(claimFile)
	if err != nil {
		t.Fatalf("marshal claim file: %v", err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode claim file JSON: %v", err)
	}

	requiredFields := []string{
		"schema_version",
		"image_id",
		"claim_version",
		"claim_flags",
		"require_bip86",
		"taproot_output_key",
		"path_commitment",
		"journal_hex",
		"journal_size_bytes",
		"proof_seal_bytes",
		"receipt_encoding",
	}
	for _, field := range requiredFields {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("missing required claim.json field %q", field)
		}
	}
}

func TestVerifyClaimFileMatchesIgnoresInformationalProofSealSize(t *testing.T) {
	claim, journal := testClaimFixture(t)

	expected := NewClaimFile(
		"8a6a2c27dd54d8fa0f99a332b57cb105"+
			"f88472d977c84bfac077cbe70907a690",
		claim,
		journal,
		111,
		"borsh",
	)
	verified := expected
	verified.ProofSealBytes = 222

	if err := verifyClaimFileMatches(expected, verified); err != nil {
		t.Fatalf(
			"claim comparison should ignore proof_seal_bytes: %v",
			err,
		)
	}
}

func TestVerifyClaimFileMatchesRejectsSemanticMismatch(t *testing.T) {
	claim, journal := testClaimFixture(t)

	expected := NewClaimFile(
		"8a6a2c27dd54d8fa0f99a332b57cb105"+
			"f88472d977c84bfac077cbe70907a690",
		claim,
		journal,
		111,
		"borsh",
	)
	verified := expected
	verified.TaprootOutputKey =
		"ffffffffffffffffffffffffffffffff" +
			"ffffffffffffffffffffffffffffffff"

	if err := verifyClaimFileMatches(expected, verified); err == nil {
		t.Fatal("expected claim comparison to reject semantic mismatch")
	}
}
