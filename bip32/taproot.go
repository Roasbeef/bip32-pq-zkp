package bip32

import (
	"crypto/sha256"
	"errors"

	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
)

var (
	// ErrInvalidBIP86Path indicates the provided path is not BIP-86 shaped.
	ErrInvalidBIP86Path = errors.New("invalid bip86 path")
	// ErrInvalidTaprootTweak indicates the tweak scalar was out of range.
	ErrInvalidTaprootTweak = errors.New("invalid taproot tweak")
	// ErrInvalidTaprootKey indicates tweak addition produced an
	// invalid key.
	ErrInvalidTaprootKey = errors.New("invalid taproot key")
	tagTapTweak          = []byte("TapTweak")
)

const (
	// BIP86Purpose is the hardened BIP-86 purpose component (`86'`).
	BIP86Purpose = HardenedKeyStart + 86
	// BIP86PathLen is the expected number of path elements for BIP-86.
	BIP86PathLen = 5
)

type taprootDeriveOptions struct {
	requireBIP86Path bool
}

func parseTaprootDeriveOptions(
	opts ...TaprootDeriveOption,
) taprootDeriveOptions {

	var options taprootDeriveOptions
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

func (options taprootDeriveOptions) claimFlags() uint32 {
	var flags uint32
	if options.requireBIP86Path {
		flags |= ClaimFlagRequireBIP86
	}
	return flags
}

// TaprootDeriveOption configures optional Taproot derivation policy checks.
type TaprootDeriveOption func(*taprootDeriveOptions)

// WithBIP86PathVerification enforces that the private derivation path matches
// the BIP-86 shape m/86'/coin_type'/account'/change/index.
func WithBIP86PathVerification() TaprootDeriveOption {
	return func(options *taprootDeriveOptions) {
		options.requireBIP86Path = true
	}
}

// DeriveTaprootOutputKey derives the BIP-32 child private key at the given
// path, converts it to the BIP-340 even-Y internal key, then computes the
// key-spend-only Taproot output key tweak.
func DeriveTaprootOutputKey(
	seed []byte, path []uint32, opts ...TaprootDeriveOption,
) ([]byte, error) {

	options := parseTaprootDeriveOptions(opts...)
	if options.requireBIP86Path && !IsBIP86Path(path) {
		return nil, ErrInvalidBIP86Path
	}

	privKey, err := DerivePrivateKey(seed, path)
	if err != nil {
		return nil, err
	}

	taprootKey, err := ComputeTaprootKeyNoScript(privKey.PubKey())
	if err != nil {
		return nil, err
	}

	return serializeXOnly(taprootKey), nil
}

// IsBIP86Path reports whether the provided derivation path matches the BIP-86
// shape m/86'/coin_type'/account'/change/index.
func IsBIP86Path(path []uint32) bool {
	if len(path) != BIP86PathLen {
		return false
	}
	if path[0] != BIP86Purpose {
		return false
	}
	if path[1] < HardenedKeyStart || path[2] < HardenedKeyStart {
		return false
	}
	if path[3] >= HardenedKeyStart || path[4] >= HardenedKeyStart {
		return false
	}
	if path[3] > 1 {
		return false
	}

	return true
}

// ComputeTaprootKeyNoScript mirrors txscript.ComputeTaprootKeyNoScript for the
// BIP-86 key-spend-only case.
func ComputeTaprootKeyNoScript(
	pubKey *secp.PublicKey,
) (*secp.PublicKey, error) {

	return ComputeTaprootOutputKey(pubKey, nil)
}

// ComputeTaprootOutputKey mirrors txscript.ComputeTaprootOutputKey using the
// local secp256k1 primitives.
func ComputeTaprootOutputKey(
	pubKey *secp.PublicKey, scriptRoot []byte,
) (*secp.PublicKey, error) {

	internalKey, err := liftXEven(pubKey)
	if err != nil {
		return nil, err
	}

	tapTweakHash := taggedHash(
		tagTapTweak, serializeXOnly(internalKey), scriptRoot,
	)

	var tweakScalar secp.ModNScalar
	if overflow := tweakScalar.SetBytes(&tapTweakHash); overflow != 0 {
		return nil, ErrInvalidTaprootTweak
	}

	var internalPoint secp.JacobianPoint
	internalKey.AsJacobian(&internalPoint)

	var tweakPoint, taprootKey secp.JacobianPoint
	secp.ScalarBaseMultNonConst(&tweakScalar, &tweakPoint)
	secp.AddNonConst(&internalPoint, &tweakPoint, &taprootKey)
	if (taprootKey.X.IsZero() && taprootKey.Y.IsZero()) ||
		taprootKey.Z.IsZero() {

		return nil, ErrInvalidTaprootKey
	}

	taprootKey.ToAffine()
	return secp.NewPublicKey(&taprootKey.X, &taprootKey.Y), nil
}

func liftXEven(pubKey *secp.PublicKey) (*secp.PublicKey, error) {
	compressed := pubKey.SerializeCompressed()
	var evenCompressed [secp.PubKeyBytesLenCompressed]byte
	evenCompressed[0] = secp.PubKeyFormatCompressedEven
	copy(evenCompressed[1:], compressed[1:])
	return secp.ParsePubKey(evenCompressed[:])
}

func taggedHash(tag []byte, msgs ...[]byte) [32]byte {
	tagHash := sha256.Sum256(tag)
	h := sha256.New()
	_, _ = h.Write(tagHash[:])
	_, _ = h.Write(tagHash[:])
	for _, msg := range msgs {
		_, _ = h.Write(msg)
	}

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
