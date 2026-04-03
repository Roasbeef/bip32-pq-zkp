package bip32

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"errors"

	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
)

const (
	HardenedKeyStart = 0x80000000
	minSeedBytes     = 16
	maxSeedBytes     = 64
)

var (
	ErrInvalidSeedLength = errors.New("invalid seed length")
	ErrInvalidMasterKey  = errors.New("invalid master key")
	ErrInvalidChildKey   = errors.New("invalid child key")
	masterKeySalt        = []byte("Bitcoin seed")
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
func DerivePrivateKey(seed []byte, path []uint32) (*secp.PrivateKey, error) {
	if len(seed) < minSeedBytes || len(seed) > maxSeedBytes {
		return nil, ErrInvalidSeedLength
	}

	sum := hmacSHA512(masterKeySalt, seed)

	var key secp.ModNScalar
	if overflow := key.SetByteSlice(sum[:32]); overflow || key.IsZero() {
		return nil, ErrInvalidMasterKey
	}

	var chainCode [32]byte
	copy(chainCode[:], sum[32:])

	for _, index := range path {
		var err error
		key, chainCode, err = deriveChild(&key, chainCode, index)
		if err != nil {
			return nil, err
		}
	}

	return secp.NewPrivateKey(&key), nil
}

func deriveChild(parentKey *secp.ModNScalar, parentChainCode [32]byte, index uint32) (secp.ModNScalar, [32]byte, error) {
	var data [37]byte
	parentPriv := secp.NewPrivateKey(parentKey)

	if index >= HardenedKeyStart {
		data[0] = 0
		copy(data[1:33], parentPriv.Serialize())
	} else {
		copy(data[:33], parentPriv.PubKey().SerializeCompressed())
	}

	binary.BigEndian.PutUint32(data[33:], index)
	sum := hmacSHA512(parentChainCode[:], data[:])

	var tweak secp.ModNScalar
	if overflow := tweak.SetByteSlice(sum[:32]); overflow || tweak.IsZero() {
		return secp.ModNScalar{}, [32]byte{}, ErrInvalidChildKey
	}

	var childKey secp.ModNScalar
	childKey.Add2(parentKey, &tweak)
	if childKey.IsZero() {
		return secp.ModNScalar{}, [32]byte{}, ErrInvalidChildKey
	}

	var childChainCode [32]byte
	copy(childChainCode[:], sum[32:])

	return childKey, childChainCode, nil
}

func hmacSHA512(key, data []byte) [64]byte {
	mac := hmac.New(sha512.New, key)
	_, _ = mac.Write(data)

	var out [64]byte
	copy(out[:], mac.Sum(nil))
	return out
}

func serializeXOnly(pubKey *secp.PublicKey) []byte {
	compressed := pubKey.SerializeCompressed()
	xOnly := make([]byte, 32)
	copy(xOnly, compressed[1:])
	return xOnly
}
