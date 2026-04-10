package bip32pqzkp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	batch "github.com/roasbeef/bip32-pq-zkp/batchclaim"
	host "github.com/roasbeef/go-zkvm/host"
)

// NestedBatchPlanFile is the manifest consumed by the one-shot nested-batch
// wrapper. It describes one recursive batch tree, one disclosure path, and
// where the wrapper should write the generated artifacts.
type NestedBatchPlanFile struct {
	// SchemaVersion identifies the plan schema.
	SchemaVersion uint32 `json:"schema_version"`

	// GuestPath optionally overrides the packaged batch guest binary path.
	GuestPath string `json:"guest,omitempty"`

	// ReceiptKind selects the final
	// receipt kind used when proving
	// each batch node. Defaults to
	// composite if omitted.
	ReceiptKind string `json:"receipt_kind,omitempty"`

	// OutputDir is where the wrapper writes all generated intermediate and
	// final artifacts.
	OutputDir string `json:"output_dir"`

	// VerifyFinal requests that
	// the wrapper verify the
	// final top-level batch
	// receipt before returning.
	VerifyFinal bool `json:"verify_final,omitempty"`

	// DisclosurePath identifies
	// which child index to
	// disclose at each nested
	// level when deriving the
	// bundled inclusion-chain
	// artifact.
	DisclosurePath []uint32 `json:"disclosure_path,omitempty"`

	// Top is the root batch node to build.
	Top NestedBatchPlanNode `json:"top"`
}

// NestedBatchPlanNode is one batch node in a recursive plan. It can aggregate
// homogeneous raw leaves, homogeneous batch claims, or heterogeneous direct
// children via fixed-size envelopes.
type NestedBatchPlanNode struct {
	// Name is an optional human-readable label used in generated artifact
	// names.
	Name string `json:"name,omitempty"`

	// LeafKind identifies the direct-child mode for this node.
	LeafKind string `json:"leaf_kind"`

	// Leaves are the ordered direct children aggregated by this node.
	Leaves []NestedBatchPlanLeaf `json:"leaves"`
}

// NestedBatchPlanLeaf is one direct child in a recursive batch plan. It is
// either:
//
//   - an external claim/receipt pair, or
//   - an inline child batch node that the wrapper proves first.
//
// Heterogeneous parent nodes may additionally set Kind to pin the direct child
// schema carried in the mixed parent envelope.
type NestedBatchPlanLeaf struct {
	// Kind optionally identifies the direct child kind for heterogeneous
	// parent nodes.
	Kind string `json:"kind,omitempty"`

	// Claim is the external child claim artifact path.
	Claim string `json:"claim,omitempty"`

	// Receipt is the external child receipt artifact path.
	Receipt string `json:"receipt,omitempty"`

	// Batch is an inline child batch node that the wrapper should prove
	// recursively before using it as a direct child.
	Batch *NestedBatchPlanNode `json:"batch,omitempty"`
}

// NestedBatchPlanConfig identifies one manifest-driven nested-batch run.
type NestedBatchPlanConfig struct {
	// PlanPath is the JSON manifest to load and execute.
	PlanPath string
}

// NestedBatchPlanReport summarizes one manifest-driven nested-batch run.
type NestedBatchPlanReport struct {
	// PlanPath is the JSON manifest used for the run.
	PlanPath string

	// OutputDir is the directory where artifacts were written.
	OutputDir string

	// TopReceiptPath is the final top-level batch receipt.
	TopReceiptPath string

	// TopClaimPath is the final top-level batch claim JSON.
	TopClaimPath string

	// InclusionChainPath is the
	// bundled nested inclusion-chain
	// artifact, when one was
	// requested.
	InclusionChainPath string

	// ImageID is the computed image ID for the batch guest.
	ImageID string

	// TopClaim is the decoded top-level batch claim.
	TopClaim BatchClaimFile

	// Verified reports whether the
	// wrapper performed a final
	// top-level verify pass.
	Verified bool
}

// nestedResolvedNode is the internal representation of one batch node after
// its children have been recursively proven. It stores the resolved leaf
// inputs, the generated receipt and claim paths, and the decoded claim file
// so that subsequent operations (inclusion derivation, verification) can
// reference the artifacts without re-reading them from disk.
type nestedResolvedNode struct {
	name        string
	leafKind    uint32
	leafInputs  []BatchLeafInput
	leaves      []nestedResolvedLeaf
	receiptPath string
	claimPath   string
	claimFile   BatchClaimFile
	receiptKind host.ReceiptKind
}

// nestedResolvedLeaf is one resolved direct child within a batch node. For
// inline child batches, the child field points at the recursively resolved
// subtree so the inclusion derivation can walk the hierarchy.
type nestedResolvedLeaf struct {
	directKind uint32
	input      BatchLeafInput
	child      *nestedResolvedNode
}

var nestedNodeNameSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// ReadNestedBatchPlanFile loads a manifest-driven nested-batch plan from disk.
func ReadNestedBatchPlanFile(path string) (NestedBatchPlanFile, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return NestedBatchPlanFile{}, fmt.Errorf(
			"read nested plan `%s`: %w", path, err,
		)
	}

	var plan NestedBatchPlanFile
	if err := json.Unmarshal(bytes, &plan); err != nil {
		return NestedBatchPlanFile{}, fmt.Errorf(
			"deserialize nested batch plan JSON: %w", err,
		)
	}

	return plan, nil
}

// RunNestedBatchPlan executes one recursive nested-batch plan end to end.
func (r *Runner) RunNestedBatchPlan(
	cfg NestedBatchPlanConfig,
) (*NestedBatchPlanReport, error) {

	if cfg.PlanPath == "" {
		return nil, errors.New("--plan is required")
	}

	plan, err := ReadNestedBatchPlanFile(cfg.PlanPath)
	if err != nil {
		return nil, err
	}
	if plan.SchemaVersion != 1 {
		return nil, fmt.Errorf(
			"unsupported nested batch plan schema version %d",
			plan.SchemaVersion,
		)
	}
	if plan.OutputDir == "" {
		return nil, errors.New(
			"nested batch plan requires output_dir",
		)
	}
	if err := os.MkdirAll(plan.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf(
			"create nested output dir `%s`: %w",
			plan.OutputDir, err,
		)
	}

	receiptKind := host.ReceiptKindComposite
	if plan.ReceiptKind != "" {
		receiptKind = resolveReceiptKind(
			host.ReceiptKind(plan.ReceiptKind),
		)
	}

	guestPath := plan.GuestPath
	if guestPath == "" {
		guestPath = DefaultBatchGuestPath
	}

	resolvedRoot, err := r.proveNestedBatchNode(
		plan.Top, nil, plan.OutputDir, guestPath, receiptKind,
	)
	if err != nil {
		return nil, err
	}

	var inclusionChainPath string
	if len(plan.DisclosurePath) != 0 {
		chainPath, err := r.deriveNestedPlanChain(
			resolvedRoot, plan.DisclosurePath, plan.OutputDir,
		)
		if err != nil {
			return nil, err
		}
		inclusionChainPath = chainPath
	}

	verified := false
	if plan.VerifyFinal {
		_, err := r.VerifyBatch(BatchVerifyConfig{
			GuestPath:               guestPath,
			ReceiptInputPath:        resolvedRoot.receiptPath,
			ClaimInputPath:          resolvedRoot.claimPath,
			InclusionChainInputPath: inclusionChainPath,
		})
		if err != nil {
			return nil, fmt.Errorf(
				"verify final nested batch: %w", err,
			)
		}
		verified = true
	}

	return &NestedBatchPlanReport{
		PlanPath:           cfg.PlanPath,
		OutputDir:          plan.OutputDir,
		TopReceiptPath:     resolvedRoot.receiptPath,
		TopClaimPath:       resolvedRoot.claimPath,
		InclusionChainPath: inclusionChainPath,
		ImageID:            resolvedRoot.claimFile.ImageID,
		TopClaim:           resolvedRoot.claimFile,
		Verified:           verified,
	}, nil
}

// proveNestedBatchNode recursively resolves and proves one batch node. It
// first resolves each leaf (either loading an external artifact or recursively
// proving an inline child batch), then calls ProveBatch to produce the node's
// own receipt and claim artifacts.
func (r *Runner) proveNestedBatchNode(
	node NestedBatchPlanNode, path []int, outputDir string,
	guestPath string,
	receiptKind host.ReceiptKind,
) (*nestedResolvedNode, error) {

	leafKind, err := ParseBatchLeafKindName(node.LeafKind)
	if err != nil {
		return nil, fmt.Errorf("parse node leaf kind: %w", err)
	}
	if len(node.Leaves) == 0 {
		return nil, errors.New(
			"nested batch node requires at least one leaf",
		)
	}

	resolved := &nestedResolvedNode{
		name:        nestedNodeArtifactBase(node.Name, path),
		leafKind:    leafKind,
		leafInputs:  make([]BatchLeafInput, 0, len(node.Leaves)),
		leaves:      make([]nestedResolvedLeaf, 0, len(node.Leaves)),
		receiptKind: receiptKind,
	}

	for idx, leaf := range node.Leaves {
		resolvedLeaf, err := r.resolveNestedBatchLeaf(
			leafKind, leaf, append(path, idx), outputDir, guestPath,
			receiptKind,
		)
		if err != nil {
			return nil, fmt.Errorf("resolve leaf %d: %w", idx, err)
		}

		resolved.leafInputs = append(
			resolved.leafInputs, resolvedLeaf.input,
		)
		resolved.leaves = append(resolved.leaves, resolvedLeaf)
	}

	receiptPath := filepath.Join(
		outputDir, resolved.name+".receipt",
	)
	claimPath := filepath.Join(
		outputDir, resolved.name+".claim.json",
	)
	report, err := r.ProveBatch(BatchProveConfig{
		GuestPath:         guestPath,
		LeafClaimKind:     leafKind,
		LeafInputs:        resolved.leafInputs,
		ReceiptKind:       receiptKind,
		ReceiptOutputPath: receiptPath,
		ClaimOutputPath:   claimPath,
	})
	if err != nil {
		return nil, fmt.Errorf("prove nested batch node: %w", err)
	}

	claimFile, err := ReadBatchClaimFile(claimPath)
	if err != nil {
		return nil, err
	}

	resolved.receiptPath = report.ReceiptOutputPath
	resolved.claimPath = report.ClaimOutputPath
	resolved.claimFile = claimFile

	return resolved, nil
}

// resolveNestedBatchLeaf resolves one direct child in a batch plan. If the
// leaf has an inline Batch field, it is recursively proven first. For
// heterogeneous parents, the leaf's Kind field is parsed and validated
// against the allowed direct child kinds.
func (r *Runner) resolveNestedBatchLeaf(
	parentLeafKind uint32, leaf NestedBatchPlanLeaf, path []int,
	outputDir string, guestPath string, receiptKind host.ReceiptKind,
) (nestedResolvedLeaf, error) {

	resolved := nestedResolvedLeaf{}
	if leaf.Batch != nil {
		child, err := r.proveNestedBatchNode(
			*leaf.Batch, path, outputDir, guestPath, receiptKind,
		)
		if err != nil {
			return nestedResolvedLeaf{}, err
		}
		resolved.child = child
		resolved.input.ClaimPath = child.claimPath
		resolved.input.ReceiptPath = child.receiptPath
	}

	if resolved.input.ClaimPath == "" {
		if leaf.Claim == "" || leaf.Receipt == "" {
			return nestedResolvedLeaf{}, errors.New(
				"nested batch leaf requires either batch or " +
					"claim+receipt paths",
			)
		}
		resolved.input.ClaimPath = leaf.Claim
		resolved.input.ReceiptPath = leaf.Receipt
	}

	if parentLeafKind != BatchLeafKindHeterogeneousEnvelopeV1 {
		return resolved, nil
	}

	kindName := leaf.Kind
	if kindName == "" && leaf.Batch != nil {
		kindName = "batch-claim-v1"
	}
	if kindName == "" {
		return nestedResolvedLeaf{}, errors.New(
			"heterogeneous parent leaf requires kind",
		)
	}

	directKind, err := ParseBatchLeafKindName(kindName)
	if err != nil {
		return nestedResolvedLeaf{}, err
	}
	if !batch.IsAllowedHeterogeneousDirectLeafKindV1(directKind) {
		return nestedResolvedLeaf{}, fmt.Errorf(
			"unsupported heterogeneous direct leaf kind %q",
			kindName,
		)
	}
	resolved.directKind = directKind
	resolved.input.DirectLeafKind = directKind

	return resolved, nil
}

// deriveNestedPlanChain walks the disclosure path through the resolved batch
// tree and derives one inclusion proof per level, then bundles them into a
// single inclusion-chain artifact. The disclosure path identifies which child
// index to follow at each level from the root down to the final disclosed
// leaf.
func (r *Runner) deriveNestedPlanChain(
	root *nestedResolvedNode, disclosurePath []uint32, outputDir string,
) (string, error) {

	if root == nil {
		return "", errors.New("missing nested root")
	}

	current := root
	proofPaths := make([]string, 0, len(disclosurePath))
	for depth, leafIndex := range disclosurePath {
		if int(leafIndex) >= len(current.leaves) {
			return "", fmt.Errorf(
				"disclosure path index %d out of range for "+
					"node %s",
				leafIndex,
				current.name,
			)
		}

		proofPath := filepath.Join(
			outputDir,
			fmt.Sprintf(
				"%s.level-%02d.inclusion.json",
				current.name,
				depth,
			),
		)
		_, err := r.DeriveBatchInclusionProof(
			BatchDeriveInclusionConfig{
				LeafClaimKind: current.leafKind,
				LeafInputs:    current.leafInputs,
				LeafIndex:     leafIndex,
				OutputPath:    proofPath,
			},
		)
		if err != nil {
			return "", fmt.Errorf(
				"derive inclusion proof for %s level %d: %w",
				current.name, depth, err,
			)
		}
		proofPaths = append(proofPaths, proofPath)

		next := current.leaves[leafIndex].child
		if next == nil {
			if depth != len(disclosurePath)-1 {
				return "", errors.New(
					"disclosure path continues past a " +
						"non-batch leaf",
				)
			}
			break
		}

		current = next
	}

	chainPath := filepath.Join(outputDir, "nested.inclusion-chain.json")
	_, err := r.BundleBatchInclusionChain(
		BatchBundleInclusionChainConfig{
			ProofInputPaths: proofPaths,
			OutputPath:      chainPath,
		},
	)
	if err != nil {
		return "", err
	}

	return chainPath, nil
}

// nestedNodeArtifactBase generates a filesystem-safe artifact base name for
// one batch node. The name encodes the node's position in the tree (as a
// dash-separated path of zero-padded indices) plus an optional sanitized
// human-readable label.
func nestedNodeArtifactBase(name string, path []int) string {
	parts := make([]string, 0, len(path)+1)
	parts = append(parts, "node")
	for _, idx := range path {
		parts = append(parts, fmt.Sprintf("%02d", idx))
	}
	if name != "" {
		sanitized := strings.ToLower(name)
		sanitized = nestedNodeNameSanitizer.ReplaceAllString(
			sanitized, "-",
		)
		sanitized = strings.Trim(sanitized, "-")
		if sanitized != "" {
			parts = append(parts, sanitized)
		}
	}

	return strings.Join(parts, "-")
}
