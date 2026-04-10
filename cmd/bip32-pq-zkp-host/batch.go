// batch.go implements the CLI flag parsing, execution, proving,
// verification, and inclusion-proof derivation for the batch aggregation
// subcommands. Leaf receipts and claim files are specified via repeatable
// --leaf-receipt and --leaf-claim flags that must appear in the same order.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	bip32pqzkp "github.com/roasbeef/bip32-pq-zkp"
	batch "github.com/roasbeef/bip32-pq-zkp/batchclaim"
	zkvmhost "github.com/roasbeef/go-zkvm/host"
)

// batchLeafList implements flag.Value for repeatable string flags.
type batchLeafList []string

func (l *batchLeafList) String() string {
	return strings.Join(*l, ",")
}

func (l *batchLeafList) Set(value string) error {
	*l = append(*l, value)
	return nil
}

type batchArgs struct {
	guest        string
	leafKind     string
	leafClaims   batchLeafList
	leafReceipts batchLeafList
}

type batchProveArgs struct {
	batchArgs
	receiptKind string
	receiptOut  string
	claimOut    string
}

type batchVerifyArgs struct {
	guest       string
	receiptIn   string
	claimIn     string
	inclusionIn string
}

type batchInclusionArgs struct {
	batchArgs
	leafIndex uint
	proofOut  string
}

func parseExecuteBatchArgs(argv []string) (batchArgs, error) {
	fs := flag.NewFlagSet("execute-batch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := batchArgs{}
	fs.StringVar(
		&args.guest, "guest", bip32pqzkp.DefaultBatchGuestPath,
		"batch guest binary path",
	)
	fs.StringVar(
		&args.leafKind, "leaf-kind", "hardened-xpriv",
		"leaf claim kind: hardened-xpriv or taproot",
	)
	fs.Var(&args.leafClaims, "leaf-claim", "leaf claim.json path (repeat)")
	fs.Var(
		&args.leafReceipts, "leaf-receipt",
		"leaf receipt path (repeat, same order as --leaf-claim)",
	)

	if err := fs.Parse(argv); err != nil {
		return batchArgs{}, err
	}

	return args, nil
}

func parseProveBatchArgs(argv []string) (batchProveArgs, error) {
	fs := flag.NewFlagSet("prove-batch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := batchProveArgs{}
	fs.StringVar(
		&args.guest, "guest", bip32pqzkp.DefaultBatchGuestPath,
		"batch guest binary path",
	)
	fs.StringVar(
		&args.leafKind, "leaf-kind", "hardened-xpriv",
		"leaf claim kind: hardened-xpriv or taproot",
	)
	fs.Var(&args.leafClaims, "leaf-claim", "leaf claim.json path (repeat)")
	fs.Var(
		&args.leafReceipts, "leaf-receipt",
		"leaf receipt path (repeat, same order as --leaf-claim)",
	)
	fs.StringVar(
		&args.receiptKind, "receipt-kind",
		string(zkvmhost.ReceiptKindComposite),
		"proof receipt kind: composite or succinct",
	)
	fs.StringVar(
		&args.receiptOut, "receipt-out", "",
		"where to write the batch receipt",
	)
	fs.StringVar(
		&args.claimOut, "claim-out", "",
		"where to write the batch claim.json",
	)

	if err := fs.Parse(argv); err != nil {
		return batchProveArgs{}, err
	}

	return args, nil
}

func parseVerifyBatchArgs(argv []string) (batchVerifyArgs, error) {
	fs := flag.NewFlagSet("verify-batch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := batchVerifyArgs{}
	fs.StringVar(
		&args.guest, "guest", bip32pqzkp.DefaultBatchGuestPath,
		"batch guest binary path",
	)
	fs.StringVar(
		&args.receiptIn, "receipt-in", "", "batch receipt to verify",
	)
	fs.StringVar(
		&args.claimIn, "claim-in", "",
		"batch claim.json to compare against",
	)
	fs.StringVar(
		&args.inclusionIn, "inclusion-in", "",
		"optional sparse inclusion proof file to verify too",
	)

	if err := fs.Parse(argv); err != nil {
		return batchVerifyArgs{}, err
	}

	return args, nil
}

func parseDeriveBatchInclusionArgs(argv []string) (batchInclusionArgs, error) {
	fs := flag.NewFlagSet("derive-batch-inclusion", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	args := batchInclusionArgs{}
	fs.StringVar(
		&args.guest, "guest", bip32pqzkp.DefaultBatchGuestPath,
		"batch guest binary path (unused, reserved for symmetry)",
	)
	fs.StringVar(
		&args.leafKind, "leaf-kind", "hardened-xpriv",
		"leaf claim kind: hardened-xpriv or taproot",
	)
	fs.Var(&args.leafClaims, "leaf-claim", "leaf claim.json path (repeat)")
	fs.Var(
		&args.leafReceipts, "leaf-receipt",
		"leaf receipt path (repeat, same order as --leaf-claim)",
	)
	fs.UintVar(&args.leafIndex, "leaf-index", 0, "leaf index to disclose")
	fs.StringVar(
		&args.proofOut, "proof-out", "",
		"where to write the inclusion proof",
	)

	if err := fs.Parse(argv); err != nil {
		return batchInclusionArgs{}, err
	}

	return args, nil
}

func executeBatch(runner *bip32pqzkp.Runner, args batchArgs) error {
	leafInputs, leafKind, err := parseBatchLeafInputs(args)
	if err != nil {
		return err
	}

	report, err := runner.ExecuteBatch(bip32pqzkp.BatchExecuteConfig{
		GuestPath:     args.guest,
		LeafClaimKind: leafKind,
		LeafInputs:    leafInputs,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Batch Execute\n")
	fmt.Printf("  Guest: %s\n", report.GuestPath)
	fmt.Printf("  Image ID: %s\n", report.ImageID)
	fmt.Printf("  Leaf count: %d\n", report.LeafCount)
	fmt.Printf("  Exit code: %s\n", report.ExitCode)
	fmt.Printf("  Segments: %d\n", report.SegmentCount)
	fmt.Printf("  Session rows: %d\n", report.SessionRows)
	printBatchClaim(report.Claim)
	return nil
}

func proveBatch(runner *bip32pqzkp.Runner, args batchProveArgs) error {
	leafInputs, leafKind, err := parseBatchLeafInputs(args.batchArgs)
	if err != nil {
		return err
	}

	report, err := runner.ProveBatch(bip32pqzkp.BatchProveConfig{
		GuestPath:         args.guest,
		LeafClaimKind:     leafKind,
		LeafInputs:        leafInputs,
		ReceiptKind:       zkvmhost.ReceiptKind(args.receiptKind),
		ReceiptOutputPath: args.receiptOut,
		ClaimOutputPath:   args.claimOut,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Batch Proof\n")
	fmt.Printf("  Guest: %s\n", report.GuestPath)
	fmt.Printf("  Image ID: %s\n", report.ImageID)
	fmt.Printf("  Leaf count: %d\n", report.LeafCount)
	fmt.Printf("  Receipt kind: %s\n", report.ReceiptKind)
	fmt.Printf("  Prover: %s\n", report.ProverName)
	fmt.Printf("  Seal bytes: %d\n", report.SealBytes)
	fmt.Printf("  Receipt: %s\n", report.ReceiptOutputPath)
	fmt.Printf("  Claim: %s\n", report.ClaimOutputPath)
	printBatchClaim(report.Claim)
	return nil
}

func verifyBatch(runner *bip32pqzkp.Runner, args batchVerifyArgs) error {
	report, err := runner.VerifyBatch(bip32pqzkp.BatchVerifyConfig{
		GuestPath:               args.guest,
		ReceiptInputPath:        args.receiptIn,
		ClaimInputPath:          args.claimIn,
		InclusionProofInputPath: args.inclusionIn,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Batch Verify\n")
	fmt.Printf("  Guest: %s\n", report.GuestPath)
	fmt.Printf("  Image ID: %s\n", report.ImageID)
	fmt.Printf("  Receipt: %s\n", report.ReceiptInputPath)
	if report.ClaimInputPath != "" {
		fmt.Printf("  Claim: %s\n", report.ClaimInputPath)
	}
	if report.InclusionProofInputPath != "" {
		fmt.Printf(
			"  Inclusion proof: %s\n",
			report.InclusionProofInputPath,
		)
	}
	fmt.Printf("  Receipt kind: %s\n", report.ReceiptKind)
	fmt.Printf("  Seal bytes: %d\n", report.SealBytes)
	printBatchClaim(report.Claim)
	return nil
}

func deriveBatchInclusion(
	runner *bip32pqzkp.Runner, args batchInclusionArgs,
) error {

	leafInputs, leafKind, err := parseBatchLeafInputs(args.batchArgs)
	if err != nil {
		return err
	}

	report, err := runner.DeriveBatchInclusionProof(
		bip32pqzkp.BatchDeriveInclusionConfig{
			LeafClaimKind: leafKind,
			LeafInputs:    leafInputs,
			LeafIndex:     uint32(args.leafIndex),
			OutputPath:    args.proofOut,
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf("Batch Inclusion Proof\n")
	fmt.Printf(
		"  Leaf kind: %s\n",
		batch.LeafKindName(report.LeafClaimKind),
	)
	fmt.Printf("  Leaf count: %d\n", report.LeafCount)
	fmt.Printf("  Leaf index: %d\n", report.LeafIndex)
	fmt.Printf("  Proof: %s\n", report.OutputPath)
	fmt.Printf("  Merkle root: %x\n", report.MerkleRoot)
	return nil
}

func parseBatchLeafInputs(
	args batchArgs,
) ([]bip32pqzkp.BatchLeafInput, uint32, error) {

	if len(args.leafClaims) == 0 {
		return nil, 0, fmt.Errorf("--leaf-claim is required")
	}
	if len(args.leafClaims) != len(args.leafReceipts) {
		return nil, 0, fmt.Errorf(
			"--leaf-claim and --leaf-receipt must be repeated " +
				"the same number of times",
		)
	}

	var leafKind uint32
	switch args.leafKind {
	case "hardened-xpriv":
		leafKind = batch.LeafKindHardenedXPriv
	case "taproot":
		leafKind = batch.LeafKindTaproot
	default:
		return nil, 0, fmt.Errorf(
			"unsupported --leaf-kind %q", args.leafKind,
		)
	}

	inputs := make([]bip32pqzkp.BatchLeafInput, 0, len(args.leafClaims))
	for i := range args.leafClaims {
		inputs = append(inputs, bip32pqzkp.BatchLeafInput{
			ReceiptPath: args.leafReceipts[i],
			ClaimPath:   args.leafClaims[i],
		})
	}

	return inputs, leafKind, nil
}

func printBatchClaim(claim batch.Claim) {
	fmt.Printf("  Batch claim\n")
	fmt.Printf("    Version: %d\n", claim.Version)
	fmt.Printf("    Flags: %d\n", claim.Flags)
	fmt.Printf(
		"    Leaf kind: %s\n",
		batch.LeafKindName(claim.LeafClaimKind),
	)
	fmt.Printf(
		"    Merkle hash: %s\n",
		batch.MerkleHashName(claim.MerkleHashKind),
	)
	fmt.Printf("    Leaf count: %d\n", claim.LeafCount)
	fmt.Printf("    Leaf guest image ID: %x\n", claim.LeafGuestImageID)
	fmt.Printf("    Merkle root: %x\n", claim.MerkleRoot)
}
