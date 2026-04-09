// extended_key_test.go cross-checks the bip32.ExtendedPrivateKey derivation
// helpers and both reduced claim types against the btcsuite/hdkeychain
// reference implementation. This ensures the minimal derivation code used
// inside the zkVM guest produces semantically identical results to a
// full-featured BIP-32 library, which is critical since the guest runs in a
// deterministic environment with no way to inspect intermediate state.
package hostcheck

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	localbip32 "github.com/roasbeef/bip32-pq-zkp/bip32"
)

// deriveReferenceExtendedKey derives a reference BIP-32 extended private key
// with btcsuite/hdkeychain so the local minimal helpers can be checked against
// a known-good implementation.
func deriveReferenceExtendedKey(
	t *testing.T, seed []byte, path []uint32,
) *hdkeychain.ExtendedKey {

	t.Helper()

	key, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("hdkeychain.NewMaster failed: %v", err)
	}

	for _, index := range path {
		key, err = key.Derive(index)
		if err != nil {
			t.Fatalf("hdkeychain.Derive(%d) failed: %v", index, err)
		}
	}

	return key
}

func TestDeriveExtendedPrivateKeyMatchesHDKeychain(t *testing.T) {
	seed, path := testVector()
	parentPath := path[:2]
	childPath := path[2:3]

	parent, err := localbip32.DeriveExtendedPrivateKey(seed, parentPath)
	if err != nil {
		t.Fatalf("DeriveExtendedPrivateKey(parent) failed: %v", err)
	}

	child, err := localbip32.DeriveHardenedRelativeExtendedPrivateKey(
		parent, childPath,
	)
	if err != nil {
		t.Fatalf(
			"DeriveHardenedRelativeExtendedPrivateKey failed: %v",
			err,
		)
	}

	want := deriveReferenceExtendedKey(t, seed, path[:3])
	wantPriv, err := want.ECPrivKey()
	if err != nil {
		t.Fatalf("hdkeychain.ECPrivKey failed: %v", err)
	}

	gotPriv := child.SerializePrivateKey()
	if !bytes.Equal(gotPriv[:], wantPriv.Serialize()) {
		t.Fatalf("private key mismatch: got %x want %x",
			gotPriv[:], wantPriv.Serialize())
	}

	wantChainCode := want.ChainCode()
	gotChainCode := child.ChainCode()
	if !bytes.Equal(gotChainCode[:], wantChainCode) {
		t.Fatalf("chain code mismatch: got %x want %x",
			gotChainCode[:], wantChainCode)
	}

	wantPub, err := want.ECPubKey()
	if err != nil {
		t.Fatalf("hdkeychain.ECPubKey failed: %v", err)
	}

	gotPub := child.SerializeCompressedPubKey()
	if !bytes.Equal(gotPub[:], wantPub.SerializeCompressed()) {
		t.Fatalf("compressed pubkey mismatch: got %x want %x",
			gotPub[:], wantPub.SerializeCompressed())
	}
}

func TestValidateAllHardenedRejectsNormalChild(t *testing.T) {
	err := localbip32.ValidateAllHardened([]uint32{
		localbip32.HardenedKeyStart,
		1,
	})
	if err != localbip32.ErrNonHardenedPath {
		t.Fatalf("unexpected error: got %v want %v",
			err, localbip32.ErrNonHardenedPath)
	}
}

func TestDeriveHardenedXPubClaimMatchesHDKeychain(t *testing.T) {
	seed, path := testVector()
	parentPath := path[:2]
	childPath := path[2:3]

	parent, err := localbip32.DeriveExtendedPrivateKey(seed, parentPath)
	if err != nil {
		t.Fatalf("DeriveExtendedPrivateKey(parent) failed: %v", err)
	}

	claim, err := localbip32.DeriveHardenedXPubClaim(parent, childPath)
	if err != nil {
		t.Fatalf("DeriveHardenedXPubClaim failed: %v", err)
	}

	want := deriveReferenceExtendedKey(t, seed, path[:3])
	wantPub, err := want.ECPubKey()
	if err != nil {
		t.Fatalf("hdkeychain.ECPubKey failed: %v", err)
	}

	if !bytes.Equal(
		claim.CompressedPubKey[:], wantPub.SerializeCompressed(),
	) {

		t.Fatalf(
			"compressed pubkey mismatch: got %x want %x",
			claim.CompressedPubKey[:],
			wantPub.SerializeCompressed(),
		)
	}

	wantChainCode := want.ChainCode()
	if !bytes.Equal(claim.ChainCode[:], wantChainCode) {
		t.Fatalf(
			"chain code mismatch: got %x want %x",
			claim.ChainCode[:], wantChainCode,
		)
	}
}

func TestDeriveHardenedXPrivClaimMatchesHDKeychain(t *testing.T) {
	seed, path := testVector()
	parentPath := path[:2]
	childPath := path[2:3]

	parent, err := localbip32.DeriveExtendedPrivateKey(seed, parentPath)
	if err != nil {
		t.Fatalf("DeriveExtendedPrivateKey(parent) failed: %v", err)
	}

	claim, err := localbip32.DeriveHardenedXPrivClaim(parent, childPath)
	if err != nil {
		t.Fatalf("DeriveHardenedXPrivClaim failed: %v", err)
	}

	want := deriveReferenceExtendedKey(t, seed, path[:3])
	wantPriv, err := want.ECPrivKey()
	if err != nil {
		t.Fatalf("hdkeychain.ECPrivKey failed: %v", err)
	}

	if !bytes.Equal(claim.ChildPrivateKey[:], wantPriv.Serialize()) {
		t.Fatalf(
			"child private key mismatch: got %x want %x",
			claim.ChildPrivateKey[:], wantPriv.Serialize(),
		)
	}

	wantChainCode := want.ChainCode()
	if !bytes.Equal(claim.ChainCode[:], wantChainCode) {
		t.Fatalf(
			"chain code mismatch: got %x want %x",
			claim.ChainCode[:], wantChainCode,
		)
	}
}
