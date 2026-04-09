package bip32pqzkp

import (
	"errors"
	"fmt"
	"os"

	localbip32 "github.com/roasbeef/bip32-pq-zkp/bip32"
	zkvmhost "github.com/roasbeef/go-zkvm/host"
)

// ExecuteHardenedXPub runs the hardened-xpub guest without generating a proof
// and returns the decoded public claim plus session metadata.
func (r *Runner) ExecuteHardenedXPub(
	cfg HardenedXPubExecuteConfig,
) (*HardenedXPubExecuteReport, error) {

	guestPath, guestBinary, imageID, err := r.loadGuestWithDefault(
		cfg.GuestPath, DefaultHardenedXPubGuestPath,
	)
	if err != nil {
		return nil, err
	}

	stdin, usingTestVector, err := BuildHardenedXPubWitnessStdin(
		cfg.Witness,
	)
	if err != nil {
		return nil, err
	}

	result, err := r.client.Execute(zkvmhost.ExecuteRequest{
		GuestBinary: guestBinary,
		Stdin:       stdin,
	})
	if err != nil {
		return nil, fmt.Errorf("execute hardened xpub guest: %w", err)
	}

	claim, err := DecodeHardenedXPubClaim(result.Journal)
	if err != nil {
		return nil, err
	}

	return &HardenedXPubExecuteReport{
		GuestPath:       guestPath,
		GuestSize:       len(guestBinary),
		ImageID:         imageID,
		UsingTestVector: usingTestVector,
		Claim:           claim,
		ExitCode:        result.ExitCode,
		JournalSize:     len(result.Journal),
		SegmentCount:    result.SegmentCount,
		SessionRows:     result.SessionRows,
	}, nil
}

// ProveHardenedXPub runs the hardened-xpub guest through the prover, writes
// the receipt and claim artifacts, and returns the decoded public claim plus
// proof metadata.
func (r *Runner) ProveHardenedXPub(
	cfg HardenedXPubProveConfig,
) (*HardenedXPubProveReport, error) {

	switch {
	case cfg.ReceiptOutputPath == "":
		return nil, errors.New("--receipt-out is required")

	case cfg.ClaimOutputPath == "":
		return nil, errors.New("--claim-out is required")
	}

	guestPath, guestBinary, imageID, err := r.loadGuestWithDefault(
		cfg.GuestPath, DefaultHardenedXPubGuestPath,
	)
	if err != nil {
		return nil, err
	}

	stdin, usingTestVector, err := BuildHardenedXPubWitnessStdin(
		cfg.Witness,
	)
	if err != nil {
		return nil, err
	}

	result, err := r.client.Prove(zkvmhost.ProveRequest{
		GuestBinary: guestBinary,
		Stdin:       stdin,
	}, zkvmhost.WithReceiptKind(resolveReceiptKind(cfg.ReceiptKind)))
	if err != nil {
		return nil, fmt.Errorf("prove hardened xpub guest: %w", err)
	}

	claim, err := DecodeHardenedXPubClaim(result.Journal)
	if err != nil {
		return nil, err
	}
	if claim.Version != localbip32.HardenedXPubClaimVersion {
		return nil, fmt.Errorf(
			"unexpected hardened xpub claim version: "+
				"got %d, want %d",
			claim.Version, localbip32.HardenedXPubClaimVersion,
		)
	}

	claimFile := NewHardenedXPubClaimFile(
		imageID, claim, result.Journal, result.SealBytes,
		result.ReceiptEncoding,
	)

	if err := writeReceipt(
		cfg.ReceiptOutputPath, result.Receipt,
	); err != nil {
		return nil, err
	}
	if err := WriteHardenedXPubClaimFile(
		cfg.ClaimOutputPath, claimFile,
	); err != nil {
		return nil, err
	}

	return &HardenedXPubProveReport{
		GuestPath:         guestPath,
		GuestSize:         len(guestBinary),
		ImageID:           imageID,
		UsingTestVector:   usingTestVector,
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

// VerifyHardenedXPub checks a stored hardened-xpub receipt against the
// current guest image ID and then validates the decoded public claim against
// either the canonical `claim.json` artifact or explicit public expectations.
func (r *Runner) VerifyHardenedXPub(
	cfg HardenedXPubVerifyConfig,
) (*HardenedXPubVerifyReport, error) {

	if cfg.ReceiptInputPath == "" {
		return nil, errors.New("--receipt-in is required")
	}

	guestPath, guestBinary, imageID, err := r.loadGuestWithDefault(
		cfg.GuestPath, DefaultHardenedXPubGuestPath,
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

	var expectedClaim *HardenedXPubClaimFile
	if cfg.ClaimInputPath != "" {
		claimFile, err := ReadHardenedXPubClaimFile(cfg.ClaimInputPath)
		if err != nil {
			return nil, err
		}

		expectedClaim = &claimFile
	}

	if expectedClaim == nil &&
		cfg.Expectations.CompressedPubKeyHex == "" &&
		cfg.Expectations.ChainCodeHex == "" {

		return nil, errors.New(
			"verify requires --claim-in or at least one explicit " +
				"expectation flag",
		)
	}

	result, err := r.client.Verify(zkvmhost.VerifyRequest{
		Receipt: receiptBytes,
		ImageID: imageID,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"verify receipt against image ID: %w", err,
		)
	}

	claim, err := DecodeHardenedXPubClaim(result.Journal)
	if err != nil {
		return nil, err
	}

	verifiedClaim := NewHardenedXPubClaimFile(
		imageID, claim, result.Journal, result.SealBytes,
		result.ReceiptEncoding,
	)

	if expectedClaim != nil {
		if err := verifyHardenedXPubClaimFileMatches(
			*expectedClaim, verifiedClaim,
		); err != nil {
			return nil, err
		}
	}
	if err := verifyHardenedXPubExpectations(
		cfg.Expectations, claim,
	); err != nil {
		return nil, err
	}

	return &HardenedXPubVerifyReport{
		GuestPath:        guestPath,
		GuestSize:        len(guestBinary),
		ImageID:          imageID,
		Claim:            claim,
		ClaimInputPath:   cfg.ClaimInputPath,
		ReceiptInputPath: cfg.ReceiptInputPath,
		JournalSize:      len(result.Journal),
		ReceiptEncoding:  result.ReceiptEncoding,
		ReceiptKind:      result.ReceiptKind,
		SealBytes:        result.SealBytes,
	}, nil
}
