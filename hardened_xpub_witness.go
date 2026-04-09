package bip32pqzkp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	localbip32 "github.com/roasbeef/bip32-pq-zkp/bip32"
)

type resolvedHardenedXPubWitness struct {
	parent          localbip32.ExtendedPrivateKey
	path            []uint32
	usingTestVector bool
}

// BuildHardenedXPubWitnessStdin serializes the reduced hardened-xpub witness
// into the raw byte layout the guest expects on stdin. The wire format is:
//
//	[parent_xpriv:32] [parent_chain_code:32]
//	[path_len:u32_le] [path_component:u32_le...]
func BuildHardenedXPubWitnessStdin(
	cfg HardenedXPubWitnessConfig,
) ([]byte, bool, error) {

	witness, err := resolveHardenedXPubWitness(cfg)
	if err != nil {
		return nil, false, err
	}

	var stdin bytes.Buffer
	stdin.Grow(32 + 32 + 4 + len(witness.path)*4)

	parentPrivateKey := witness.parent.SerializePrivateKey()
	if _, err := stdin.Write(parentPrivateKey[:]); err != nil {
		return nil, false, fmt.Errorf(
			"write parent private key: %w", err,
		)
	}

	parentChainCode := witness.parent.ChainCode()
	if _, err := stdin.Write(parentChainCode[:]); err != nil {
		return nil, false, fmt.Errorf(
			"write parent chain code: %w", err,
		)
	}

	if err := binary.Write(
		&stdin, binary.LittleEndian, uint32(len(witness.path)),
	); err != nil {
		return nil, false, fmt.Errorf(
			"write hardened path length: %w", err,
		)
	}

	if err := binary.Write(
		&stdin, binary.LittleEndian, witness.path,
	); err != nil {
		return nil, false, fmt.Errorf(
			"write hardened path: %w", err,
		)
	}

	return stdin.Bytes(), witness.usingTestVector, nil
}

func resolveHardenedXPubWitness(
	cfg HardenedXPubWitnessConfig,
) (resolvedHardenedXPubWitness, error) {

	switch {
	case cfg.ParentPrivateKeyHex != "" &&
		cfg.ParentChainCodeHex != "" &&
		cfg.Path != "" &&
		cfg.UseTestVector:

		return resolvedHardenedXPubWitness{}, errors.New(
			"--use-test-vector cannot be combined with " +
				"--parent-xpriv-hex/" +
				"--parent-chain-code-hex/--path",
		)

	case cfg.ParentPrivateKeyHex != "" && cfg.ParentChainCodeHex == "":
		return resolvedHardenedXPubWitness{}, errors.New(
			"--parent-chain-code-hex is required when " +
				"--parent-xpriv-hex is set",
		)

	case cfg.ParentPrivateKeyHex == "" && cfg.ParentChainCodeHex != "":
		return resolvedHardenedXPubWitness{}, errors.New(
			"--parent-xpriv-hex is required when " +
				"--parent-chain-code-hex is set",
		)

	case cfg.ParentPrivateKeyHex != "" && cfg.Path == "":
		return resolvedHardenedXPubWitness{}, errors.New(
			"--path is required when --parent-xpriv-hex is set",
		)

	case cfg.ParentPrivateKeyHex == "" && cfg.Path != "":
		return resolvedHardenedXPubWitness{}, errors.New(
			"--parent-xpriv-hex is required when --path is set",
		)

	case cfg.ParentPrivateKeyHex != "":
		parentPrivateKey, err := decodeHexArray32(
			"--parent-xpriv-hex", cfg.ParentPrivateKeyHex,
		)
		if err != nil {
			return resolvedHardenedXPubWitness{}, err
		}

		parentChainCode, err := decodeHexArray32(
			"--parent-chain-code-hex", cfg.ParentChainCodeHex,
		)
		if err != nil {
			return resolvedHardenedXPubWitness{}, err
		}

		parent, err := localbip32.NewExtendedPrivateKey(
			parentPrivateKey, parentChainCode,
		)
		if err != nil {
			return resolvedHardenedXPubWitness{}, err
		}

		path, err := ParseBIP32Path(cfg.Path)
		if err != nil {
			return resolvedHardenedXPubWitness{}, fmt.Errorf(
				"parse --path: %w", err,
			)
		}
		if err := localbip32.ValidateAllHardened(path); err != nil {
			return resolvedHardenedXPubWitness{}, err
		}

		return resolvedHardenedXPubWitness{
			parent:          parent,
			path:            path,
			usingTestVector: false,
		}, nil

	case cfg.UseTestVector:
		parent, err := defaultReducedParentExtendedPrivateKey()
		if err != nil {
			return resolvedHardenedXPubWitness{}, err
		}

		return resolvedHardenedXPubWitness{
			parent: parent,
			path: append(
				[]uint32(nil), defaultReducedChildPath...,
			),
			usingTestVector: true,
		}, nil

	default:
		return resolvedHardenedXPubWitness{}, errors.New(
			"hardened-xpub guest requires --parent-xpriv-hex, " +
				"--parent-chain-code-hex, and --path, or " +
				"--use-test-vector",
		)
	}
}

func defaultReducedParentExtendedPrivateKey() (localbip32.ExtendedPrivateKey,
	error) {

	seed := append([]byte(nil), defaultBIP32Seed...)

	return localbip32.DeriveExtendedPrivateKey(
		seed, defaultReducedParentPath,
	)
}
