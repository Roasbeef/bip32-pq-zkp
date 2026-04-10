# Nested Batch Claims Design

## Current Status

The first cut is now implemented:

- parent batches accept `batch-claim-v1` as a direct leaf kind
- parent leaves use the serialized 84-byte child batch `claim.json`
  journal
- `bundle-batch-inclusion-chain` writes one verifier artifact for the full
  inclusion path
- `verify-batch` accepts either that bundled chain or repeated
  `--inclusion-in` flags
- parent batches enforce homogeneous child subtree policy across all
  `batch-claim-v1` leaves:
  - child batch version and flags
  - child leaf kind
  - child Merkle hash kind
  - child leaf guest image ID
- a three-level hierarchy `P = batch(H, L)` is validated end to end, with:
  - top root `8fabf9c04a03e18f47ef37fe23c3bdbfb9984767d77b055b53a5ae10e4d7aaf3`
  - batch guest image ID
    `c864c3b7dcd4c2326de27277115d0899ab5ca59543c642fda3e3a49551552a33`

What is still deferred:

- flat-root-preserving accumulation
- mixed direct leaf kinds at one parent level
- a more efficient one-shot wrapper that avoids rebuilding `host-ffi` and
  `batch-platform-latest` on every Make target step

## Goal

Add the first hierarchical batching layer without redesigning the current
fixed-size batch claim or Merkle construction.

The design target is:

1. build child batches over small chunks of leaves
2. produce one child batch receipt plus one child batch claim per chunk
3. aggregate those child batch claims into a parent batch
4. verify one final parent receipt, then walk one inclusion path per level

This keeps the existing v1 batch model intact while adding:

- incremental construction
- bounded fan-out per proof step
- reuse of earlier chunk proofs
- a path to parallel batch proving

## Non-Goal

This design does **not** preserve one flat Merkle root over all original
leaves across arbitrary chunking. The parent root commits to child batch
claims, not directly to the original leaf claims.

If we later require a single flat root over all original leaves, that is a
separate accumulator redesign. See `mmr-accumulator-sketch.md`.

## Semantics

### Current v1 Batch

Today, one batch receipt proves:

- there exist `N` valid leaf receipts
- all come from the same `leaf_guest_image_id`
- their ordered journals hash to `merkle_root`

The final public claim is:

- `batch_version`
- `batch_flags`
- `leaf_claim_kind`
- `merkle_hash_kind`
- `leaf_count`
- `leaf_guest_image_id`
- `merkle_root`

### Nested v2 Batch

The parent batch would prove:

- there exist `M` valid **child batch receipts**
- all come from the same child batch guest image
- their ordered child-batch journals hash to the parent `merkle_root`

So the parent root no longer commits directly to original leaf journals. It
commits to child batch claims.

That is acceptable as long as the verifier story is explicit:

1. verify the final parent receipt
2. prove one disclosed child batch claim is included in the parent root
3. prove one disclosed original leaf is included in that child batch root

Repeat step 2 if more than one hierarchy level exists.

## Minimal Data Model Extension

The smallest useful extension is a new parent-leaf kind:

```text
LeafKindBatchClaimV1 = 3
```

This represents one serialized child batch claim used as a leaf inside a
higher-level parent batch.

### Why This Is Enough

The existing batch claim already carries the metadata the parent needs:

- child batch version
- child leaf claim kind
- child Merkle hash kind
- child leaf count
- child leaf guest image ID
- child Merkle root

So the parent guest can treat the child batch claim as one opaque fixed-size
leaf record.

## Required Claim And Guest Changes

### 1. Add `LeafKindBatchClaimV1`

Files affected:

- `batchclaim/claim.go`
- `batch_types.go`
- CLI parsing / reporting in `cmd/bip32-pq-zkp-host/batch.go`

This gives the parent batch a stable schema tag for child batch claims.

### 2. Support 84-Byte Leaves

Current limitation:

- leaf journals are assumed to be 72 bytes
- batch claims are 84 bytes

Required changes:

- `batchLeafJournalSize(...)` in `batch_support.go`
- guest-side `supportedLeafClaimSize` assumption in `guest_batch/main.go`
- batch witness-building comments and wire-format docs

The guest can stay simple if it still assumes *one fixed size per batch*,
selected by `leaf_claim_kind`.

### 3. Pin Child Batch Guest Image IDs

The current claim already pins one `leaf_guest_image_id` for the batch.

For parent batches, that field naturally becomes:

- the common guest image ID used by all child batch receipts

So no new claim field is required for the first nested version.

### 4. Chained Inclusion Proofs

Current verifier mode:

- final batch receipt
- one inclusion proof into the final root

Nested verifier mode:

- final parent receipt
- one inclusion proof into the parent root
- one disclosed child batch claim
- one inclusion proof into the child batch root
- one disclosed original leaf claim

So the verifier artifact becomes a chain of level-local proofs.

The simplest first implementation is to keep one JSON file per level rather
than inventing a new bundled proof format immediately.

## Concrete Example

Assume 15 original leaves and chunk size 5.

### Child Level

- child batch `A`: leaves `0..4`
- child batch `B`: leaves `5..9`
- child batch `C`: leaves `10..14`

Artifacts:

- `A.receipt`, `A.claim.json`
- `B.receipt`, `B.claim.json`
- `C.receipt`, `C.claim.json`

Each child claim is 84 bytes when serialized as a journal.

### Parent Level

The parent guest takes:

- child batch receipts as assumptions
- child batch journals as private stdin witness bytes

It then:

1. verifies each child batch receipt with `zkvm.Verify(...)`
2. computes a Merkle root over the ordered child batch journals
3. commits one new parent batch claim

The final parent receipt is the standalone distributed proof.

### Sparse Verification

To prove original leaf `7` is included:

1. verify the parent receipt
2. show that child batch `B` is in the parent root
3. show that leaf `7` is in child batch `B`'s root

That is one inclusion proof at the parent level and one at the child level.

## Host Interface Recommendation

Keep the current host API shape and add one more batch leaf kind.

That implies:

- no new top-level prove command is required
- parent batches can still use:
  - `execute-batch`
  - `prove-batch`
  - `verify-batch`
  - `derive-batch-inclusion`

The only new CLI surface needed for the first cut is support for:

- `batch-claim-v1` or similar as a `--leaf-kind`

## Suggested Phase Plan

### Phase 1: Structural Support

1. Add `LeafKindBatchClaimV1 = 3`
2. Support 84-byte leaves
3. Update guest and host validation paths
4. Update docs and reports

Success criterion:

- parent batch over existing child batch claims executes and proves

### Phase 2: Sparse Verification Chain

1. Add host helpers for verifying one parent-level inclusion proof
2. Reuse existing inclusion verification for the child level
3. Add a wrapper report or CLI docs for the two-step verification flow

Success criterion:

- verify one original disclosed leaf through parent + child inclusion proofs

### Phase 3: Measure Hierarchy

1. compare flat batch vs nested batch at the same total leaf count
2. compare:
   - composite receipt size
   - succinct receipt size
   - prove time
   - peak memory
   - verifier artifact size

Success criterion:

- know whether hierarchy is worth carrying into the main public story

## Expected Tradeoffs

### Pros

- incremental batch construction
- reuse of prior chunk proofs
- smaller per-step prove jobs
- natural parallelism
- straightforward extension of current code

### Cons

- parent root no longer commits directly to original leaves
- sparse verification needs one proof per level
- more moving parts in the verifier story

## Recommendation

If we want the first hierarchical version soon, implement nested batch claims
first.

It is the shortest path that:

- reuses the current guest/host split
- preserves the fixed-size batch claim shape
- preserves the current Merkle tree
- avoids a premature accumulator redesign

If later we need one canonical flat root over all original leaves regardless
of chunking, that should be treated as a separate protocol layer.
