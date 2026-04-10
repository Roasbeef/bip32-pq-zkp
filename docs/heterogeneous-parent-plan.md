# Heterogeneous Parent Leaves Plan

## Goal

Extend the current homogeneous nested-batch design so one parent batch can
mix direct leaf kinds such as:

- `batch_claim_v1`
- one raw hardened-xpriv leaf
- one raw Taproot leaf

The intended shape is:

```text
parent batch = {batch_claim_v1, raw_leaf_1, raw_leaf_2, ...}
```

without forcing every direct child at that level to share one
`leaf_claim_kind` and one `leaf_guest_image_id`.

## Current Constraint

The current batch guest is still fundamentally:

- one `leaf_claim_kind` per batch
- one fixed journal size per batch
- one pinned `leaf_guest_image_id` per batch

That is visible in both the guest stdin format and the batch claim schema.
It is why the current implementation supports:

- homogeneous raw leaves
- homogeneous `batch_claim_v1` leaves

but not a mixed direct parent such as:

```text
{batch_claim_v1, raw_leaf_1, raw_leaf_2}
```

## Main Design Question

What should the parent root commit to when direct children are mixed?

Two broad options exist:

1. commit directly to the raw child journals
2. commit to a fixed-size per-leaf envelope that carries the metadata needed
   to interpret the child

The second option is the practical one.

## Recommended Direction: Fixed-Size Heterogeneous Leaf Envelope

The parent batch should stop hashing bare child journals directly and instead
hash a fixed-size envelope for each direct child.

Each envelope would carry:

- the direct child kind
- the guest image ID to verify that child receipt against
- the actual journal length
- the actual journal bytes, padded to a fixed maximum

That gives the parent guest enough information to:

1. recover the real child journal from the padded envelope
2. call `zkvm.Verify(child_image_id, child_journal)` per child
3. hash one fixed-size direct-child record into the parent Merkle tree

## Why Full Journal Bytes Inside The Envelope

The most important design choice is whether the envelope should contain:

- the full child journal bytes
- or only a hash of the child journal

The recommended v1 choice is: include the full child journal bytes.

Why:

- current direct child journals are still small:
  - raw leaf claims are 72 bytes
  - `batch_claim_v1` is 84 bytes
- the maximum direct-child journal size across the current designs is only 84
  bytes
- storing the full journal keeps sparse verification simple
- verifiers can decode a disclosed raw child directly from the inclusion proof
  without needing an extra side artifact just to recover the journal

The alternative hash-only envelope is possible, but it would force the
verifier to carry:

- the inclusion proof
- the raw child journal out of band
- the child image ID out of band

That is less ergonomic and makes the claim semantics harder to read.

## Recommended Envelope Shape

The exact byte layout can still move a little, but the right structure is:

```text
[envelope_version:u32]
[direct_leaf_kind:u32]
[child_verify_image_id:32]
[child_journal_len:u32]
[child_journal_padded:MAX_DIRECT_CHILD_JOURNAL]
```

where:

- `direct_leaf_kind` names the semantic child type:
  - raw Taproot
  - raw hardened-xpriv
  - `batch_claim_v1`
- `child_verify_image_id` is the image ID the parent guest must use for
  `zkvm.Verify`
- `child_journal_len` recovers the real child journal from the padded bytes
- `MAX_DIRECT_CHILD_JOURNAL` is initially `84`

This is intentionally a parent-level envelope. It does not replace the
underlying child claim schemas.

## Why A New Batch Claim Version Is Needed

The current batch claim v1 has one field whose meaning is:

- one common `leaf_guest_image_id` for all direct leaves

That cannot represent a heterogeneous parent, because each direct child may
come from a different guest image.

So heterogeneous parents need a new batch claim version. The simplest shape is:

```text
[batch_version:u32]
[batch_flags:u32]
[direct_leaf_mode:u32]
[merkle_hash_kind:u32]
[leaf_count:u32]
[policy_digest:32]
[merkle_root:32]
```

The existing 84-byte size can stay the same if the old `leaf_guest_image_id`
field is reinterpreted as a `policy_digest` under the new batch version.

That `policy_digest` should commit to the parent-level policy, for example:

- envelope version
- maximum direct-child journal size
- allowed direct child kinds
- whether `batch_claim_v1` leaves are allowed

The exact policy contents can stay minimal in v2. The important point is that
the parent claim must no longer pretend there is one shared child guest image.

## Verifier Story

With the envelope design, sparse verification becomes:

1. verify the final parent receipt
2. verify one inclusion proof for one disclosed direct-child envelope
3. decode the direct child from that envelope
4. branch by child kind

For a raw leaf child:

- the disclosed envelope already contains the full raw child journal
- the verifier can decode and check that raw leaf immediately

For a `batch_claim_v1` child:

- the disclosed envelope contains the child batch claim journal
- the verifier can stop there, or continue with a lower-level inclusion proof

So heterogeneous parents do not eliminate chained verification. They just let
one parent level directly mix:

- raw leaves
- child batch claims

## What Changes In The Guest

The current guest reads:

- one batch-global leaf kind
- one batch-global leaf image ID
- one batch-global fixed leaf size

The heterogeneous guest would instead read:

- one batch-global direct-child mode, such as `heterogeneous_envelope_v1`
- one batch-global Merkle hash kind
- one batch-global leaf count
- then one fixed-size heterogeneous envelope per child

The guest loop would become:

1. read one envelope
2. decode the child kind, image ID, and real journal bytes
3. call `zkvm.Verify(child_image_id, child_journal)`
4. hash the fixed-size envelope into the Merkle tree

That is the minimum structural change needed for mixed direct children.

## What Changes On The Host

The host loader would need to:

- accept mixed child claim files in one parent batch
- map each child claim into one heterogeneous envelope
- choose the correct child verify image ID per child
- write the envelope bytes into the guest stdin witness
- derive inclusion proofs over envelopes instead of bare raw journals

The sparse-inclusion proof schema will also need a new version so the verifier
can decode the disclosed envelope and know what direct child type it contains.

## Recommended Implementation Order

### Phase 1: Schema Work

1. define `heterogeneous_leaf_envelope_v1`
2. define `batch_claim_v2`
3. define the policy-digest contents
4. document the new verifier contract

Success criterion:

- the schema is fixed before changing the guest or CLI

### Phase 2: Guest Support

1. add a heterogeneous parent mode to the batch guest
2. parse fixed-size envelopes instead of one global leaf kind
3. verify each child against its per-envelope image ID
4. hash the envelopes into the Merkle root

Success criterion:

- the guest can execute and prove a parent over mixed direct children

### Phase 3: Host + CLI Support

1. add host-side envelope construction
2. add CLI support for mixed direct leaves
3. add new inclusion-proof schema for envelope-based sparse verification
4. add verifier support for mixed direct children

Success criterion:

- a parent batch can mix:
  - one `batch_claim_v1`
  - one raw hardened-xpriv leaf
  - one raw Taproot leaf

### Phase 4: Measurement

1. compare homogeneous nested vs heterogeneous parent layouts
2. compare verifier artifact size and disclosure complexity
3. compare proving time and memory

Success criterion:

- decide whether heterogeneous parents are worth keeping as a first-class
  path

## Recommended Scope Cut

The recommended first heterogeneous implementation is narrow:

- only mixed direct children at one parent level
- only the current three child kinds:
  - raw Taproot
  - raw hardened-xpriv
  - `batch_claim_v1`
- fixed `MAX_DIRECT_CHILD_JOURNAL = 84`
- SHA-256 only

That keeps the change set bounded and lets the verifier contract stay readable.

## Non-Goal

This plan does not solve the flat-root-preserving append-only problem.

If the long-term goal becomes:

- one canonical root over all original leaves across arbitrary chunking

then the MMR / frontier / bag-of-peaks direction is still the cleaner fit.

Heterogeneous parents solve:

- mixed direct child semantics inside the current hierarchical tree

They do not replace the separate accumulator problem.
