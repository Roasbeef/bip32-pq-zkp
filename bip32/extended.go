// extended.go provides the ExtendedPrivateKey type used by all three proof
// lanes. The full Taproot lane originally inlined its own master-key and
// path-walk logic in bip32.go; the refactored code now routes through these
// helpers so the full lane and the reduced hardened variants share a single
// BIP-32 derivation core.
//
// The key design choice is that ExtendedPrivateKey carries only the private
// scalar and the chain code -- no depth, fingerprint, or child index. Those
// fields are part of the BIP-32 serialization format (xprv/xpub strings) but
// are not needed for the derivation arithmetic itself. Keeping the type
// minimal reduces the guest-side witness size and avoids unnecessary
// serialization work inside the zkVM.
package bip32

import (
	"errors"

	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
)

var (
	// ErrInvalidExtendedPrivateKey indicates the serialized
	// private-key bytes do not represent a valid secp256k1 secret scalar.
	// This fires when the 32-byte key overflows the secp256k1 group order
	// or is the zero scalar.
	ErrInvalidExtendedPrivateKey = errors.New(
		"invalid extended private key",
	)

	// ErrNonHardenedPath indicates a path contained at least one
	// non-hardened component when a hardened-only derivation was required.
	// The reduced proof variants enforce hardened-only paths because
	// non-hardened child derivation leaks the parent public key in the
	// HMAC input, which weakens the post-quantum security argument.
	ErrNonHardenedPath = errors.New(
		"non-hardened child in hardened-only path",
	)
)

// ExtendedPrivateKey is the minimal BIP-32 extended private key representation
// needed by all three proof lanes. It pairs a secp256k1 private scalar with
// the 32-byte BIP-32 chain code. The chain code acts as entropy for the
// HMAC-SHA512 child derivation step and must be carried alongside the key
// through every derivation level.
type ExtendedPrivateKey struct {
	// key is the secp256k1 secret scalar in Montgomery form. Using
	// ModNScalar rather than raw bytes ensures the scalar is always
	// reduced modulo the group order.
	key secp.ModNScalar

	// chainCode is the 32-byte BIP-32 chain code used as the HMAC key
	// during child derivation.
	chainCode [32]byte
}

// NewExtendedPrivateKey constructs an ExtendedPrivateKey from serialized
// private-key bytes plus a chain code. The key bytes must encode a valid,
// non-zero secp256k1 secret scalar.
//
// This is the constructor used by both the host-side witness builder (when
// assembling explicit parent xpriv material) and internally by the
// master-key derivation path. The overflow check catches the rare case
// where the 32-byte value exceeds the secp256k1 group order n.
func NewExtendedPrivateKey(
	privateKey [32]byte, chainCode [32]byte,
) (ExtendedPrivateKey, error) {

	var (
		key      secp.ModNScalar
		overflow uint32
	)

	// SetBytes interprets the 32-byte big-endian value as a scalar and
	// returns 1 if the value overflowed the group order. Both overflow
	// and zero are invalid BIP-32 keys.
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
// from a high-entropy seed and returns the resulting extended private key.
// This is the very first step that makes BIP-32 post-quantum interesting:
// the seed passes through HMAC-SHA512 (a symmetric primitive immune to
// Shor's algorithm), so even a quantum adversary who breaks the child
// public key cannot invert this step to recover the seed.
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
// child path from the supplied parent extended private key. Hardened-only
// derivation is the basis of the reduced proof variants: it keeps the HMAC
// input dependent on the private key at every step, so a quantum adversary
// who can compute EC discrete logs still cannot forge the derivation chain
// without knowing the parent secret material.
func DeriveHardenedRelativeExtendedPrivateKey(
	parent ExtendedPrivateKey, path []uint32,
) (ExtendedPrivateKey, error) {

	if err := ValidateAllHardened(path); err != nil {
		return ExtendedPrivateKey{}, err
	}

	return DeriveRelativeExtendedPrivateKey(parent, path)
}

// ValidateAllHardened ensures every path component is in the hardened child
// range (index >= 0x80000000). This is a precondition for the reduced
// proof variants that must avoid non-hardened derivation.
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

// SerializeCompressedPubKey returns the 33-byte SEC compressed secp256k1
// public key. This is the format used by the hardened-xpub claim's public
// output. It includes the 0x02/0x03 parity prefix.
func (xpriv ExtendedPrivateKey) SerializeCompressedPubKey() [33]byte {
	var out [33]byte
	copy(out[:], xpriv.PrivateKey().PubKey().SerializeCompressed())
	return out
}

// SerializeXOnlyPubKey returns the 32-byte BIP-340 x-only public key.
// This is the format used by the full Taproot lane, where the key is
// further tweaked by the BIP-341 tagged hash.
func (xpriv ExtendedPrivateKey) SerializeXOnlyPubKey() [32]byte {
	var out [32]byte
	copy(out[:], serializeXOnly(xpriv.PrivateKey().PubKey()))
	return out
}
