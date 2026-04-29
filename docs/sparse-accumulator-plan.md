# Sparse Accumulator Plan

## Why This Exists

The current batch system already supports:

- flat homogeneous batches
- nested homogeneous batches
- heterogeneous parents that mix raw leaves and child batch claims

But the nested model still has one important limitation:

- parent roots commit to child batch claims, not directly to all original
  leaves

That means a shape like:

- `A = {1, 2, 3}`
- later add `4`

cannot become one new flat root over `{1, 2, 3, 4}` unless we keep carrying
`A` as an intermediate leaf or redesign the accumulator.

This note sketches one alternative to MMR for that next phase:

- a compressed sparse Merkle accumulator

## Why Sparse Merkle Instead Of MMR

An MMR is naturally an append-only log accumulator:

- leaves are ordered by insertion position
- appends are natural
- the final root is a commitment to an ordered list

That is attractive if the only goal is:

- append more leaves to one log

But a sparse Merkle tree is a better fit if the goal is:

- one flat root over original leaves
- sparse inclusion and non-inclusion proofs
- future heterogeneous leaf types
- keyed membership rather than only positional append semantics

So the tradeoff is:

- MMR:
  - simpler append-only semantics
  - weaker keyed-set semantics
- sparse tree:
  - stronger keyed-set semantics
  - more explicit update/non-inclusion rules
  - bigger redesign

## High-Level Model

Instead of batching child claims into parent leaves, each proof becomes a
state transition on one sparse root:

```text
old_root + update_witnesses + new_leaves -> new_root
```

The final claim would then say:

- starting from `old_root`
- after inserting or updating this set of keyed leaves
- the resulting accumulator root is `new_root`

That is closer to a persistent authenticated map than a tree of batch claims.

## What This Buys Us

If the accumulator commits directly to original leaves, then:

- the root remains flat across incremental updates
- adding leaf `4` after `{1,2,3}` can produce a new root for `{1,2,3,4}`
  without keeping `{1,2,3}` as a child batch leaf
- sparse verifiers can check one keyed leaf with one sparse inclusion proof
- non-inclusion becomes available too, if we need it later

That makes it a stronger long-term fit than the current nested model if we
care about “one evolving root over all claims so far”.

## Likely Leaf Shape

The natural direct leaf is still some fixed-size envelope-like claim record.

For example:

- `leaf_kind`
- `verify_image_id`
- `journal_len`
- `journal_bytes` or a digest thereof
- optional application flags

In other words, the sparse tree would probably reuse the same idea as the
current heterogeneous direct-child envelope, but keyed in a flat sparse
accumulator instead of hashed as one level of a batch tree.

## Keying Options

The first major design choice is the sparse-tree key.

### Option 1: Positional Keys

```text
key = uint256(index)
```

Pros:

- simple append-style mental model
- deterministic ordering
- close to the current positional Merkle tree

Cons:

- sparse tree is then acting mostly like a positional log
- does not naturally deduplicate or identify claims by meaning

### Option 2: Application Identity Keys

```text
key = H(app_identifier)
```

Examples:

- `H(utxo_outpoint)`
- `H(address || branch || index)`
- `H(batch_namespace || local_index)`

Pros:

- natural sparse membership semantics
- better fit for “prove this particular thing is present”

Cons:

- requires a stable external identity
- duplicates and overwrite rules need to be explicit

### Option 3: Claim-Derived Keys

```text
key = H(leaf_kind || verify_image_id || journal)
```

Pros:

- self-contained
- no external naming scheme required

Cons:

- behaves more like a set than an append-only log
- hard to reason about updates/replacements

## Append-Only Semantics

This is the biggest difference from MMR.

A sparse tree is a keyed map, not an append-only log by default. So if we
want “append-only” behavior, we must prove an insertion policy.

The cleanest v1 rule would be:

- a new leaf may be inserted only if that key was previously empty

That means each update proof must include:

- a non-inclusion proof for every newly inserted key against the old root
- recomputation of the new root after the insertions

That gives us append-only-by-policy rather than append-only-by-structure.

## Claim Shape Sketch

The current batch claim is 84 bytes and commits only to a single root plus a
small amount of batch metadata.

A sparse-accumulator state transition claim could look like:

```text
[version:u32]
[flags:u32]
[accumulator_kind:u32]
[leaf_policy_kind:u32]
[update_count:u32]
[old_root:32]
[new_root:32]
```

That is still compact and stays flat with respect to the number of inserted
leaves.

Possible variants:

- include only `new_root` if the old root is implicit
- include a `policy_digest` instead of `leaf_policy_kind`
- include `leaf_count` if the application wants size accounting

## Proof Artifact Shape

The final verifier-facing artifacts would still naturally split into:

1. final receipt
2. decoded `claim.json`
3. sparse proof artifacts

The likely difference from the current Merkle branch model is that the sparse
proof artifact becomes:

- inclusion proof
  or
- non-inclusion proof

against the sparse accumulator root.

If we adopt a compressed sparse proof style similar to Taproot Assets, those
proof artifacts can stay much smaller than naively shipping a full 256-level
sibling list.

## Why Taproot Assets Is Relevant

Taproot Assets uses a Merkle-Sum Sparse Merkle Tree (MS-SMT). The sum feature
is probably unnecessary for our immediate use case, but the broader design is
still instructive:

- fixed keyspace
- cached empty nodes
- efficient inclusion and non-inclusion proofs
- compressed proof representation for empty branches

For our use case, the most relevant takeaways are:

- compressed sparse proofs
- persistent flat root semantics
- efficient sparse verification

We likely would not reuse their Go implementation directly inside the TinyGo
guest. It is better thought of as a design reference rather than drop-in code.

## What Would Need To Change

Compared with the current batch design, this is a larger redesign.

### Guest Changes

The guest would need to:

1. read an old root and one or more update witnesses
2. verify inclusion or non-inclusion proofs for the affected keys
3. apply the leaf inserts or updates
4. recompute the new sparse root
5. commit a state-transition claim

### Host Changes

The host would need to:

1. maintain a sparse accumulator store or deterministic builder
2. derive compressed sparse witnesses for inserts and lookups
3. feed those witnesses into the guest
4. serialize verifier-facing sparse proof artifacts

### Verifier Changes

A sparse verifier would check:

1. the final receipt
2. the decoded state-transition claim
3. one sparse inclusion or non-inclusion proof against the committed root

So verifier UX remains simple, but the proof artifact type changes.

## Effect On Final Succinct Receipt Size

This probably does **not** dramatically reduce the final succinct receipt size
relative to the current batch lane.

The main expected effect is structural, not miraculous compression:

- better accumulator semantics
- flat root over original leaves
- better sparse verification model

The final succinct receipt would likely remain on the same order of magnitude
as the current ~223 KB scale, assuming the public journal remains compact.

The place where sparse accumulation matters more is:

- incremental semantics
- proof-chain shape
- sparse inclusion / non-inclusion UX

not “make the final succinct proof an order of magnitude smaller”.

## Comparison With The Current Nested Batch Design

### Current Nested Batch

Pros:

- already implemented
- simple reuse of the current batch guest
- easy to reason about level by level

Cons:

- parent root commits to child batch claims
- not one flat evolving root over original leaves
- sparse verification path deepens with hierarchy

### Sparse Accumulator

Pros:

- one flat evolving root over original leaves
- inclusion and non-inclusion proofs
- better fit for future heterogeneous keyed claims

Cons:

- bigger redesign
- needs explicit append-only/update policy
- likely more guest logic than the current batch guest

## Recommended Incremental Plan

If we take this direction, the smallest sensible sequence is:

1. Freeze a v1 sparse-leaf envelope and keying rule.
2. Pick the append-only insertion policy.
3. Prototype host-side sparse witness generation outside the guest.
4. Implement a guest that verifies one-key insertions first.
5. Measure:
   - prove time
   - final receipt size
   - sparse inclusion/non-inclusion artifact sizes
6. Only then decide whether this replaces or complements the current nested
   batch design.

## Recommendation

If the next problem is:

- “how do we maintain one evolving root over original claims without carrying
  intermediate child batches as leaves?”

then a compressed sparse Merkle accumulator looks like a stronger fit than an
MMR.

If the next problem is simply:

- “how do we append to an ordered log with the smallest conceptual change?”

then MMR is still the smaller step.

So the tradeoff is:

- MMR first:
  - smaller conceptual delta
- sparse accumulator first:
  - stronger long-term semantics for keyed heterogeneous claims
