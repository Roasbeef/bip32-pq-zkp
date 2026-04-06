package bip32

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"errors"

	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
)

const (
	// HardenedKeyStart is the BIP-32 bit used to mark hardened children.
	// Any child index >= this value triggers the hardened derivation path,
	// which uses the parent private key directly instead of the public key.
	HardenedKeyStart = 0x80000000

	// minSeedBytes is the minimum BIP-32 seed length (128 bits).
	minSeedBytes = 16

	// maxSeedBytes is the maximum BIP-32 seed length (512 bits).
	maxSeedBytes = 64
)

var (
	// ErrInvalidSeedLength indicates the seed is outside the BIP-32 bounds.
	ErrInvalidSeedLength = errors.New("invalid seed length")

	// ErrInvalidMasterKey indicates the seed-derived master key was
	// invalid.
	ErrInvalidMasterKey = errors.New("invalid master key")

	// ErrInvalidChildKey indicates a child derivation produced an
	// invalid key.
	ErrInvalidChildKey = errors.New("invalid child key")

	// masterKeySalt is the HMAC-SHA512 key used by BIP-32 to derive the
	// master extended private key from a seed. The BIP-32 spec hardcodes
	// this as the ASCII string "Bitcoin seed".
	masterKeySalt = []byte("Bitcoin seed")
)

// DeriveXOnly derives the BIP-32 child private key at the given path and
// returns the corresponding x-only public key bytes.
func DeriveXOnly(seed []byte, path []uint32) ([]byte, error) {
	privKey, err := DerivePrivateKey(seed, path)
	if err != nil {
		return nil, err
	}

	return serializeXOnly(privKey.PubKey()), nil
}

// DerivePrivateKey derives the BIP-32 child private key at the given path.
// This is a minimal subset intended for zkVM-friendly environments and only
// supports private derivation.
//
// The derivation follows BIP-32 exactly:
//  1. Compute HMAC-SHA512(key="Bitcoin seed", data=seed) to get the master
//     key (left 32 bytes) and master chain code (right 32 bytes).
//  2. For each index in the path, apply CKDpriv to derive the child key.
func DerivePrivateKey(seed []byte, path []uint32) (*secp.PrivateKey, error) {
	if len(seed) < minSeedBytes || len(seed) > maxSeedBytes {
		return nil, ErrInvalidSeedLength
	}

	// BIP-32 master key generation: HMAC-SHA512 keyed by "Bitcoin seed".
	// The left 32 bytes become the master private key scalar, the right
	// 32 bytes become the master chain code.
	sum := hmacSHA512(masterKeySalt, seed)

	var key secp.ModNScalar
	if overflow := key.SetByteSlice(sum[:32]); overflow || key.IsZero() {
		return nil, ErrInvalidMasterKey
	}

	var chainCode [32]byte
	copy(chainCode[:], sum[32:])

	// Walk the derivation path, applying CKDpriv at each step.
	for _, index := range path {
		var err error
		key, chainCode, err = deriveChild(&key, chainCode, index)
		if err != nil {
			return nil, err
		}
	}

	return secp.NewPrivateKey(&key), nil
}

// deriveChild implements BIP-32 CKDpriv: it derives one child key from a
// parent key and chain code.
//
// For hardened children (index >= 0x80000000), the HMAC input is:
//
//	HMAC-SHA512(chain_code, 0x00 || ser256(parent_key) || ser32(index))
//
// For non-hardened (normal) children, the HMAC input uses the compressed
// public key instead:
//
//	HMAC-SHA512(chain_code, serP(point(parent_key)) || ser32(index))
//
// The child key is then: parse256(IL) + parent_key mod n, with the right
// half IR becoming the child chain code.
func deriveChild(
	parentKey *secp.ModNScalar, parentChainCode [32]byte,
	index uint32,
) (secp.ModNScalar, [32]byte, error) {

	// Build the 37-byte HMAC input: 33 bytes of key material + 4 bytes
	// of big-endian index.
	var data [37]byte
	parentPriv := secp.NewPrivateKey(parentKey)

	if index >= HardenedKeyStart {
		// Hardened child: prefix 0x00 + raw 32-byte private key.
		data[0] = 0
		copy(data[1:33], parentPriv.Serialize())
	} else {
		// Normal child: 33-byte compressed public key.
		copy(data[:33], parentPriv.PubKey().SerializeCompressed())
	}

	binary.BigEndian.PutUint32(data[33:], index)
	sum := hmacSHA512(parentChainCode[:], data[:])

	// The left 32 bytes (IL) are the tweak scalar. If IL overflows the
	// curve order or is zero, the key is invalid per the BIP-32 spec.
	var tweak secp.ModNScalar
	if overflow := tweak.SetByteSlice(sum[:32]); overflow ||
		tweak.IsZero() {

		return secp.ModNScalar{}, [32]byte{}, ErrInvalidChildKey
	}

	// Child key = IL + parent_key mod n.
	var childKey secp.ModNScalar
	childKey.Add2(parentKey, &tweak)
	if childKey.IsZero() {
		return secp.ModNScalar{}, [32]byte{}, ErrInvalidChildKey
	}

	// The right 32 bytes (IR) become the child chain code.
	var childChainCode [32]byte
	copy(childChainCode[:], sum[32:])

	return childKey, childChainCode, nil
}

// hmacSHA512 computes HMAC-SHA512 with the given key and data. BIP-32 uses
// this primitive for both master key generation and child key derivation.
func hmacSHA512(key, data []byte) [64]byte {
	mac := hmac.New(sha512.New, key)
	_, _ = mac.Write(data)

	var out [64]byte
	copy(out[:], mac.Sum(nil))
	return out
}

// serializeXOnly extracts the 32-byte x-coordinate from a compressed public
// key. This is the BIP-340 x-only serialization used by Taproot.
func serializeXOnly(pubKey *secp.PublicKey) []byte {
	compressed := pubKey.SerializeCompressed()
	xOnly := make([]byte, 32)
	copy(xOnly, compressed[1:])
	return xOnly
}
