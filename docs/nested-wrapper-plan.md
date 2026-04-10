# One-Shot Nested Wrapper Plan

## Goal

Reduce the orchestration overhead of the current nested-batch flow.

Today the workflow is split across several Make targets and CLI invocations:

- prove child batches
- derive child inclusion proofs
- prove parent batches
- derive parent inclusion proofs
- bundle the inclusion chain
- verify the final receipt

This works, but it has two costs:

- repeated build / existence checks for `host-ffi` and guest binaries
- repeated `go run` startup and flag plumbing across each step

## Current State

The current building blocks already exist in the `Runner`:

- `ProveBatch`
- `DeriveBatchInclusionProof`
- `BundleBatchInclusionChain`
- `VerifyBatch`

So the missing piece is not proof functionality. It is orchestration.

## Options

### Option A: Bigger Makefile Target

Add one Make target that:

1. builds `host-ffi` once
2. builds `batch-platform-latest` once
3. shells out through the existing CLI subcommands in sequence

Advantages:

- smallest code change
- easy to wire into the existing repo flow

Disadvantages:

- still multiple process launches
- still stringly-typed path passing
- still awkward for arbitrary deeper hierarchies

### Option B: One New CLI Orchestration Command

Add one new host CLI command, for example:

- `prove-nested-batch`
- or `run-nested-batch-plan`

That command would:

1. load one nested-batch plan file
2. create one `Runner`
3. reuse the same host process to prove child batches, derive proofs, build
   the bundled chain, and optionally verify the final parent

Advantages:

- removes repeated `go run` and repeated client startup
- keeps all nested-batch orchestration in one typed Go path
- much easier to extend to deeper hierarchies later

Disadvantages:

- larger code change than a pure Makefile target
- needs a plan/manifest schema

### Option C: Full Planner / DAG Executor

Add a generic nested-batch planner that can execute an arbitrary tree or DAG
of child batches.

Advantages:

- most flexible long-term solution

Disadvantages:

- overkill for the current repo
- too much machinery before the heterogeneous-parent question is settled

## Recommendation

Use Option B.

The right near-term shape is:

- one new CLI orchestration command
- backed by one JSON plan file
- still exposed through one thin Make target

That removes the repeated rebuild checks and process churn while staying close
to the current `Runner` abstraction.

## Recommended Plan File Shape

The first manifest only needs to describe homogeneous nested batches.

A minimal shape is:

```json
{
  "schema_version": 1,
  "guest": "./batch-platform-latest.bin",
  "receipt_kind": "succinct",
  "leaf_kind": "hardened-xpriv",
  "levels": [
    {
      "name": "children",
      "groups": [
        ["leaf-a.claim.json", "leaf-b.claim.json"],
        ["leaf-c.claim.json", "leaf-d.claim.json"]
      ],
      "receipt_paths": [
        ["leaf-a.receipt", "leaf-b.receipt"],
        ["leaf-c.receipt", "leaf-d.receipt"]
      ]
    },
    {
      "name": "parent",
      "group_sources": ["children"]
    }
  ],
  "disclose_path": [0, 1],
  "output_dir": "./artifacts/nested"
}
```

The exact schema can be better than this, but the important idea is:

- child groups are declared once
- the wrapper owns the artifact naming and path plumbing
- one disclosure path identifies which leaf chain to materialize into the
  bundled inclusion artifact

## Recommended Command Behavior

The one-shot command should:

1. validate the manifest
2. preflight the required binaries once
3. instantiate one `Runner`
4. prove all child batches
5. prove the parent batch
6. derive the needed inclusion proofs
7. bundle the chain
8. optionally run final verification
9. emit one summary report

That summary report should include:

- all generated receipt paths
- all generated claim paths
- inclusion proof paths
- bundled inclusion-chain path
- top-level image ID
- top-level Merkle root
- receipt kind
- total wall clock

## Why This Helps Even Before Heterogeneous Parents

The current homogeneous nested design is already real enough to benefit from
better orchestration:

- three-level hierarchy is validated
- bundled inclusion-chain verification exists
- parent subtree policy is enforced

So a one-shot wrapper is not speculative. It would just remove friction from a
workflow we already know is valid.

## Interaction With Heterogeneous Parents

The wrapper should stay one layer above the leaf schema details.

That means:

- the first wrapper only needs to understand the current homogeneous plan
- if heterogeneous parents are added later, the manifest can grow a new node
  type without rewriting the wrapper architecture

So the wrapper work does not block on heterogeneous-parent design.

## Recommended Implementation Order

### Phase 1: Manifest Schema

1. define `nested_batch_plan_v1`
2. define the artifact layout policy
3. define the summary report schema

Success criterion:

- the orchestration inputs are stable before code is written

### Phase 2: CLI Command

1. add one new orchestration subcommand
2. load the manifest
3. drive the existing `Runner` methods in one process

Success criterion:

- one command can reproduce the current homogeneous nested flow end to end

### Phase 3: Makefile Wrapper

1. add one new Make target that calls the orchestration command
2. keep the lower-level subcommands for debugging and benchmarking

Success criterion:

- users can choose between:
  - one-shot wrapper
  - low-level step-by-step commands

## Non-Goal

This wrapper does not need to be a generic distributed proving scheduler.

It only needs to:

- avoid repeated rebuild checks
- reduce process churn
- make the current nested flow reproducible from one command
