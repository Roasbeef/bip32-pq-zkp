package main

import (
	"flag"
	"fmt"
	"os"

	bip32pqzkp "github.com/roasbeef/bip32-pq-zkp"
	zkvmhost "github.com/roasbeef/go-zkvm/host"
)

type hardenedXPubWitnessArgs struct {
	guest              string
	parentXPrivHex     string
	parentChainCodeHex string
	path               string
	useTestVector      bool
}

type hardenedXPubProveArgs struct {
	witness     hardenedXPubWitnessArgs
	receiptKind string
	receiptOut  string
	claimOut    string
}

type hardenedXPubVerifyArgs struct {
	guest                string
	receiptIn            string
	claimIn              string
	expectedCompressedPk string
	expectedChainCode    string
}

func parseExecuteHardenedXPubArgs(
	argv []string,
) (hardenedXPubWitnessArgs, error) {

	fs := flag.NewFlagSet("execute-hardened-xpub", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := hardenedXPubWitnessArgs{}
	fs.StringVar(
		&args.guest, "guest", bip32pqzkp.DefaultHardenedXPubGuestPath,
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
		&args.path, "path", "", "relative hardened BIP-32 path",
	)
	fs.BoolVar(
		&args.useTestVector, "use-test-vector", false,
		"use the built-in reduced proof test vector",
	)

	if err := fs.Parse(argv); err != nil {
		return hardenedXPubWitnessArgs{}, err
	}

	return args, nil
}

func parseProveHardenedXPubArgs(
	argv []string,
) (hardenedXPubProveArgs, error) {

	fs := flag.NewFlagSet("prove-hardened-xpub", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := hardenedXPubProveArgs{}
	fs.StringVar(
		&args.witness.guest, "guest",
		bip32pqzkp.DefaultHardenedXPubGuestPath,
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
		&args.witness.path, "path", "", "relative hardened BIP-32 path",
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
		return hardenedXPubProveArgs{}, err
	}

	return args, nil
}

func parseVerifyHardenedXPubArgs(
	argv []string,
) (hardenedXPubVerifyArgs, error) {

	fs := flag.NewFlagSet("verify-hardened-xpub", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := hardenedXPubVerifyArgs{}
	fs.StringVar(
		&args.guest, "guest", bip32pqzkp.DefaultHardenedXPubGuestPath,
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
		&args.expectedCompressedPk, "expected-compressed-pubkey", "",
		"expected compressed child xpub",
	)
	fs.StringVar(
		&args.expectedChainCode, "expected-chain-code", "",
		"expected child chain code",
	)

	if err := fs.Parse(argv); err != nil {
		return hardenedXPubVerifyArgs{}, err
	}

	return args, nil
}

func executeHardenedXPub(
	runner *bip32pqzkp.Runner, args hardenedXPubWitnessArgs,
) error {

	report, err := runner.ExecuteHardenedXPub(
		bip32pqzkp.HardenedXPubExecuteConfig{
			GuestPath: args.guest,
			Witness:   hardenedXPubWitnessConfigFromArgs(args),
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

	printHardenedXPubWitnessSummary(report.UsingTestVector)

	fmt.Println("✓ Execution successful")
	printHardenedXPubClaim(report.Claim)

	fmt.Println("Session info:")
	fmt.Printf("  Exit code: %s\n", report.ExitCode)
	fmt.Printf("  Journal size: %d bytes\n", report.JournalSize)
	fmt.Printf("  Segments: %d\n", report.SegmentCount)
	fmt.Printf("  Rows: %d\n", report.SessionRows)

	return nil
}

func proveHardenedXPub(
	runner *bip32pqzkp.Runner, args hardenedXPubProveArgs,
) error {

	report, err := runner.ProveHardenedXPub(
		bip32pqzkp.HardenedXPubProveConfig{
			GuestPath: args.witness.guest,
			Witness: hardenedXPubWitnessConfigFromArgs(
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

	printHardenedXPubWitnessSummary(report.UsingTestVector)

	fmt.Printf("✓ Using prover backend: %s\n", report.ProverName)
	fmt.Printf("✓ Receipt kind: %s\n", report.ReceiptKind)
	printAccelerationStatus()

	fmt.Println("✓ Proof generated and self-verified")
	printHardenedXPubClaim(report.Claim)

	fmt.Println("Artifacts:")
	fmt.Printf("  Receipt: %s\n", report.ReceiptOutputPath)
	fmt.Printf("  Canonical claim.json: %s\n", report.ClaimOutputPath)

	fmt.Println("Receipt info:")
	fmt.Printf("  Journal size: %d bytes\n", report.JournalSize)
	fmt.Printf("  Receipt kind: %s\n", report.ReceiptKind)
	fmt.Printf("  Proof seal size: %d bytes\n", report.SealBytes)

	return nil
}

func verifyHardenedXPub(
	runner *bip32pqzkp.Runner, args hardenedXPubVerifyArgs,
) error {

	report, err := runner.VerifyHardenedXPub(
		bip32pqzkp.HardenedXPubVerifyConfig{
			GuestPath:        args.guest,
			ReceiptInputPath: args.receiptIn,
			ClaimInputPath:   args.claimIn,
			Expectations: bip32pqzkp.
				HardenedXPubVerifyExpectations{
				CompressedPubKeyHex: args.expectedCompressedPk,
				ChainCodeHex:        args.expectedChainCode,
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
	printHardenedXPubClaim(report.Claim)

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

func printHardenedXPubWitnessSummary(usingTestVector bool) {
	if usingTestVector {
		fmt.Println(
			"✓ Sending private parent xpriv witness " +
				"(built-in hardened-xpub test vector)",
		)
		return
	}

	fmt.Println("✓ Sending private parent xpriv witness")
}

func printHardenedXPubClaim(claim bip32pqzkp.HardenedXPubClaim) {
	fmt.Println("Claim:")
	fmt.Printf("  Version: %d\n", claim.Version)
	fmt.Printf("  Flags: %d\n", claim.Flags)
	fmt.Printf("  Compressed pubkey: %s\n", claim.CompressedPubKeyHex())
	fmt.Printf("  Chain code: %s\n", claim.ChainCodeHex())
}

func hardenedXPubWitnessConfigFromArgs(
	args hardenedXPubWitnessArgs,
) bip32pqzkp.HardenedXPubWitnessConfig {

	return bip32pqzkp.HardenedXPubWitnessConfig{
		ParentPrivateKeyHex: args.parentXPrivHex,
		ParentChainCodeHex:  args.parentChainCodeHex,
		Path:                args.path,
		UseTestVector:       args.useTestVector,
	}
}
