package main

import (
	"flag"
	"fmt"
	"os"

	bip32pqzkp "github.com/roasbeef/bip32-pq-zkp"
	zkvmhost "github.com/roasbeef/go-zkvm/host"
)

type hardenedXPrivWitnessArgs struct {
	guest              string
	parentXPrivHex     string
	parentChainCodeHex string
	path               string
	useTestVector      bool
}

type hardenedXPrivProveArgs struct {
	witness     hardenedXPrivWitnessArgs
	receiptKind string
	receiptOut  string
	claimOut    string
}

type hardenedXPrivVerifyArgs struct {
	guest                   string
	receiptIn               string
	claimIn                 string
	expectedChildPrivateKey string
	expectedChainCode       string
}

func parseExecuteHardenedXPrivArgs(
	argv []string,
) (hardenedXPrivWitnessArgs, error) {

	fs := flag.NewFlagSet("execute-hardened-xpriv", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := hardenedXPrivWitnessArgs{}
	fs.StringVar(
		&args.guest, "guest", bip32pqzkp.DefaultHardenedXPrivGuestPath,
		"guest binary path",
	)
	fs.StringVar(
		&args.parentXPrivHex, "parent-xpriv-hex", "",
		"serialized parent xpriv scalar",
	)
	fs.StringVar(
		&args.parentChainCodeHex, "parent-chain-code-hex", "",
		"parent BIP-32 chain code",
	)
	fs.StringVar(
		&args.path, "path", "", "single hardened child step",
	)
	fs.BoolVar(
		&args.useTestVector, "use-test-vector", false,
		"use the built-in reduced proof test vector",
	)

	if err := fs.Parse(argv); err != nil {
		return hardenedXPrivWitnessArgs{}, err
	}

	return args, nil
}

func parseProveHardenedXPrivArgs(
	argv []string,
) (hardenedXPrivProveArgs, error) {

	fs := flag.NewFlagSet("prove-hardened-xpriv", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := hardenedXPrivProveArgs{}
	fs.StringVar(
		&args.witness.guest, "guest",
		bip32pqzkp.DefaultHardenedXPrivGuestPath,
		"guest binary path",
	)
	fs.StringVar(
		&args.witness.parentXPrivHex, "parent-xpriv-hex", "",
		"serialized parent xpriv scalar",
	)
	fs.StringVar(
		&args.witness.parentChainCodeHex, "parent-chain-code-hex", "",
		"parent BIP-32 chain code",
	)
	fs.StringVar(
		&args.witness.path, "path", "", "single hardened child step",
	)
	fs.BoolVar(
		&args.witness.useTestVector, "use-test-vector", false,
		"use the built-in reduced proof test vector",
	)
	fs.StringVar(
		&args.receiptKind, "receipt-kind",
		string(zkvmhost.ReceiptKindComposite),
		"proof receipt kind: composite or succinct",
	)
	fs.StringVar(
		&args.receiptOut, "receipt-out", "",
		"where to write the proof receipt",
	)
	fs.StringVar(
		&args.claimOut, "claim-out", "",
		"where to write the canonical claim.json verifier artifact",
	)

	if err := fs.Parse(argv); err != nil {
		return hardenedXPrivProveArgs{}, err
	}

	return args, nil
}

func parseVerifyHardenedXPrivArgs(
	argv []string,
) (hardenedXPrivVerifyArgs, error) {

	fs := flag.NewFlagSet("verify-hardened-xpriv", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := hardenedXPrivVerifyArgs{}
	fs.StringVar(
		&args.guest, "guest", bip32pqzkp.DefaultHardenedXPrivGuestPath,
		"guest binary path",
	)
	fs.StringVar(
		&args.receiptIn, "receipt-in", "", "receipt file to verify",
	)
	fs.StringVar(
		&args.claimIn, "claim-in", "",
		"canonical claim.json file to compare against",
	)
	fs.StringVar(
		&args.expectedChildPrivateKey, "expected-child-private-key", "",
		"expected child xpriv bytes",
	)
	fs.StringVar(
		&args.expectedChainCode, "expected-chain-code", "",
		"expected child chain code",
	)

	if err := fs.Parse(argv); err != nil {
		return hardenedXPrivVerifyArgs{}, err
	}

	return args, nil
}

func executeHardenedXPriv(
	runner *bip32pqzkp.Runner, args hardenedXPrivWitnessArgs,
) error {

	report, err := runner.ExecuteHardenedXPriv(
		bip32pqzkp.HardenedXPrivExecuteConfig{
			GuestPath: args.guest,
			Witness:   hardenedXPrivWitnessConfigFromArgs(args),
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf(
		"✓ Loaded guest binary `%s`: %d bytes\n",
		report.GuestPath, report.GuestSize,
	)
	fmt.Printf("✓ Image ID: %s\n", report.ImageID)

	printHardenedXPrivWitnessSummary(report.UsingTestVector)

	fmt.Println("✓ Execution successful")
	printHardenedXPrivClaim(report.Claim)

	fmt.Println("Session info:")
	fmt.Printf("  Exit code: %s\n", report.ExitCode)
	fmt.Printf("  Journal size: %d bytes\n", report.JournalSize)
	fmt.Printf("  Segments: %d\n", report.SegmentCount)
	fmt.Printf("  Rows: %d\n", report.SessionRows)

	return nil
}

func proveHardenedXPriv(
	runner *bip32pqzkp.Runner, args hardenedXPrivProveArgs,
) error {

	report, err := runner.ProveHardenedXPriv(
		bip32pqzkp.HardenedXPrivProveConfig{
			GuestPath: args.witness.guest,
			Witness: hardenedXPrivWitnessConfigFromArgs(
				args.witness,
			),
			ReceiptKind: zkvmhost.ReceiptKind(
				args.receiptKind,
			),
			ReceiptOutputPath: args.receiptOut,
			ClaimOutputPath:   args.claimOut,
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf(
		"✓ Loaded guest binary `%s`: %d bytes\n",
		report.GuestPath, report.GuestSize,
	)
	fmt.Printf("✓ Image ID: %s\n", report.ImageID)

	printHardenedXPrivWitnessSummary(report.UsingTestVector)

	fmt.Printf("✓ Using prover backend: %s\n", report.ProverName)
	fmt.Printf("✓ Receipt kind: %s\n", report.ReceiptKind)
	printAccelerationStatus()

	fmt.Println("✓ Proof generated and self-verified")
	printHardenedXPrivClaim(report.Claim)

	fmt.Println("Artifacts:")
	fmt.Printf("  Receipt: %s\n", report.ReceiptOutputPath)
	fmt.Printf("  Canonical claim.json: %s\n", report.ClaimOutputPath)

	fmt.Println("Receipt info:")
	fmt.Printf("  Journal size: %d bytes\n", report.JournalSize)
	fmt.Printf("  Receipt kind: %s\n", report.ReceiptKind)
	fmt.Printf("  Proof seal size: %d bytes\n", report.SealBytes)

	return nil
}

func verifyHardenedXPriv(
	runner *bip32pqzkp.Runner, args hardenedXPrivVerifyArgs,
) error {

	report, err := runner.VerifyHardenedXPriv(
		bip32pqzkp.HardenedXPrivVerifyConfig{
			GuestPath:        args.guest,
			ReceiptInputPath: args.receiptIn,
			ClaimInputPath:   args.claimIn,
			Expectations: bip32pqzkp.
				HardenedXPrivVerifyExpectations{
				ChildPrivateKeyHex: args.
					expectedChildPrivateKey,
				ChainCodeHex: args.expectedChainCode,
			},
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf(
		"✓ Loaded guest binary `%s`: %d bytes\n",
		report.GuestPath, report.GuestSize,
	)
	fmt.Printf("✓ Computed image ID: %s\n", report.ImageID)

	fmt.Println("✓ Receipt verified")
	printHardenedXPrivClaim(report.Claim)

	fmt.Println("Receipt info:")
	fmt.Printf("  Journal size: %d bytes\n", report.JournalSize)
	fmt.Printf("  Receipt kind: %s\n", report.ReceiptKind)
	fmt.Printf("  Proof seal size: %d bytes\n", report.SealBytes)
	fmt.Printf("  Receipt file: %s\n", report.ReceiptInputPath)
	if report.ClaimInputPath != "" {
		fmt.Printf(
			"  Canonical claim.json: %s\n",
			report.ClaimInputPath,
		)
	}

	return nil
}

func printHardenedXPrivWitnessSummary(usingTestVector bool) {
	if usingTestVector {
		fmt.Println(
			"✓ Sending private parent xpriv witness " +
				"(built-in hardened-xpriv test vector)",
		)
		return
	}

	fmt.Println("✓ Sending private parent xpriv witness")
}

func printHardenedXPrivClaim(claim bip32pqzkp.HardenedXPrivClaim) {
	fmt.Println("Claim:")
	fmt.Printf("  Version: %d\n", claim.Version)
	fmt.Printf("  Flags: %d\n", claim.Flags)
	fmt.Printf("  Child private key: %s\n", claim.ChildPrivateKeyHex())
	fmt.Printf("  Chain code: %s\n", claim.ChainCodeHex())
}

func hardenedXPrivWitnessConfigFromArgs(
	args hardenedXPrivWitnessArgs,
) bip32pqzkp.HardenedXPrivWitnessConfig {

	return bip32pqzkp.HardenedXPrivWitnessConfig{
		ParentPrivateKeyHex: args.parentXPrivHex,
		ParentChainCodeHex:  args.parentChainCodeHex,
		Path:                args.path,
		UseTestVector:       args.useTestVector,
	}
}
