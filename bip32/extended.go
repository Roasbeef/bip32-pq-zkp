package bip32

import (
	"errors"

	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
)

var (
	// ErrInvalidExtendedPrivateKey indicates the serialized
	// private-key bytes do not represent a valid secp256k1 secret scalar.
	ErrInvalidExtendedPrivateKey = errors.New(
		"invalid extended private key",
	)

	// ErrNonHardenedPath indicates a path contained at least one
	// non-hardened component when a hardened-only derivation was required.
	ErrNonHardenedPath = errors.New(
		"non-hardened child in hardened-only path",
	)
)

// ExtendedPrivateKey is the minimal BIP-32 extended private key representation
// needed by the reduced proof variants. It contains a secp256k1 private scalar
// plus the 32-byte BIP-32 chain code.
type ExtendedPrivateKey struct {
	key       secp.ModNScalar
	chainCode [32]byte
}

// NewExtendedPrivateKey constructs an ExtendedPrivateKey from serialized
// private-key bytes plus a chain code. The key bytes must encode a valid,
// non-zero secp256k1 secret scalar.
func NewExtendedPrivateKey(
	privateKey [32]byte, chainCode [32]byte,
) (ExtendedPrivateKey, error) {

	var (
		key      secp.ModNScalar
		overflow uint32
	)
	overflow = key.SetBytes(&privateKey)
	if overflow != 0 || key.IsZero() {
		return ExtendedPrivateKey{}, ErrInvalidExtendedPrivateKey
	}

	return ExtendedPrivateKey{
		key:       key,
		chainCode: chainCode,
	}, nil
}

// DeriveMasterExtendedPrivateKey performs the BIP-32 master-key derivation
// from a seed and returns the resulting extended private key.
func DeriveMasterExtendedPrivateKey(seed []byte) (ExtendedPrivateKey, error) {
	if len(seed) < minSeedBytes || len(seed) > maxSeedBytes {
		return ExtendedPrivateKey{}, ErrInvalidSeedLength
	}

	sum := hmacSHA512(masterKeySalt, seed)

	var privateKey [32]byte
	copy(privateKey[:], sum[:32])

	var chainCode [32]byte
	copy(chainCode[:], sum[32:])

	return NewExtendedPrivateKey(privateKey, chainCode)
}

// DeriveExtendedPrivateKey derives the BIP-32 extended private key at the
// given path from the provided master seed.
func DeriveExtendedPrivateKey(
	seed []byte, path []uint32,
) (ExtendedPrivateKey, error) {

	xpriv, err := DeriveMasterExtendedPrivateKey(seed)
	if err != nil {
		return ExtendedPrivateKey{}, err
	}

	return DeriveRelativeExtendedPrivateKey(xpriv, path)
}

// DeriveRelativeExtendedPrivateKey derives a relative child path from the
// supplied parent extended private key.
func DeriveRelativeExtendedPrivateKey(
	parent ExtendedPrivateKey, path []uint32,
) (ExtendedPrivateKey, error) {

	child := parent
	for _, index := range path {
		var err error
		child, err = DeriveChildExtendedPrivateKey(child, index)
		if err != nil {
			return ExtendedPrivateKey{}, err
		}
	}

	return child, nil
}

// DeriveHardenedRelativeExtendedPrivateKey derives a hardened-only relative
// child path from the supplied parent extended private key.
func DeriveHardenedRelativeExtendedPrivateKey(
	parent ExtendedPrivateKey, path []uint32,
) (ExtendedPrivateKey, error) {

	if err := ValidateAllHardened(path); err != nil {
		return ExtendedPrivateKey{}, err
	}

	return DeriveRelativeExtendedPrivateKey(parent, path)
}

// ValidateAllHardened ensures every path component is in the hardened child
// range.
func ValidateAllHardened(path []uint32) error {
	for _, index := range path {
		if index < HardenedKeyStart {
			return ErrNonHardenedPath
		}
	}

	return nil
}

// DeriveChildExtendedPrivateKey derives a single child extended private key
// from the supplied parent extended private key.
func DeriveChildExtendedPrivateKey(
	parent ExtendedPrivateKey, index uint32,
) (ExtendedPrivateKey, error) {

	childKey, childChainCode, err := deriveChild(
		&parent.key, parent.chainCode, index,
	)
	if err != nil {
		return ExtendedPrivateKey{}, err
	}

	return ExtendedPrivateKey{
		key:       childKey,
		chainCode: childChainCode,
	}, nil
}

// DeriveHardenedChildExtendedPrivateKey derives a single hardened child
// extended private key from the supplied parent extended private key.
func DeriveHardenedChildExtendedPrivateKey(
	parent ExtendedPrivateKey, index uint32,
) (ExtendedPrivateKey, error) {

	if index < HardenedKeyStart {
		return ExtendedPrivateKey{}, ErrNonHardenedPath
	}

	return DeriveChildExtendedPrivateKey(parent, index)
}

// PrivateKey returns the child private key as a secp256k1 private key.
func (xpriv ExtendedPrivateKey) PrivateKey() *secp.PrivateKey {
	return secp.NewPrivateKey(&xpriv.key)
}

// ChainCode returns the 32-byte BIP-32 chain code.
func (xpriv ExtendedPrivateKey) ChainCode() [32]byte {
	return xpriv.chainCode
}

// SerializePrivateKey returns the serialized 32-byte private key scalar.
func (xpriv ExtendedPrivateKey) SerializePrivateKey() [32]byte {
	var out [32]byte
	copy(out[:], xpriv.PrivateKey().Serialize())
	return out
}

// SerializeCompressedPubKey returns the compressed secp256k1 public key bytes.
func (xpriv ExtendedPrivateKey) SerializeCompressedPubKey() [33]byte {
	var out [33]byte
	copy(out[:], xpriv.PrivateKey().PubKey().SerializeCompressed())
	return out
}

// SerializeXOnlyPubKey returns the BIP-340 x-only public key bytes.
func (xpriv ExtendedPrivateKey) SerializeXOnlyPubKey() [32]byte {
	var out [32]byte
	copy(out[:], serializeXOnly(xpriv.PrivateKey().PubKey()))
	return out
}
