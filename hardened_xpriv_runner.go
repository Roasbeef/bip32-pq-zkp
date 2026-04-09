package bip32pqzkp

import (
	"errors"
	"fmt"
	"os"

	localbip32 "github.com/roasbeef/bip32-pq-zkp/bip32"
	zkvmhost "github.com/roasbeef/go-zkvm/host"
)

// ExecuteHardenedXPriv runs the hardened-xpriv guest without generating a
// proof and returns the decoded public claim plus session metadata.
func (r *Runner) ExecuteHardenedXPriv(
	cfg HardenedXPrivExecuteConfig,
) (*HardenedXPrivExecuteReport, error) {

	guestPath, guestBinary, imageID, err := r.loadGuestWithDefault(
		cfg.GuestPath, DefaultHardenedXPrivGuestPath,
	)
	if err != nil {
		return nil, err
	}

	stdin, usingTestVector, err := BuildHardenedXPrivWitnessStdin(
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
		return nil, fmt.Errorf("execute hardened xpriv guest: %w", err)
	}

	claim, err := DecodeHardenedXPrivClaim(result.Journal)
	if err != nil {
		return nil, err
	}

	return &HardenedXPrivExecuteReport{
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

// ProveHardenedXPriv runs the hardened-xpriv guest through the prover, writes
// the receipt and claim artifacts, and returns the decoded public claim plus
// proof metadata.
func (r *Runner) ProveHardenedXPriv(
	cfg HardenedXPrivProveConfig,
) (*HardenedXPrivProveReport, error) {

	switch {
	case cfg.ReceiptOutputPath == "":
		return nil, errors.New("--receipt-out is required")

	case cfg.ClaimOutputPath == "":
		return nil, errors.New("--claim-out is required")
	}

	guestPath, guestBinary, imageID, err := r.loadGuestWithDefault(
		cfg.GuestPath, DefaultHardenedXPrivGuestPath,
	)
	if err != nil {
		return nil, err
	}

	stdin, usingTestVector, err := BuildHardenedXPrivWitnessStdin(
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
		return nil, fmt.Errorf("prove hardened xpriv guest: %w", err)
	}

	claim, err := DecodeHardenedXPrivClaim(result.Journal)
	if err != nil {
		return nil, err
	}
	if claim.Version != localbip32.HardenedXPrivClaimVersion {
		return nil, fmt.Errorf(
			"unexpected hardened xpriv claim version: "+
				"got %d, want %d",
			claim.Version, localbip32.HardenedXPrivClaimVersion,
		)
	}

	claimFile := NewHardenedXPrivClaimFile(
		imageID, claim, result.Journal, result.SealBytes,
		result.ReceiptEncoding,
	)

	if err := writeReceipt(
		cfg.ReceiptOutputPath, result.Receipt,
	); err != nil {
		return nil, err
	}
	if err := WriteHardenedXPrivClaimFile(
		cfg.ClaimOutputPath, claimFile,
	); err != nil {
		return nil, err
	}

	return &HardenedXPrivProveReport{
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

// VerifyHardenedXPriv checks a stored hardened-xpriv receipt against the
// current guest image ID and then validates the decoded public claim against
// either the canonical `claim.json` artifact or explicit public expectations.
func (r *Runner) VerifyHardenedXPriv(
	cfg HardenedXPrivVerifyConfig,
) (*HardenedXPrivVerifyReport, error) {

	if cfg.ReceiptInputPath == "" {
		return nil, errors.New("--receipt-in is required")
	}

	guestPath, guestBinary, imageID, err := r.loadGuestWithDefault(
		cfg.GuestPath, DefaultHardenedXPrivGuestPath,
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

	var expectedClaim *HardenedXPrivClaimFile
	if cfg.ClaimInputPath != "" {
		claimFile, err := ReadHardenedXPrivClaimFile(cfg.ClaimInputPath)
		if err != nil {
			return nil, err
		}

		expectedClaim = &claimFile
	}

	if expectedClaim == nil &&
		cfg.Expectations.ChildPrivateKeyHex == "" &&
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

	claim, err := DecodeHardenedXPrivClaim(result.Journal)
	if err != nil {
		return nil, err
	}

	verifiedClaim := NewHardenedXPrivClaimFile(
		imageID, claim, result.Journal, result.SealBytes,
		result.ReceiptEncoding,
	)

	if expectedClaim != nil {
		if err := verifyHardenedXPrivClaimFileMatches(
			*expectedClaim, verifiedClaim,
		); err != nil {
			return nil, err
		}
	}
	if err := verifyHardenedXPrivExpectations(
		cfg.Expectations, claim,
	); err != nil {
		return nil, err
	}

	return &HardenedXPrivVerifyReport{
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
