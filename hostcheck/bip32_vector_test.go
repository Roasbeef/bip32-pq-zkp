// Package hostcheck validates the local bip32 derivation helpers against
// btcd/txscript as the reference implementation. These tests ensure that the
// minimal BIP-32 and Taproot helpers used by the guest produce identical
// results to the full btcd implementation.
package hostcheck

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/txscript"
	"github.com/roasbeef/bip32-pq-zkp/bip32"
)

// testVector returns the built-in BIP-32 test vector 1 seed (16 bytes) and
// the BIP-86 derivation path m/86'/0'/0'/0/0 used throughout the demo.
func testVector() ([]byte, []uint32) {
	seed := []byte{
		0x00, 0x01, 0x02, 0x03,
		0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b,
		0x0c, 0x0d, 0x0e, 0x0f,
	}
	path := []uint32{
		bip32.HardenedKeyStart + 86,
		bip32.HardenedKeyStart + 0,
		bip32.HardenedKeyStart + 0,
		0,
		0,
	}
	return seed, path
}

func TestDeriveXOnlyMatchesKnownInternalKey(t *testing.T) {
	seed, path := testVector()

	xOnly, err := bip32.DeriveXOnly(seed, path)
	if err != nil {
		t.Fatalf("DeriveXOnly failed: %v", err)
	}

	got := hex.EncodeToString(xOnly)
	const want = "" +
		"8724f200544f593846c3a868faa13cfc" +
		"47fb29842e010758c0b95e6f79896434"
	if got != want {
		t.Fatalf("unexpected x-only key: got %s want %s", got, want)
	}
}

func TestDeriveTaprootOutputKeyMatchesTxscriptReference(t *testing.T) {
	seed, path := testVector()

	got, err := bip32.DeriveTaprootOutputKey(seed, path)
	if err != nil {
		t.Fatalf("DeriveTaprootOutputKey failed: %v", err)
	}

	privKey, err := bip32.DerivePrivateKey(seed, path)
	if err != nil {
		t.Fatalf("DerivePrivateKey failed: %v", err)
	}

	refPrivKey, _ := btcec.PrivKeyFromBytes(privKey.Serialize())
	want := schnorr.SerializePubKey(
		txscript.ComputeTaprootKeyNoScript(refPrivKey.PubKey()),
	)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected taproot output key: got %s want %s",
			hex.EncodeToString(got), hex.EncodeToString(want))
	}

	const wantHex = "" +
		"00324bf6fa47a8d70cb5519957dd54a02" +
		"b385c0ead8e4f92f9f07f992b288ee6"
	if gotHex := hex.EncodeToString(got); gotHex != wantHex {
		t.Fatalf("unexpected taproot output key hex: got %s want %s",
			gotHex, wantHex)
	}
}

func TestDeriveTaprootClaimMatchesExpectedPublicMaterial(t *testing.T) {
	seed, path := testVector()

	claim, err := bip32.DeriveTaprootClaim(
		seed, path, bip32.WithBIP86PathVerification(),
	)
	if err != nil {
		t.Fatalf("DeriveTaprootClaim failed: %v", err)
	}

	if claim.Version != bip32.ClaimVersion {
		t.Fatalf(
			"unexpected claim version: got %d want %d",
			claim.Version, bip32.ClaimVersion,
		)
	}
	if claim.Flags != bip32.ClaimFlagRequireBIP86 {
		t.Fatalf(
			"unexpected claim flags: got %d want %d",
			claim.Flags, bip32.ClaimFlagRequireBIP86,
		)
	}

	const wantTaprootKey = "" +
		"00324bf6fa47a8d70cb5519957dd54a02" +
		"b385c0ead8e4f92f9f07f992b288ee6"
	gotTaprootKey := hex.EncodeToString(claim.TaprootOutputKey[:])
	if gotTaprootKey != wantTaprootKey {
		t.Fatalf(
			"unexpected taproot output key in claim: "+
				"got %s want %s",
			gotTaprootKey, wantTaprootKey,
		)
	}

	expectedCommitment := bip32.CommitPath(path)
	if claim.PathCommitment != expectedCommitment {
		t.Fatalf("unexpected path commitment: got %s want %s",
			hex.EncodeToString(claim.PathCommitment[:]),
			hex.EncodeToString(expectedCommitment[:]),
		)
	}

	encoded := claim.Encode()
	decoded, err := bip32.DecodePublicClaim(encoded)
	if err != nil {
		t.Fatalf("DecodePublicClaim failed: %v", err)
	}
	if decoded != claim {
		t.Fatalf(
			"claim round-trip mismatch: got %+v want %+v",
			decoded, claim,
		)
	}
}
