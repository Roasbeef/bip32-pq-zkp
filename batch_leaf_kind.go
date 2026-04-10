// batch_leaf_kind.go provides the human-readable name-to-wire-format parser
// for batch leaf kinds. This is shared between the CLI flag parser and the
// nested plan manifest loader so both accept the same set of kind labels.

package bip32pqzkp

import (
	"fmt"

	batch "github.com/roasbeef/bip32-pq-zkp/batchclaim"
)

// ParseBatchLeafKindName parses one human-readable batch leaf kind label into
// its wire-format identifier.
func ParseBatchLeafKindName(name string) (uint32, error) {
	switch name {
	case "hardened-xpriv":
		return batch.LeafKindHardenedXPriv, nil

	case "taproot":
		return batch.LeafKindTaproot, nil

	case "batch-claim-v1", "batch-claim":
		return batch.LeafKindBatchClaimV1, nil

	case "heterogeneous-envelope-v1", "heterogeneous-envelope",
		"heterogeneous":
		return batch.LeafKindHeterogeneousEnvelopeV1, nil

	default:
		return 0, fmt.Errorf("unsupported batch leaf kind %q", name)
	}
}
