package bip32

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

const (
	ClaimVersion          = 1
	ClaimFlagRequireBIP86 = 1
	PublicClaimSize       = 72
)

var ErrInvalidPublicClaimSize = errors.New("invalid public claim size")

// PublicClaim is the verifier-visible claim material committed by the guest.
type PublicClaim struct {
	Version          uint32
	Flags            uint32
	TaprootOutputKey [32]byte
	PathCommitment   [32]byte
}

// DeriveTaprootClaim derives the Taproot output key and packages the
// verifier-visible claim material for the current private witness.
func DeriveTaprootClaim(seed []byte, path []uint32, opts ...TaprootDeriveOption) (PublicClaim, error) {
	options := parseTaprootDeriveOptions(opts...)

	outputKey, err := DeriveTaprootOutputKey(seed, path, opts...)
	if err != nil {
		return PublicClaim{}, err
	}

	var claim PublicClaim
	claim.Version = ClaimVersion
	claim.Flags = options.claimFlags()
	copy(claim.TaprootOutputKey[:], outputKey)
	claim.PathCommitment = CommitPath(path)

	return claim, nil
}

// Encode serializes the public claim into the journal layout used by the guest.
func (claim PublicClaim) Encode() []byte {
	out := make([]byte, PublicClaimSize)
	binary.LittleEndian.PutUint32(out[0:4], claim.Version)
	binary.LittleEndian.PutUint32(out[4:8], claim.Flags)
	copy(out[8:40], claim.TaprootOutputKey[:])
	copy(out[40:72], claim.PathCommitment[:])
	return out
}

// DecodePublicClaim parses the committed journal bytes into a structured claim.
func DecodePublicClaim(data []byte) (PublicClaim, error) {
	if len(data) != PublicClaimSize {
		return PublicClaim{}, ErrInvalidPublicClaimSize
	}

	var claim PublicClaim
	claim.Version = binary.LittleEndian.Uint32(data[0:4])
	claim.Flags = binary.LittleEndian.Uint32(data[4:8])
	copy(claim.TaprootOutputKey[:], data[8:40])
	copy(claim.PathCommitment[:], data[40:72])
	return claim, nil
}

// CommitPath domain-separates and hashes the private path so it can appear in
// the verifier-facing claim without revealing the path itself.
func CommitPath(path []uint32) [32]byte {
	h := sha256.New()
	_, _ = h.Write([]byte("bip32-pq-zkp:path:v1"))

	var word [4]byte
	binary.LittleEndian.PutUint32(word[:], uint32(len(path)))
	_, _ = h.Write(word[:])

	for _, index := range path {
		binary.LittleEndian.PutUint32(word[:], index)
		_, _ = h.Write(word[:])
	}

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
