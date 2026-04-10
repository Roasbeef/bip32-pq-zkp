// Package main is the demo-specific CLI for the bip32-pq-zkp proof. It
// provides three families of subcommands for the full Taproot lane and the
// two reduced comparison lanes:
//   - execute / prove / verify
//   - execute-hardened-xpub / prove-hardened-xpub / verify-hardened-xpub
//   - execute-hardened-xpriv / prove-hardened-xpriv / verify-hardened-xpriv
//   - execute-batch / prove-batch / verify-batch / derive-batch-inclusion
//
// This is a thin entrypoint that delegates all heavy lifting to the
// bip32pqzkp.Runner, which in turn uses the go-zkvm/host package.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	bip32pqzkp "github.com/roasbeef/bip32-pq-zkp"
	zkvmhost "github.com/roasbeef/go-zkvm/host"
)

// witnessArgs holds the parsed CLI flags for the private witness input.
type witnessArgs struct {
	guest         string
	seedHex       string
	path          string
	useTestVector bool
	requireBIP86  bool
}

// proveArgs extends witnessArgs with the output artifact paths.
type proveArgs struct {
	witness     witnessArgs
	receiptKind string
	receiptOut  string
	claimOut    string
}

// optionalBool implements flag.Value for a bool flag that distinguishes
// "not set" from "set to false". Used for --require-bip86 in verify mode.
type optionalBool struct {
	set   bool
	value bool
}

// String returns the string form expected by the flag package.
func (o *optionalBool) String() string {
	if !o.set {
		return ""
	}

	return strconv.FormatBool(o.value)
}

// Set parses the optional bool value from the CLI.
func (o *optionalBool) Set(value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}

	o.set = true
	o.value = parsed

	return nil
}

// verifyArgs holds the parsed CLI flags for receipt verification.
type verifyArgs struct {
	guest                  string
	receiptIn              string
	claimIn                string
	expectedPubkey         string
	expectedPathCommitment string
	expectedPath           string
	requireBIP86           optionalBool
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	runner, err := bip32pqzkp.NewRunner()
	if err != nil {
		fatalf("initialize Go host runner: %v", err)
	}
	defer runner.Close()

	switch os.Args[1] {
	case "execute":
		args, err := parseExecuteArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := execute(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "prove":
		args, err := parseProveArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := prove(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "verify":
		args, err := parseVerifyArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := verify(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "execute-hardened-xpub":
		args, err := parseExecuteHardenedXPubArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := executeHardenedXPub(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "prove-hardened-xpub":
		args, err := parseProveHardenedXPubArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := proveHardenedXPub(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "verify-hardened-xpub":
		args, err := parseVerifyHardenedXPubArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := verifyHardenedXPub(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "execute-hardened-xpriv":
		args, err := parseExecuteHardenedXPrivArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := executeHardenedXPriv(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "prove-hardened-xpriv":
		args, err := parseProveHardenedXPrivArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := proveHardenedXPriv(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "verify-hardened-xpriv":
		args, err := parseVerifyHardenedXPrivArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := verifyHardenedXPriv(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "execute-batch":
		args, err := parseExecuteBatchArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := executeBatch(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "prove-batch":
		args, err := parseProveBatchArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := proveBatch(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "verify-batch":
		args, err := parseVerifyBatchArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := verifyBatch(runner, args); err != nil {
			fatalf("%v", err)
		}

	case "derive-batch-inclusion":
		args, err := parseDeriveBatchInclusionArgs(os.Args[2:])
		if err != nil {
			fatalf("%v", err)
		}

		if err := deriveBatchInclusion(runner, args); err != nil {
			fatalf("%v", err)
		}

	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(
		os.Stderr,
		"usage: %s <execute|prove|verify|"+
			"execute-hardened-xpub|prove-hardened-xpub|"+
			"verify-hardened-xpub|execute-hardened-xpriv|"+
			"prove-hardened-xpriv|verify-hardened-xpriv|"+
			"execute-batch|prove-batch|verify-batch|"+
			"derive-batch-inclusion> [flags]\n",
		filepath.Base(os.Args[0]),
	)
}

func parseExecuteArgs(argv []string) (witnessArgs, error) {
	fs := flag.NewFlagSet("execute", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := witnessArgs{}
	fs.StringVar(
		&args.guest, "guest", bip32pqzkp.DefaultGuestPath,
		"guest binary path",
	)
	fs.StringVar(&args.seedHex, "seed-hex", "", "private seed hex")
	fs.StringVar(&args.path, "path", "", "BIP-32 derivation path")
	fs.BoolVar(
		&args.useTestVector, "use-test-vector", false,
		"use the built-in BIP-32 test vector",
	)
	fs.BoolVar(
		&args.requireBIP86, "require-bip86", true,
		"require the path to match BIP-86 (default true)",
	)

	if err := fs.Parse(argv); err != nil {
		return witnessArgs{}, err
	}

	return args, nil
}

func parseProveArgs(argv []string) (proveArgs, error) {
	fs := flag.NewFlagSet("prove", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := proveArgs{}
	fs.StringVar(
		&args.witness.guest, "guest", bip32pqzkp.DefaultGuestPath,
		"guest binary path",
	)
	fs.StringVar(&args.witness.seedHex, "seed-hex", "", "private seed hex")
	fs.StringVar(
		&args.witness.path, "path", "", "BIP-32 derivation path",
	)
	fs.BoolVar(
		&args.witness.useTestVector, "use-test-vector", false,
		"use the built-in BIP-32 test vector",
	)
	fs.BoolVar(
		&args.witness.requireBIP86, "require-bip86", true,
		"require the path to match BIP-86 (default true)",
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
		return proveArgs{}, err
	}

	return args, nil
}

func parseVerifyArgs(argv []string) (verifyArgs, error) {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := verifyArgs{
		requireBIP86: optionalBool{
			set:   true,
			value: true,
		},
	}
	fs.StringVar(
		&args.guest, "guest", bip32pqzkp.DefaultGuestPath,
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
		&args.expectedPubkey, "expected-pubkey", "",
		"expected taproot output key",
	)
	fs.StringVar(
		&args.expectedPathCommitment, "expected-path-commitment", "",
		"expected path commitment",
	)
	fs.StringVar(
		&args.expectedPath, "expected-path", "",
		"expected BIP-32 derivation path",
	)
	fs.Var(
		&args.requireBIP86, "require-bip86",
		"expected require-bip86 flag (default true)",
	)

	if err := fs.Parse(argv); err != nil {
		return verifyArgs{}, err
	}

	return args, nil
}

// execute runs the guest in execute-only mode (no proof generation) and
// prints the decoded public claim plus session metadata.
func execute(runner *bip32pqzkp.Runner, args witnessArgs) error {
	report, err := runner.Execute(bip32pqzkp.ExecuteConfig{
		GuestPath: args.guest,
		Witness:   witnessConfigFromArgs(args),
	})
	if err != nil {
		return err
	}

	fmt.Printf(
		"✓ Loaded guest binary `%s`: %d bytes\n",
		report.GuestPath, report.GuestSize,
	)
	fmt.Printf("✓ Image ID: %s\n", report.ImageID)

	printWitnessSummary(args.requireBIP86, report.UsingTestVector)

	fmt.Println("✓ Execution successful")
	printClaim(report.Claim)

	fmt.Println("Session info:")
	fmt.Printf("  Exit code: %s\n", report.ExitCode)
	fmt.Printf("  Journal size: %d bytes\n", report.JournalSize)
	fmt.Printf("  Segments: %d\n", report.SegmentCount)
	fmt.Printf("  Rows: %d\n", report.SessionRows)

	return nil
}

// prove generates a STARK proof and writes the receipt and claim.json
// verifier artifacts to the paths specified in args.
func prove(runner *bip32pqzkp.Runner, args proveArgs) error {
	report, err := runner.Prove(bip32pqzkp.ProveConfig{
		GuestPath:         args.witness.guest,
		Witness:           witnessConfigFromArgs(args.witness),
		ReceiptKind:       zkvmhost.ReceiptKind(args.receiptKind),
		ReceiptOutputPath: args.receiptOut,
		ClaimOutputPath:   args.claimOut,
	})
	if err != nil {
		return err
	}

	fmt.Printf(
		"✓ Loaded guest binary `%s`: %d bytes\n",
		report.GuestPath, report.GuestSize,
	)
	fmt.Printf("✓ Image ID: %s\n", report.ImageID)

	printWitnessSummary(args.witness.requireBIP86, report.UsingTestVector)

	fmt.Printf("✓ Using prover backend: %s\n", report.ProverName)
	fmt.Printf("✓ Receipt kind: %s\n", report.ReceiptKind)
	printAccelerationStatus()

	fmt.Println("✓ Proof generated and self-verified")
	printClaim(report.Claim)

	fmt.Println("Artifacts:")
	fmt.Printf("  Receipt: %s\n", report.ReceiptOutputPath)
	fmt.Printf("  Canonical claim.json: %s\n", report.ClaimOutputPath)

	fmt.Println("Receipt info:")
	fmt.Printf("  Journal size: %d bytes\n", report.JournalSize)
	fmt.Printf("  Receipt kind: %s\n", report.ReceiptKind)
	fmt.Printf("  Proof seal size: %d bytes\n", report.SealBytes)

	return nil
}

// verify checks a stored receipt against the current guest image ID and
// compares the decoded claim against the canonical claim.json or explicit
// per-field expectations.
func verify(runner *bip32pqzkp.Runner, args verifyArgs) error {
	report, err := runner.Verify(bip32pqzkp.VerifyConfig{
		GuestPath:        args.guest,
		ReceiptInputPath: args.receiptIn,
		ClaimInputPath:   args.claimIn,
		Expectations:     verifyExpectationsFromArgs(args),
	})
	if err != nil {
		return err
	}

	fmt.Printf(
		"✓ Loaded guest binary `%s`: %d bytes\n",
		report.GuestPath, report.GuestSize,
	)
	fmt.Printf("✓ Computed image ID: %s\n", report.ImageID)

	fmt.Println("✓ Receipt verified")
	printClaim(report.Claim)

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

// printWitnessSummary prints a one-line summary of the witness mode used.
func printWitnessSummary(requireBIP86, usingTestVector bool) {
	witnessDesc := "private BIP-32 witness"
	if requireBIP86 {
		witnessDesc = "private BIP-32 witness with BIP-86 policy"
	}

	if usingTestVector {
		fmt.Printf("✓ Sending %s (built-in test vector)\n", witnessDesc)
		return
	}

	fmt.Printf("✓ Sending %s\n", witnessDesc)
}

// printClaim prints the decoded public claim fields in human-readable form.
func printClaim(claim bip32pqzkp.PublicClaim) {
	fmt.Println("Claim:")
	fmt.Printf("  Version: %d\n", claim.Version)
	fmt.Printf("  Flags: %d\n", claim.Flags)
	fmt.Printf("  Require BIP-86: %v\n", claim.RequireBIP86())
	fmt.Printf("  Taproot output key: %s\n", claim.TaprootOutputKeyHex())
	fmt.Printf("  Path commitment: %s\n", claim.PathCommitmentHex())
}

// printAccelerationStatus reports whether Metal GPU acceleration is available
// for local proving on Apple Silicon.
func printAccelerationStatus() {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		if os.Getenv("RISC0_FORCE_CPU_PROVER") != "" {
			fmt.Println("! Metal acceleration disabled by " +
				"RISC0_FORCE_CPU_PROVER=1")
		} else {
			fmt.Println(
				"✓ Metal acceleration compiled in " +
					"for local proving",
			)
		}
	}
}

// witnessConfigFromArgs converts parsed CLI flags into a WitnessConfig.
func witnessConfigFromArgs(args witnessArgs) bip32pqzkp.WitnessConfig {
	return bip32pqzkp.WitnessConfig{
		SeedHex:       args.seedHex,
		Path:          args.path,
		UseTestVector: args.useTestVector,
		RequireBIP86:  args.requireBIP86,
	}
}

// verifyExpectationsFromArgs converts parsed verify CLI flags into a
// VerifyExpectations struct.
func verifyExpectationsFromArgs(args verifyArgs) bip32pqzkp.VerifyExpectations {
	expectations := bip32pqzkp.VerifyExpectations{
		PubKeyHex:         args.expectedPubkey,
		PathCommitmentHex: args.expectedPathCommitment,
		Path:              args.expectedPath,
	}
	if args.requireBIP86.set {
		expectations.RequireBIP86 = &args.requireBIP86.value
	}

	return expectations
}

// fatalf prints a formatted error to stderr and exits with code 1.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
