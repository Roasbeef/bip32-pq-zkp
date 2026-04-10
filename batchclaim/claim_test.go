// claim_test.go validates the leaf kind registry constants and their
// associated journal sizes. These are critical invariants: changing a
// leaf kind name or size would break round-trip compatibility between
// the guest, host, and external verifiers.

package batchclaim

import "testing"

func TestLeafKindName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind uint32
		name string
	}{
		{kind: LeafKindTaproot, name: "taproot"},
		{kind: LeafKindHardenedXPriv, name: "hardened_xpriv"},
		{kind: LeafKindBatchClaimV1, name: "batch_claim_v1"},
		{
			kind: LeafKindHeterogeneousEnvelopeV1,
			name: "heterogeneous_envelope_v1",
		},
	}

	for _, test := range tests {
		if got := LeafKindName(test.kind); got != test.name {
			t.Fatalf(
				"LeafKindName(%d) = %q, want %q",
				test.kind,
				got,
				test.name,
			)
		}
	}
}

func TestLeafClaimSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind uint32
		size int
		ok   bool
	}{
		{kind: LeafKindTaproot, size: 72, ok: true},
		{kind: LeafKindHardenedXPriv, size: 72, ok: true},
		{kind: LeafKindBatchClaimV1, size: PublicClaimSize, ok: true},
		{
			kind: LeafKindHeterogeneousEnvelopeV1,
			size: HeterogeneousEnvelopeSizeV1,
			ok:   true,
		},
		{kind: 99, size: 0, ok: false},
	}

	for _, test := range tests {
		size, ok := LeafClaimSize(test.kind)
		if ok != test.ok {
			t.Fatalf(
				"LeafClaimSize(%d) ok = %v, want %v",
				test.kind,
				ok,
				test.ok,
			)
		}
		if size != test.size {
			t.Fatalf(
				"LeafClaimSize(%d) size = %d, want %d",
				test.kind,
				size,
				test.size,
			)
		}
	}
}

func TestHeterogeneousEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	imageID := [32]byte{1, 2, 3}
	journal := make([]byte, PublicClaimSize)
	for i := range journal {
		journal[i] = byte(i)
	}

	envelope, err := NewHeterogeneousEnvelopeV1(
		LeafKindBatchClaimV1, imageID, journal,
	)
	if err != nil {
		t.Fatalf("NewHeterogeneousEnvelopeV1 failed: %v", err)
	}

	decoded, err := DecodeHeterogeneousEnvelopeV1(
		func() []byte {
			encoded := envelope.Encode()
			return encoded[:]
		}(),
	)
	if err != nil {
		t.Fatalf("DecodeHeterogeneousEnvelopeV1 failed: %v", err)
	}

	if decoded.DirectLeafKind != LeafKindBatchClaimV1 {
		t.Fatalf(
			"DirectLeafKind = %d, want %d",
			decoded.DirectLeafKind, LeafKindBatchClaimV1,
		)
	}
	if decoded.VerifyImageID != imageID {
		t.Fatalf(
			"VerifyImageID = %x, want %x",
			decoded.VerifyImageID, imageID,
		)
	}
	if got := decoded.JournalBytes(); string(got) != string(journal) {
		t.Fatalf("JournalBytes mismatch")
	}
}

func TestHeterogeneousPolicyDigestV1Stable(t *testing.T) {
	t.Parallel()

	digestA := HeterogeneousPolicyDigestV1()
	digestB := HeterogeneousPolicyDigestV1()
	if digestA != digestB {
		t.Fatalf("policy digest should be stable")
	}
}
