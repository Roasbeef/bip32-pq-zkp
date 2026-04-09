package bip32

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
)

const (
	// HardenedXPrivClaimVersion is the current serialized claim version for
	// the parent-xpriv to child-xpriv single hardened-step statement.
	HardenedXPrivClaimVersion = 1

	// HardenedXPrivClaimSize is the serialized journal size of a hardened
	// xpriv claim.
	HardenedXPrivClaimSize = 72
)

var (
	// ErrInvalidHardenedXPrivClaimSize indicates the committed journal has
	// the wrong size for a hardened xpriv claim.
	ErrInvalidHardenedXPrivClaimSize = errors.New(
		"invalid hardened xpriv claim size",
	)

	// ErrExpectedSingleHardenedStep indicates the caller supplied something
	// other than exactly one hardened child step.
	ErrExpectedSingleHardenedStep = errors.New(
		"expected exactly one hardened child step",
	)
)

// HardenedXPrivClaim commits to the child xpriv material derived from a
// private parent xpriv via exactly one hardened child step.
type HardenedXPrivClaim struct {
	// Version identifies the serialized claim format.
	Version uint32

	// Flags records verifier-visible policy bits. Version 1 uses zero.
	Flags uint32

	// ChildPrivateKey is the final 32-byte child secret scalar.
	ChildPrivateKey [32]byte

	// ChainCode is the final 32-byte BIP-32 child chain code.
	ChainCode [32]byte
}

// DeriveHardenedXPrivClaim derives exactly one hardened child step from the
// supplied parent xpriv and returns the resulting child xpriv claim.
func DeriveHardenedXPrivClaim(
	parent ExtendedPrivateKey, path []uint32,
) (HardenedXPrivClaim, error) {

	index, err := SingleHardenedChild(path)
	if err != nil {
		return HardenedXPrivClaim{}, err
	}

	child, err := DeriveHardenedChildExtendedPrivateKey(parent, index)
	if err != nil {
		return HardenedXPrivClaim{}, err
	}

	var claim HardenedXPrivClaim
	claim.Version = HardenedXPrivClaimVersion
	claim.Flags = 0
	claim.ChildPrivateKey = child.SerializePrivateKey()
	claim.ChainCode = child.ChainCode()

	return claim, nil
}

// Encode serializes the hardened xpriv claim into the guest journal format.
func (claim HardenedXPrivClaim) Encode() []byte {
	out := make([]byte, HardenedXPrivClaimSize)
	binary.LittleEndian.PutUint32(out[0:4], claim.Version)
	binary.LittleEndian.PutUint32(out[4:8], claim.Flags)
	copy(out[8:40], claim.ChildPrivateKey[:])
	copy(out[40:72], claim.ChainCode[:])

	return out
}

// DecodeHardenedXPrivClaim parses the committed journal bytes into a
// structured hardened xpriv claim.
func DecodeHardenedXPrivClaim(data []byte) (HardenedXPrivClaim, error) {
	if len(data) != HardenedXPrivClaimSize {
		return HardenedXPrivClaim{}, ErrInvalidHardenedXPrivClaimSize
	}

	var claim HardenedXPrivClaim
	claim.Version = binary.LittleEndian.Uint32(data[0:4])
	claim.Flags = binary.LittleEndian.Uint32(data[4:8])
	copy(claim.ChildPrivateKey[:], data[8:40])
	copy(claim.ChainCode[:], data[40:72])

	return claim, nil
}

// ChildPrivateKeyHex returns the derived child xpriv bytes as lowercase hex.
func (claim HardenedXPrivClaim) ChildPrivateKeyHex() string {
	return hex.EncodeToString(claim.ChildPrivateKey[:])
}

// ChainCodeHex returns the child chain code as lowercase hex.
func (claim HardenedXPrivClaim) ChainCodeHex() string {
	return hex.EncodeToString(claim.ChainCode[:])
}

// SingleHardenedChild validates that the supplied path consists of exactly
// one hardened child index and returns that index.
func SingleHardenedChild(path []uint32) (uint32, error) {
	switch {
	case len(path) != 1:
		return 0, ErrExpectedSingleHardenedStep

	case path[0] < HardenedKeyStart:
		return 0, ErrNonHardenedPath
	}

	return path[0], nil
}
