// batch_runner.go implements Execute, Prove, Verify, and DeriveBatchInclusion
// for the recursive batch aggregation lane. The Execute and Prove paths
// follow the same pattern as the single-leaf lanes but with two additions:
//
//  1. Leaf receipts are loaded, verified, and supplied as host-side
//     assumptions via the go-zkvm AssumptionReceipt type.
//  2. The batch witness stdin includes the leaf journals that the guest
//     will re-verify inside the zkVM.
//
// The Verify path supports two modes: batch-only (just verify the final
// receipt) and sparse-inclusion (verify the receipt, then check one
// disclosed leaf against the committed Merkle root).
//
// DeriveBatchInclusion is a host-only operation that builds a Merkle
// inclusion proof for one leaf from the full ordered leaf set. It does
// not require the prover or any receipts -- just the leaf claim files.
package bip32pqzkp

import (
	"errors"
	"fmt"
	"os"

	batch "github.com/roasbeef/bip32-pq-zkp/batchclaim"
	zkvmhost "github.com/roasbeef/go-zkvm/host"
)

// ExecuteBatch runs the aggregation guest without generating a proof and
// returns the decoded public batch claim plus session metadata.
func (r *Runner) ExecuteBatch(
	cfg BatchExecuteConfig,
) (*BatchExecuteReport, error) {

	guestPath, guestBinary, imageID, err := r.loadGuestWithDefault(
		cfg.GuestPath, DefaultBatchGuestPath,
	)
	if err != nil {
		return nil, err
	}

	leaves, err := r.loadBatchLeaves(cfg.LeafClaimKind, cfg.LeafInputs)
	if err != nil {
		return nil, err
	}

	stdin, err := buildBatchWitnessStdin(
		cfg.LeafClaimKind, leaves.leafGuestImageID, leaves.journals,
	)
	if err != nil {
		return nil, err
	}

	result, err := r.client.Execute(zkvmhost.ExecuteRequest{
		GuestBinary: guestBinary,
		Stdin:       stdin,
		Assumptions: leaves.assumptions,
	})
	if err != nil {
		return nil, fmt.Errorf("execute batch guest: %w", err)
	}

	claim, err := DecodeBatchClaim(result.Journal)
	if err != nil {
		return nil, err
	}

	return &BatchExecuteReport{
		GuestPath:    guestPath,
		GuestSize:    len(guestBinary),
		ImageID:      imageID,
		LeafCount:    claim.LeafCount,
		Claim:        claim,
		ExitCode:     result.ExitCode,
		JournalSize:  len(result.Journal),
		SegmentCount: result.SegmentCount,
		SessionRows:  result.SessionRows,
	}, nil
}

// ProveBatch runs the aggregation guest through the prover, writes the receipt
// and claim artifacts, and returns the decoded public batch claim plus proof
// metadata.
func (r *Runner) ProveBatch(
	cfg BatchProveConfig,
) (*BatchProveReport, error) {

	switch {
	case cfg.ReceiptOutputPath == "":
		return nil, errors.New("--receipt-out is required")
	case cfg.ClaimOutputPath == "":
		return nil, errors.New("--claim-out is required")
	}

	guestPath, guestBinary, imageID, err := r.loadGuestWithDefault(
		cfg.GuestPath, DefaultBatchGuestPath,
	)
	if err != nil {
		return nil, err
	}

	leaves, err := r.loadBatchLeaves(cfg.LeafClaimKind, cfg.LeafInputs)
	if err != nil {
		return nil, err
	}

	stdin, err := buildBatchWitnessStdin(
		cfg.LeafClaimKind, leaves.leafGuestImageID, leaves.journals,
	)
	if err != nil {
		return nil, err
	}

	result, err := r.client.Prove(
		zkvmhost.ProveRequest{
			GuestBinary: guestBinary,
			Stdin:       stdin,
			Assumptions: leaves.assumptions,
		},
		zkvmhost.WithReceiptKind(resolveReceiptKind(cfg.ReceiptKind)),
	)
	if err != nil {
		return nil, fmt.Errorf("prove batch guest: %w", err)
	}

	claim, err := DecodeBatchClaim(result.Journal)
	if err != nil {
		return nil, err
	}
	if claim.Version != batch.Version {
		return nil, fmt.Errorf(
			"unexpected batch claim version: got %d, want %d",
			claim.Version, batch.Version,
		)
	}

	claimFile := NewBatchClaimFile(
		imageID, claim, result.Journal, result.SealBytes,
		result.ReceiptEncoding,
	)
	if err := writeReceipt(
		cfg.ReceiptOutputPath, result.Receipt,
	); err != nil {
		return nil, err
	}
	if err := WriteBatchClaimFile(
		cfg.ClaimOutputPath, claimFile,
	); err != nil {
		return nil, err
	}

	return &BatchProveReport{
		GuestPath:         guestPath,
		GuestSize:         len(guestBinary),
		ImageID:           imageID,
		LeafCount:         claim.LeafCount,
		Claim:             claim,
		ReceiptOutputPath: cfg.ReceiptOutputPath,
		ClaimOutputPath:   cfg.ClaimOutputPath,
		JournalSize:       len(result.Journal),
		ReceiptEncoding:   result.ReceiptEncoding,
		ReceiptKind:       result.ReceiptKind,
		ProverName:        result.ProverName,
		SealBytes:         result.SealBytes,
	}, nil
}

// VerifyBatch checks a stored batch receipt against the current batch guest
// image ID and optionally verifies one sparse inclusion proof.
func (r *Runner) VerifyBatch(
	cfg BatchVerifyConfig,
) (*BatchVerifyReport, error) {

	if cfg.ReceiptInputPath == "" {
		return nil, errors.New("--receipt-in is required")
	}

	guestPath, guestBinary, imageID, err := r.loadGuestWithDefault(
		cfg.GuestPath, DefaultBatchGuestPath,
	)
	if err != nil {
		return nil, err
	}

	receiptBytes, err := os.ReadFile(cfg.ReceiptInputPath)
	if err != nil {
		return nil, fmt.Errorf(
			"read receipt `%s`: %w", cfg.ReceiptInputPath, err,
		)
	}

	result, err := r.client.Verify(zkvmhost.VerifyRequest{
		Receipt: receiptBytes,
		ImageID: imageID,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"verify batch receipt against image ID: %w", err,
		)
	}

	claim, err := DecodeBatchClaim(result.Journal)
	if err != nil {
		return nil, err
	}

	verifiedClaim := NewBatchClaimFile(
		imageID, claim, result.Journal, result.SealBytes,
		result.ReceiptEncoding,
	)
	if cfg.ClaimInputPath != "" {
		expectedClaim, err := ReadBatchClaimFile(cfg.ClaimInputPath)
		if err != nil {
			return nil, err
		}
		if err := verifyBatchClaimFileMatches(
			expectedClaim, verifiedClaim,
		); err != nil {
			return nil, err
		}
	}
	if cfg.InclusionProofInputPath != "" {
		inclusionProof, err := ReadBatchInclusionProofFile(
			cfg.InclusionProofInputPath,
		)
		if err != nil {
			return nil, err
		}
		if err := verifyBatchInclusionProof(
			claim, inclusionProof,
		); err != nil {
			return nil, err
		}
	}

	return &BatchVerifyReport{
		GuestPath:               guestPath,
		GuestSize:               len(guestBinary),
		ImageID:                 imageID,
		Claim:                   claim,
		ClaimInputPath:          cfg.ClaimInputPath,
		ReceiptInputPath:        cfg.ReceiptInputPath,
		InclusionProofInputPath: cfg.InclusionProofInputPath,
		JournalSize:             len(result.Journal),
		ReceiptEncoding:         result.ReceiptEncoding,
		ReceiptKind:             result.ReceiptKind,
		SealBytes:               result.SealBytes,
	}, nil
}

// DeriveBatchInclusionProof derives one sparse inclusion proof outside the
// guest from an ordered leaf set.
func (r *Runner) DeriveBatchInclusionProof(
	cfg BatchDeriveInclusionConfig,
) (*BatchDeriveInclusionReport, error) {

	if cfg.OutputPath == "" {
		return nil, errors.New("--proof-out is required")
	}

	leaves, err := r.loadBatchLeaves(cfg.LeafClaimKind, cfg.LeafInputs)
	if err != nil {
		return nil, err
	}

	proof, root, err := batch.BuildProof(
		leaves.journals, int(cfg.LeafIndex), sumSHA256Host,
	)
	if err != nil {
		return nil, err
	}

	proofFile := BatchInclusionProofFile{
		SchemaVersion:     1,
		LeafClaimKind:     cfg.LeafClaimKind,
		LeafClaimKindName: batch.LeafKindName(cfg.LeafClaimKind),
		MerkleHashKind:    batch.MerkleHashSHA256,
		MerkleHashKindName: batch.MerkleHashName(
			batch.MerkleHashSHA256,
		),
		LeafIndex:      proof.LeafIndex,
		LeafCount:      proof.LeafCount,
		LeafJournalHex: hexString(proof.LeafClaim),
		Siblings:       encodeDigestHexList(proof.Siblings),
	}
	if err := WriteBatchInclusionProofFile(
		cfg.OutputPath, proofFile,
	); err != nil {
		return nil, err
	}

	return &BatchDeriveInclusionReport{
		LeafClaimKind: cfg.LeafClaimKind,
		LeafCount:     proof.LeafCount,
		LeafIndex:     proof.LeafIndex,
		OutputPath:    cfg.OutputPath,
		MerkleRoot:    root,
	}, nil
}

func hexString(bytes []byte) string {
	return fmt.Sprintf("%x", bytes)
}

func encodeDigestHexList(digests [][32]byte) []string {
	if len(digests) == 0 {
		return nil
	}

	encoded := make([]string, 0, len(digests))
	for _, digest := range digests {
		encoded = append(encoded, hexString(digest[:]))
	}

	return encoded
}
