# Batch Recursion Walkthrough

## Why This Exists

`bip32-pq-zkp` uses the generic `go-zkvm` composition machinery, but applies it
to a specific batch-aggregation design:

- leaf proofs are produced first
- the batch guest depends on those leaf proofs
- the final batch proof commits only a Merkle root and a fixed-size batch claim

This note explains that concrete flow at the batch lane level.

## The Core Batch Statement

For a homogeneous batch, the final batch receipt proves:

- there exist `N` valid leaf receipts
- they all use the pinned direct-leaf guest image
- their ordered journals hash to the committed `merkle_root`

For a heterogeneous parent, the final batch receipt proves:

- there exist `N` valid direct-child receipts
- each direct child satisfies the pinned heterogeneous envelope policy
- the ordered direct-child envelopes hash to the committed `merkle_root`

In both cases, the final public batch claim is fixed-size:

- 84 bytes in the journal
- a small decoded `claim.json` beside the receipt

## What The Runner Does

The batch runner lives in:

- `batch_runner.go`

Its job is to prepare the two different inputs the proving stack needs:

1. concrete child receipts as assumptions
2. ordered public child records as guest stdin

That split is important.

The runner loads each child:

- `receipt`
- `claim.json`

Then it uses the child `claim.json` to recover the public child journal bytes
and the child verification image ID. It passes:

- the actual child receipt bytes into `Assumptions`
- the ordered child public records into the batch guest stdin

So the runner is responsible for turning repo artifacts into:

- prover-side recursive assumptions
- guest-visible public child records

## What `guest_batch` Does

The batch guest lives in:

- `guest_batch/main.go`

It reads from stdin:

- `leaf_claim_kind`
- `merkle_hash_kind`
- `leaf_context_digest`
- `leaf_count`
- `leaf_record_0 .. leaf_record_N-1`

Then for each child record it:

1. validates the record shape for the batch mode
2. calls `zkvm.Verify(...)` to register one assumption
3. keeps the record bytes for Merkle hashing

After that it:

4. computes the Merkle root over the ordered child records
5. commits one fixed-size batch claim to the journal

The guest never receives the child private witnesses. It only sees:

- child public journals or heterogeneous envelopes
- the batch configuration

## What `zkvm.Verify(...)` Means In The Batch Lane

Inside the batch guest, `zkvm.Verify(...)` does not verify a child proof
directly.

Instead it says:

- “this batch proof depends on there existing a valid child receipt for this
  exact `(imageID, journal)` claim”

The host separately supplies the actual succinct child receipts as assumptions.
During proving, recursion resolves those child receipts against the assumptions
the guest registered.

So the batch lane always has two representations of the same children:

- guest side:
  - digest-only assumptions
- host side:
  - concrete succinct child receipts

## Homogeneous Batches

In a homogeneous batch, the guest uses one shared direct-leaf image ID for all
children:

```text
zkvm.Verify(shared_leaf_image_id, leaf_journal)
```

That is why the batch claim can pin one common `leaf_guest_image_id` in
`batch_version = 1`.

Examples:

- all hardened-xpriv leaves
- all Taproot leaves
- all child batch claims

## Heterogeneous Parents

In a heterogeneous parent, each direct child is wrapped in a fixed-size
envelope:

- direct child kind
- per-child verify image ID
- journal length
- padded journal bytes

So the guest uses:

```text
zkvm.Verify(envelope.verify_image_id, child_journal)
```

Here the batch claim can no longer pin one shared child image ID, so the same
32-byte slot is reinterpreted as a `policy_digest` under `batch_version = 2`.

That pinned policy digest tells verifiers which envelope rules were enforced by
the guest.

This is what enables a parent shape like:

- `A = {1, 2, 3}`
- `B = {A, 4}`

where `A` is itself a batch claim and `4` is a raw leaf proof.

## What The Final Verifier Needs

The verifier-facing artifacts split into three classes:

1. final receipt
   - the actual zk proof
2. `claim.json`
   - decoded view of the fixed-size public batch claim
3. inclusion artifacts
   - used only when the verifier wants sparse disclosure

For batch-only verification, the verifier checks:

- final receipt
- expected batch guest image ID
- optional expected `claim.json`

For sparse verification, the verifier additionally checks:

- Merkle inclusion of the disclosed child in the committed root
- and, if the disclosed child is itself a batch claim, the next inclusion level

So in a shape like `B = {A, 4}`:

- to disclose `4`, the verifier checks one inclusion proof into `B`
- to disclose something inside `A`, the verifier checks:
  - inclusion of `A` into `B`
  - inclusion of the target leaf into `A`

## Why This Stays Small

The final batch claim never enumerates all children.

Instead, it commits only:

- batch version and flags
- direct child kind or envelope policy
- Merkle hash kind
- child count
- image ID or policy digest
- Merkle root

That is why:

- the final batch journal stays 84 bytes
- `claim.json` stays small
- the final succinct receipt stays on the same ~223 KB scale

What grows is not the top-level claim. What grows is:

- proving work
- prover memory
- sparse-disclosure Merkle branches

## How This Connects To Nested Batching

The first nested layer reuses the same batch guest:

- child batches are first proven on their own
- parent batches then aggregate child batch claims

That works because a child batch claim is itself just another fixed-size public
record.

The verifier then walks the tree level by level:

1. verify the top-level receipt
2. verify inclusion of the disclosed child batch claim
3. verify inclusion of the disclosed leaf inside that child batch

Repeat as needed for more levels.

## Where To Read Next

- `batch-aggregation.md`
  - batch schema, verifier modes, and measured scaling
- `nested-batching.md`
  - current nested design and supported hierarchy shapes
- `claim.md`
  - single-leaf claim semantics
