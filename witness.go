package bip32pqzkp

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type resolvedWitness struct {
	seed            []byte
	path            []uint32
	flags           uint32
	usingTestVector bool
}

// BuildWitnessStdin serializes the demo witness into the raw byte layout the
// guest expects on stdin.
func BuildWitnessStdin(cfg WitnessConfig) ([]byte, bool, error) {
	witness, err := resolveWitness(cfg)
	if err != nil {
		return nil, false, err
	}

	stdin := make([]byte, 0, 4+4+len(witness.seed)+4+len(witness.path)*4)
	stdin = appendU32(stdin, witness.flags)
	stdin = appendU32(stdin, uint32(len(witness.seed)))
	stdin = append(stdin, witness.seed...)
	stdin = appendU32(stdin, uint32(len(witness.path)))
	for _, component := range witness.path {
		stdin = appendU32(stdin, component)
	}

	return stdin, witness.usingTestVector, nil
}

func resolveWitness(cfg WitnessConfig) (resolvedWitness, error) {
	switch {
	case cfg.SeedHex != "" && cfg.Path != "" && cfg.UseTestVector:
		return resolvedWitness{}, errors.New(
			"--use-test-vector cannot be combined with " +
				"--seed-hex/--path",
		)

	case cfg.SeedHex != "" && cfg.Path == "":
		return resolvedWitness{}, errors.New(
			"--path is required when --seed-hex is set",
		)

	case cfg.SeedHex == "" && cfg.Path != "":
		return resolvedWitness{}, errors.New(
			"--seed-hex is required when --path is set",
		)

	case cfg.SeedHex != "" && cfg.Path != "":
		seed, err := decodeHex(cfg.SeedHex)
		if err != nil {
			return resolvedWitness{}, fmt.Errorf(
				"decode --seed-hex: %w", err,
			)
		}

		path, err := ParseBIP32Path(cfg.Path)
		if err != nil {
			return resolvedWitness{}, fmt.Errorf(
				"parse --path: %w", err,
			)
		}

		return finalizeWitness(seed, path, false, cfg.RequireBIP86)

	case cfg.UseTestVector:
		seed := append([]byte(nil), defaultBIP32Seed...)
		path := append([]uint32(nil), defaultBIP32Path...)

		return finalizeWitness(seed, path, true, cfg.RequireBIP86)

	default:
		return resolvedWitness{}, errors.New(
			"bip32 guest requires --seed-hex and --path, or " +
				"--use-test-vector",
		)
	}
}

func finalizeWitness(seed []byte, path []uint32, usingTestVector bool,
	requireBIP86 bool) (resolvedWitness, error) {

	if requireBIP86 {
		if err := ValidateBIP86Path(path); err != nil {
			return resolvedWitness{}, err
		}
	}

	var flags uint32
	if requireBIP86 {
		flags |= witnessFlagRequireBIP86
	}

	return resolvedWitness{
		seed:            seed,
		path:            path,
		flags:           flags,
		usingTestVector: usingTestVector,
	}, nil
}

// ParseBIP32Path accepts either slash-form or comma-form derivation paths and
// returns the encoded 32-bit path components expected by the guest.
func ParseBIP32Path(pathSpec string) ([]uint32, error) {
	trimmed := strings.TrimSpace(pathSpec)
	stripped := strings.TrimPrefix(strings.TrimPrefix(trimmed, "m/"), "M/")
	if stripped == "" {
		return []uint32{}, nil
	}

	separator := ","
	if strings.Contains(stripped, "/") {
		separator = "/"
	}

	components := strings.Split(stripped, separator)
	path := make([]uint32, 0, len(components))
	for _, component := range components {
		component = strings.TrimSpace(component)
		if component == "" {
			return nil, errors.New("empty path component")
		}

		hardened := strings.HasSuffix(component, "'") ||
			strings.HasSuffix(component, "h") ||
			strings.HasSuffix(component, "H")

		digits := component
		if hardened {
			digits = component[:len(component)-1]
		}

		value, err := strconv.ParseUint(digits, 10, 32)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid path component `%s`", component,
			)
		}

		componentValue := uint32(value)
		if hardened {
			componentValue += bip32HardenedKeyStart
		}

		path = append(path, componentValue)
	}

	return path, nil
}

// ValidateBIP86Path enforces the standard BIP-86 path shape:
// `m/86'/coin_type'/account'/change/index`.
func ValidateBIP86Path(path []uint32) error {
	if len(path) != 5 {
		return errors.New("BIP-86 path must have 5 elements")
	}
	if path[0] != bip86Purpose {
		return errors.New("BIP-86 purpose must be 86'")
	}
	if path[1] < bip32HardenedKeyStart {
		return errors.New("coin_type must be hardened")
	}
	if path[2] < bip32HardenedKeyStart {
		return errors.New("account must be hardened")
	}
	if path[3] >= bip32HardenedKeyStart {
		return errors.New("change must not be hardened")
	}
	if path[4] >= bip32HardenedKeyStart {
		return errors.New("index must not be hardened")
	}
	if path[3] > 1 {
		return errors.New("change must be 0 or 1")
	}

	return nil
}

// PathCommitmentFromPath reproduces the verifier-visible path commitment used
// by the guest claim.
func PathCommitmentFromPath(path []uint32) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte("bip32-pq-zkp:path:v1"))

	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(path)))
	hasher.Write(lenBuf[:])

	for _, component := range path {
		var componentBuf [4]byte
		binary.LittleEndian.PutUint32(componentBuf[:], component)
		hasher.Write(componentBuf[:])
	}

	var out [32]byte
	copy(out[:], hasher.Sum(nil))

	return out
}

func decodeHex(value string) ([]byte, error) {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X")

	return hex.DecodeString(trimmed)
}

func decodeHexArray32(label, value string) ([32]byte, error) {
	bytes, err := decodeHex(value)
	if err != nil {
		return [32]byte{}, fmt.Errorf("decode %s: %w", label, err)
	}
	if len(bytes) != 32 {
		return [32]byte{}, fmt.Errorf(
			"%s must be 32 bytes, got %d", label, len(bytes),
		)
	}

	var out [32]byte
	copy(out[:], bytes)

	return out, nil
}

func appendU32(dst []byte, value uint32) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], value)

	return append(dst, buf[:]...)
}
