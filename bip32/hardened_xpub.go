// hardened_xpub.go implements the claim type for the reduced hardened-xpub
// proof lane. This is the middle-ground variant: it proves a hardened-only
// derivation from a parent xpriv to a child compressed public key. Unlike
// the xpriv variant, the guest must perform at least one EC point
// multiplication (to compute the public key from the derived private key),
// which is why its proving cost (~14s) sits between the xpriv (~2s) and
// full Taproot (~49s) lanes.
//
// The advantage over the xpriv variant is that the public claim reveals
// only the compressed child public key and chain code, not the child
// private key. This is a stronger privacy property for the claim output.
package bip32

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
)

const (
	// HardenedXPubClaimVersion is the current serialized claim version for
	// the parent-xpriv to child-xpub hardened-derivation statement.
	HardenedXPubClaimVersion = 1

	// HardenedXPubClaimSize is the serialized journal size of a hardened
	// xpub claim: 4 (version) + 4 (flags) + 33 (compressed pubkey) +
	// 32 (chain code) = 73 bytes.
	HardenedXPubClaimSize = 73
)

var (
	// ErrInvalidHardenedXPubClaimSize indicates the committed journal has
	// the wrong size for a hardened xpub claim.
	ErrInvalidHardenedXPubClaimSize = errors.New(
		"invalid hardened xpub claim size",
	)
)

// HardenedXPubClaim commits to the public child xpub material derived from
// a private parent xpriv plus one or more hardened child steps.
type HardenedXPubClaim struct {
	// Version identifies the serialized claim format.
	Version uint32

	// Flags records verifier-visible policy bits. Version 1 uses zero.
	Flags uint32

	// CompressedPubKey is the final compressed secp256k1 child public key.
	CompressedPubKey [33]byte

	// ChainCode is the final 32-byte BIP-32 child chain code.
	ChainCode [32]byte
}

// DeriveHardenedXPubClaim derives a hardened-only relative child path from
// the supplied parent xpriv and returns the resulting public claim.
func DeriveHardenedXPubClaim(
	parent ExtendedPrivateKey, path []uint32,
) (HardenedXPubClaim, error) {

	child, err := DeriveHardenedRelativeExtendedPrivateKey(parent, path)
	if err != nil {
		return HardenedXPubClaim{}, err
	}

	var claim HardenedXPubClaim
	claim.Version = HardenedXPubClaimVersion
	claim.Flags = 0
	claim.CompressedPubKey = child.SerializeCompressedPubKey()
	claim.ChainCode = child.ChainCode()

	return claim, nil
}

// Encode serializes the hardened xpub claim into the guest journal format.
func (claim HardenedXPubClaim) Encode() []byte {
	out := make([]byte, HardenedXPubClaimSize)
	binary.LittleEndian.PutUint32(out[0:4], claim.Version)
	binary.LittleEndian.PutUint32(out[4:8], claim.Flags)
	copy(out[8:41], claim.CompressedPubKey[:])
	copy(out[41:73], claim.ChainCode[:])

	return out
}

// DecodeHardenedXPubClaim parses the committed journal bytes into a structured
// hardened xpub claim.
func DecodeHardenedXPubClaim(data []byte) (HardenedXPubClaim, error) {
	if len(data) != HardenedXPubClaimSize {
		return HardenedXPubClaim{}, ErrInvalidHardenedXPubClaimSize
	}

	var claim HardenedXPubClaim
	claim.Version = binary.LittleEndian.Uint32(data[0:4])
	claim.Flags = binary.LittleEndian.Uint32(data[4:8])
	copy(claim.CompressedPubKey[:], data[8:41])
	copy(claim.ChainCode[:], data[41:73])

	return claim, nil
}

// CompressedPubKeyHex returns the compressed child xpub key as lowercase hex.
func (claim HardenedXPubClaim) CompressedPubKeyHex() string {
	return hex.EncodeToString(claim.CompressedPubKey[:])
}

// ChainCodeHex returns the child chain code as lowercase hex.
func (claim HardenedXPubClaim) ChainCodeHex() string {
	return hex.EncodeToString(claim.ChainCode[:])
}
