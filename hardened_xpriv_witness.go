// hardened_xpriv_witness.go builds the private witness stdin for the
// reduced hardened-xpriv guest. The witness is a compact 68-byte payload:
// 32 bytes parent private key + 32 bytes parent chain code + 4 bytes
// hardened child index. This is significantly smaller than the full Taproot
// witness (which includes the raw seed and a variable-length path) because
// the host pre-derives the parent xpriv outside the guest.
package bip32pqzkp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	localbip32 "github.com/roasbeef/bip32-pq-zkp/bip32"
)

// resolvedHardenedXPrivWitness is the internal resolved form of the witness
// after validating and parsing the CLI or test-vector inputs.
type resolvedHardenedXPrivWitness struct {
	parent          localbip32.ExtendedPrivateKey
	index           uint32
	usingTestVector bool
}

// BuildHardenedXPrivWitnessStdin serializes the reduced hardened-xpriv witness
// into the raw byte layout the guest expects on stdin. The wire format is:
//
//	[parent_xpriv:32] [parent_chain_code:32] [child_index:u32_le]
func BuildHardenedXPrivWitnessStdin(
	cfg HardenedXPrivWitnessConfig,
) ([]byte, bool, error) {

	witness, err := resolveHardenedXPrivWitness(cfg)
	if err != nil {
		return nil, false, err
	}

	var stdin bytes.Buffer
	stdin.Grow(32 + 32 + 4)

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
		&stdin, binary.LittleEndian, witness.index,
	); err != nil {
		return nil, false, fmt.Errorf("write child index: %w", err)
	}

	return stdin.Bytes(), witness.usingTestVector, nil
}

func resolveHardenedXPrivWitness(
	cfg HardenedXPrivWitnessConfig,
) (resolvedHardenedXPrivWitness, error) {

	switch {
	case cfg.ParentPrivateKeyHex != "" &&
		cfg.ParentChainCodeHex != "" &&
		cfg.Path != "" &&
		cfg.UseTestVector:

		return resolvedHardenedXPrivWitness{}, errors.New(
			"--use-test-vector cannot be combined with " +
				"--parent-xpriv-hex/" +
				"--parent-chain-code-hex/--path",
		)

	case cfg.ParentPrivateKeyHex != "" && cfg.ParentChainCodeHex == "":
		return resolvedHardenedXPrivWitness{}, errors.New(
			"--parent-chain-code-hex is required when " +
				"--parent-xpriv-hex is set",
		)

	case cfg.ParentPrivateKeyHex == "" && cfg.ParentChainCodeHex != "":
		return resolvedHardenedXPrivWitness{}, errors.New(
			"--parent-xpriv-hex is required when " +
				"--parent-chain-code-hex is set",
		)

	case cfg.ParentPrivateKeyHex != "" && cfg.Path == "":
		return resolvedHardenedXPrivWitness{}, errors.New(
			"--path is required when --parent-xpriv-hex is set",
		)

	case cfg.ParentPrivateKeyHex == "" && cfg.Path != "":
		return resolvedHardenedXPrivWitness{}, errors.New(
			"--parent-xpriv-hex is required when --path is set",
		)

	case cfg.ParentPrivateKeyHex != "":
		parentPrivateKey, err := decodeHexArray32(
			"--parent-xpriv-hex", cfg.ParentPrivateKeyHex,
		)
		if err != nil {
			return resolvedHardenedXPrivWitness{}, err
		}

		parentChainCode, err := decodeHexArray32(
			"--parent-chain-code-hex", cfg.ParentChainCodeHex,
		)
		if err != nil {
			return resolvedHardenedXPrivWitness{}, err
		}

		parent, err := localbip32.NewExtendedPrivateKey(
			parentPrivateKey, parentChainCode,
		)
		if err != nil {
			return resolvedHardenedXPrivWitness{}, err
		}

		path, err := ParseBIP32Path(cfg.Path)
		if err != nil {
			return resolvedHardenedXPrivWitness{}, fmt.Errorf(
				"parse --path: %w", err,
			)
		}

		index, err := localbip32.SingleHardenedChild(path)
		if err != nil {
			return resolvedHardenedXPrivWitness{}, err
		}

		return resolvedHardenedXPrivWitness{
			parent:          parent,
			index:           index,
			usingTestVector: false,
		}, nil

	case cfg.UseTestVector:
		parent, err := defaultReducedParentExtendedPrivateKey()
		if err != nil {
			return resolvedHardenedXPrivWitness{}, err
		}

		index, err := localbip32.SingleHardenedChild(
			defaultReducedChildPath,
		)
		if err != nil {
			return resolvedHardenedXPrivWitness{}, err
		}

		return resolvedHardenedXPrivWitness{
			parent:          parent,
			index:           index,
			usingTestVector: true,
		}, nil

	default:
		return resolvedHardenedXPrivWitness{}, errors.New(
			"hardened-xpriv guest requires --parent-xpriv-hex, " +
				"--parent-chain-code-hex, and --path, or " +
				"--use-test-vector",
		)
	}
}
