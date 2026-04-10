// nested_plan_test.go validates the manifest-driven nested batch plan parsing
// and the artifact name generation helpers without invoking the actual prover.

package bip32pqzkp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNestedNodeArtifactBase(t *testing.T) {
	t.Parallel()

	got := nestedNodeArtifactBase("Top Batch", []int{0, 2})
	want := "node-00-02-top-batch"
	if got != want {
		t.Fatalf("nestedNodeArtifactBase = %q, want %q", got, want)
	}
}

func TestReadNestedBatchPlanFile(t *testing.T) {
	t.Parallel()

	planPath := filepath.Join(t.TempDir(), "plan.json")
	const planJSON = `{
  "schema_version": 1,
  "output_dir": "./out",
  "top": {
    "leaf_kind": "hardened-xpriv",
    "leaves": [
      {
        "claim": "a.claim.json",
        "receipt": "a.receipt"
      }
    ]
  }
}`
	if err := os.WriteFile(planPath, []byte(planJSON), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	plan, err := ReadNestedBatchPlanFile(planPath)
	if err != nil {
		t.Fatalf("ReadNestedBatchPlanFile failed: %v", err)
	}
	if plan.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", plan.SchemaVersion)
	}
	if plan.Top.LeafKind != "hardened-xpriv" {
		t.Fatalf(
			"Top.LeafKind = %q, want hardened-xpriv",
			plan.Top.LeafKind,
		)
	}
	if len(plan.Top.Leaves) != 1 {
		t.Fatalf("got %d top leaves, want 1", len(plan.Top.Leaves))
	}
}
