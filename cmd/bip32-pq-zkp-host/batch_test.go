// batch_test.go validates the CLI flag parsing for the batch subcommands,
// especially the repeatable --leaf-claim / --leaf-receipt / --inclusion-in
// flags and the new --leaf-kind batch-claim-v1 and --inclusion-chain-in
// flag introduced for nested batch verification.

package main

import (
	"testing"

	batch "github.com/roasbeef/bip32-pq-zkp/batchclaim"
)

func TestParseBatchLeafInputsSupportsBatchClaimV1(t *testing.T) {
	t.Parallel()

	inputs, leafKind, err := parseBatchLeafInputs(batchArgs{
		leafKind:     "batch-claim-v1",
		leafClaims:   batchLeafList{"a.claim.json"},
		leafReceipts: batchLeafList{"a.receipt"},
	})
	if err != nil {
		t.Fatalf("parseBatchLeafInputs failed: %v", err)
	}
	if leafKind != batch.LeafKindBatchClaimV1 {
		t.Fatalf(
			"leafKind = %d, want %d",
			leafKind,
			batch.LeafKindBatchClaimV1,
		)
	}
	if len(inputs) != 1 {
		t.Fatalf("got %d inputs, want 1", len(inputs))
	}
}

func TestParseBatchLeafInputsSupportsHeterogeneousEnvelope(t *testing.T) {
	t.Parallel()

	inputs, leafKind, err := parseBatchLeafInputs(batchArgs{
		leafKind:    "heterogeneous-envelope-v1",
		directKinds: batchLeafList{"batch-claim-v1", "hardened-xpriv"},
		leafClaims: batchLeafList{
			"child.claim.json",
			"leaf.claim.json",
		},
		leafReceipts: batchLeafList{
			"child.receipt",
			"leaf.receipt",
		},
	})
	if err != nil {
		t.Fatalf("parseBatchLeafInputs failed: %v", err)
	}
	if leafKind != batch.LeafKindHeterogeneousEnvelopeV1 {
		t.Fatalf(
			"leafKind = %d, want %d",
			leafKind,
			batch.LeafKindHeterogeneousEnvelopeV1,
		)
	}
	if len(inputs) != 2 {
		t.Fatalf("got %d inputs, want 2", len(inputs))
	}
	if inputs[0].DirectLeafKind != batch.LeafKindBatchClaimV1 {
		t.Fatalf(
			"first direct kind = %d, want %d",
			inputs[0].DirectLeafKind,
			batch.LeafKindBatchClaimV1,
		)
	}
	if inputs[1].DirectLeafKind != batch.LeafKindHardenedXPriv {
		t.Fatalf(
			"second direct kind = %d, want %d",
			inputs[1].DirectLeafKind,
			batch.LeafKindHardenedXPriv,
		)
	}
}

func TestParseBatchLeafInputsRejectsMissingHeterogeneousKinds(t *testing.T) {
	t.Parallel()

	_, _, err := parseBatchLeafInputs(batchArgs{
		leafKind: "heterogeneous-envelope-v1",
		leafClaims: batchLeafList{
			"a.claim.json",
		},
		leafReceipts: batchLeafList{
			"a.receipt",
		},
	})
	if err == nil {
		t.Fatalf("expected missing direct leaf kind error")
	}
}

func TestParseRunNestedBatchPlanArgs(t *testing.T) {
	t.Parallel()

	args, err := parseRunNestedBatchPlanArgs([]string{
		"--plan", "plan.json",
	})
	if err != nil {
		t.Fatalf("parseRunNestedBatchPlanArgs failed: %v", err)
	}
	if args.plan != "plan.json" {
		t.Fatalf("plan = %q, want plan.json", args.plan)
	}
}

func TestParseVerifyBatchArgsCollectsRepeatedInclusionFlags(t *testing.T) {
	t.Parallel()

	args, err := parseVerifyBatchArgs([]string{
		"--receipt-in", "parent.receipt",
		"--inclusion-in", "parent.json",
		"--inclusion-in", "child.json",
	})
	if err != nil {
		t.Fatalf("parseVerifyBatchArgs failed: %v", err)
	}

	if len(args.inclusionIns) != 2 {
		t.Fatalf(
			"got %d inclusion proofs, want 2",
			len(args.inclusionIns),
		)
	}
	if args.inclusionIns[0] != "parent.json" {
		t.Fatalf(
			"first inclusion = %q, want parent.json",
			args.inclusionIns[0],
		)
	}
	if args.inclusionIns[1] != "child.json" {
		t.Fatalf(
			"second inclusion = %q, want child.json",
			args.inclusionIns[1],
		)
	}
}

func TestParseVerifyBatchArgsAcceptsInclusionChain(t *testing.T) {
	t.Parallel()

	args, err := parseVerifyBatchArgs([]string{
		"--receipt-in", "parent.receipt",
		"--inclusion-chain-in", "chain.json",
	})
	if err != nil {
		t.Fatalf("parseVerifyBatchArgs failed: %v", err)
	}

	if args.chainIn != "chain.json" {
		t.Fatalf(
			"chainIn = %q, want chain.json",
			args.chainIn,
		)
	}
	if len(args.inclusionIns) != 0 {
		t.Fatalf(
			"got %d repeated inclusion proofs, want 0",
			len(args.inclusionIns),
		)
	}
}

func TestParseBundleBatchInclusionChainArgs(t *testing.T) {
	t.Parallel()

	args, err := parseBundleBatchInclusionChainArgs([]string{
		"--inclusion-in", "parent.json",
		"--inclusion-in", "child.json",
		"--chain-out", "chain.json",
	})
	if err != nil {
		t.Fatalf(
			"parseBundleBatchInclusionChainArgs failed: %v",
			err,
		)
	}

	if len(args.inclusionIns) != 2 {
		t.Fatalf(
			"got %d inclusion proofs, want 2",
			len(args.inclusionIns),
		)
	}
	if args.chainOut != "chain.json" {
		t.Fatalf(
			"chainOut = %q, want chain.json",
			args.chainOut,
		)
	}
}
