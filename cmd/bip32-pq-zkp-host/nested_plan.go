package main

import (
	"flag"
	"fmt"
	"os"

	bip32pqzkp "github.com/roasbeef/bip32-pq-zkp"
)

// nestedPlanArgs holds the parsed flags for the one-shot nested-batch plan
// command.
type nestedPlanArgs struct {
	plan string
}

func parseRunNestedBatchPlanArgs(argv []string) (nestedPlanArgs, error) {
	fs := flag.NewFlagSet(
		"run-nested-batch-plan",
		flag.ContinueOnError,
	)
	fs.SetOutput(os.Stderr)

	args := nestedPlanArgs{}
	fs.StringVar(&args.plan, "plan", "", "nested batch plan JSON path")

	if err := fs.Parse(argv); err != nil {
		return nestedPlanArgs{}, err
	}

	return args, nil
}

func runNestedBatchPlan(
	runner *bip32pqzkp.Runner, args nestedPlanArgs,
) error {

	report, err := runner.RunNestedBatchPlan(
		bip32pqzkp.NestedBatchPlanConfig{
			PlanPath: args.plan,
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf("Nested Batch Plan\n")
	fmt.Printf("  Plan: %s\n", report.PlanPath)
	fmt.Printf("  Output dir: %s\n", report.OutputDir)
	fmt.Printf("  Top receipt: %s\n", report.TopReceiptPath)
	fmt.Printf("  Top claim: %s\n", report.TopClaimPath)
	if report.InclusionChainPath != "" {
		fmt.Printf(
			"  Inclusion chain: %s\n",
			report.InclusionChainPath,
		)
	}
	fmt.Printf("  Image ID: %s\n", report.ImageID)
	fmt.Printf("  Verified: %t\n", report.Verified)

	return nil
}
