// envelope.go defines the fixed-size heterogeneous direct-child envelope
// used by parent batches that mix raw leaves and child batch claims at one
// level. The envelope carries the direct child kind, the per-child verify
// image ID, and the raw journal bytes padded to a fixed maximum size. This
// enables the parent guest to call zkvm.Verify with each child's own image
// ID rather than requiring all children to share one common image.
//
// The envelope schema is designed to keep the Merkle tree leaf size fixed
// regardless of which direct child kind appears. The current maximum journal
// size is PublicClaimSize (84 bytes), giving an envelope size of 128 bytes.
// If a future leaf kind has a larger journal, the envelope size will need to
// grow, which would require a new envelope version.

package batchclaim

import (
	"encoding/binary"
	"fmt"
)

const (
	// HeterogeneousEnvelopeVersionV1 is the first fixed-size direct-child
	// envelope schema used by heterogeneous parent batches.
	HeterogeneousEnvelopeVersionV1 = 1

	// HeterogeneousEnvelopeMaxJournalSizeV1 is the largest direct-child
	// journal size supported by the first heterogeneous parent design.
	HeterogeneousEnvelopeMaxJournalSizeV1 = PublicClaimSize

	// HeterogeneousEnvelopeSizeV1 is the serialized byte size of the v1
	// direct-child envelope:
	// 4 (version) + 4 (direct child kind) + 32 (verify image id) +
	// 4 (journal length) + 84 (padded journal) = 128.
	HeterogeneousEnvelopeSizeV1 = 128
)

// HeterogeneousEnvelopeV1 is the fixed-size direct-child record hashed by a
// heterogeneous parent batch. Each envelope carries the direct child kind,
// the image ID to use for recursive verification, and the direct child's raw
// journal bytes padded to a fixed maximum size.
type HeterogeneousEnvelopeV1 struct {
	// Version identifies the serialized envelope format.
	Version uint32

	// DirectLeafKind identifies the semantic child type stored in the
	// envelope.
	DirectLeafKind uint32

	// VerifyImageID is the image ID that the parent guest must use for
	// `zkvm.Verify` against the direct child receipt.
	VerifyImageID [32]byte

	// JournalLen is the actual direct-child journal byte length.
	JournalLen uint32

	// PaddedJournal stores the direct-child journal padded to the fixed
	// maximum size.
	PaddedJournal [HeterogeneousEnvelopeMaxJournalSizeV1]byte
}

// NewHeterogeneousEnvelopeV1 constructs a new envelope for one mixed direct
// child. The supported child kinds are the current raw batch leaf schemas plus
// `batch_claim_v1`.
func NewHeterogeneousEnvelopeV1(
	directLeafKind uint32, verifyImageID [32]byte, journal []byte,
) (HeterogeneousEnvelopeV1, error) {

	if !IsAllowedHeterogeneousDirectLeafKindV1(directLeafKind) {
		return HeterogeneousEnvelopeV1{}, fmt.Errorf(
			"unsupported heterogeneous direct leaf kind %d",
			directLeafKind,
		)
	}

	expectedSize, ok := LeafClaimSize(directLeafKind)
	if !ok {
		return HeterogeneousEnvelopeV1{}, fmt.Errorf(
			"unknown direct leaf kind %d",
			directLeafKind,
		)
	}
	if len(journal) != expectedSize {
		return HeterogeneousEnvelopeV1{}, fmt.Errorf(
			"direct leaf journal size mismatch: got %d, want %d",
			len(journal), expectedSize,
		)
	}

	var padded [HeterogeneousEnvelopeMaxJournalSizeV1]byte
	copy(padded[:], journal)

	return HeterogeneousEnvelopeV1{
		Version:        HeterogeneousEnvelopeVersionV1,
		DirectLeafKind: directLeafKind,
		VerifyImageID:  verifyImageID,
		JournalLen:     uint32(len(journal)),
		PaddedJournal:  padded,
	}, nil
}

// Encode serializes the envelope into its fixed 128-byte layout.
func (e HeterogeneousEnvelopeV1) Encode() [HeterogeneousEnvelopeSizeV1]byte {
	var out [HeterogeneousEnvelopeSizeV1]byte
	binary.LittleEndian.PutUint32(out[0:4], e.Version)
	binary.LittleEndian.PutUint32(out[4:8], e.DirectLeafKind)
	copy(out[8:40], e.VerifyImageID[:])
	binary.LittleEndian.PutUint32(out[40:44], e.JournalLen)
	copy(out[44:], e.PaddedJournal[:])
	return out
}

// DecodeHeterogeneousEnvelopeV1 parses one fixed-size v1 envelope.
func DecodeHeterogeneousEnvelopeV1(
	record []byte,
) (HeterogeneousEnvelopeV1, error) {

	if len(record) != HeterogeneousEnvelopeSizeV1 {
		return HeterogeneousEnvelopeV1{}, fmt.Errorf(
			"unexpected heterogeneous envelope size: got %d "+
				"bytes, want %d",
			len(record), HeterogeneousEnvelopeSizeV1,
		)
	}

	var envelope HeterogeneousEnvelopeV1
	envelope.Version = binary.LittleEndian.Uint32(record[0:4])
	envelope.DirectLeafKind = binary.LittleEndian.Uint32(record[4:8])
	copy(envelope.VerifyImageID[:], record[8:40])
	envelope.JournalLen = binary.LittleEndian.Uint32(record[40:44])
	copy(envelope.PaddedJournal[:], record[44:])

	if envelope.Version != HeterogeneousEnvelopeVersionV1 {
		return HeterogeneousEnvelopeV1{}, fmt.Errorf(
			"unsupported heterogeneous envelope version %d",
			envelope.Version,
		)
	}
	if !IsAllowedHeterogeneousDirectLeafKindV1(
		envelope.DirectLeafKind,
	) {

		return HeterogeneousEnvelopeV1{}, fmt.Errorf(
			"unsupported heterogeneous direct leaf kind %d",
			envelope.DirectLeafKind,
		)
	}
	expectedSize, ok := LeafClaimSize(envelope.DirectLeafKind)
	if !ok {
		return HeterogeneousEnvelopeV1{}, fmt.Errorf(
			"unknown heterogeneous direct leaf kind %d",
			envelope.DirectLeafKind,
		)
	}
	if int(envelope.JournalLen) != expectedSize {
		return HeterogeneousEnvelopeV1{}, fmt.Errorf(
			"heterogeneous direct journal size mismatch: "+
				"got %d, want %d",
			envelope.JournalLen,
			expectedSize,
		)
	}

	return envelope, nil
}

// JournalBytes returns the real, unpadded direct-child journal.
func (e HeterogeneousEnvelopeV1) JournalBytes() []byte {
	end := int(e.JournalLen)
	if end < 0 || end > len(e.PaddedJournal) {
		return nil
	}

	return append([]byte(nil), e.PaddedJournal[:end]...)
}

// IsAllowedHeterogeneousDirectLeafKindV1 reports whether the direct child kind
// is currently supported inside the first heterogeneous parent mode.
func IsAllowedHeterogeneousDirectLeafKindV1(kind uint32) bool {
	switch kind {
	case LeafKindTaproot, LeafKindHardenedXPriv, LeafKindBatchClaimV1:
		return true

	default:
		return false
	}
}
