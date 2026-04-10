# MMR / Append-Only Accumulator Sketch

## Why This Exists

The current batch design is good for:

- one-level batching
- nested batch-of-batch claims
- sparse verification with one inclusion proof per level

It is **not** good for one specific requirement:

- preserve one canonical flat root over all original leaves while still adding
  leaves incrementally in separately-built chunks

That is where an append-only accumulator becomes interesting.

## What Problem It Solves

Suppose we want to:

1. prove 5 leaves today
2. prove 5 more leaves tomorrow
3. prove 5 more later
4. still end up with one root that means “the ordered set of all 15 original
   leaves”

The current binary Merkle tree does not compose that way across arbitrary
chunks. Once we compress a chunk into one subtree root, we lose enough shape
information that we cannot always recover the flat-tree root over the original
leaves.

An append-only accumulator such as an MMR avoids that by explicitly carrying
the forest / frontier state needed to keep extending the set.

## High-Level MMR Shape

An MMR stores the leaf set as a forest of perfect binary subtrees, often
described as “peaks”.

Appending one new leaf:

1. creates a new singleton peak
2. repeatedly merges equal-height peaks
3. updates the peak list

The public commitment is usually:

- the ordered list of peaks
- or one root derived from the peaks
- plus the leaf count

This makes append-only updates natural.

## What A Proof Would Look Like

For one disclosed original leaf, the verifier would likely need:

1. the final zk receipt
2. the final accumulator claim
3. the disclosed leaf claim
4. an inclusion proof inside one peak
5. enough peak / frontier data to recompute the final accumulator commitment

So compared with the current Merkle-tree batch:

- the final receipt story can still stay “one receipt”
- but the inclusion proof becomes an accumulator-specific proof, not an
  ordinary single Merkle branch

## What The Batch Claim Would Need

The current fixed-size batch claim is:

- version
- flags
- leaf claim kind
- Merkle hash kind
- leaf count
- leaf guest image ID
- one Merkle root

An append-only accumulator would need a different public claim, likely with:

- accumulator version
- leaf claim kind
- accumulator hash kind
- leaf count
- leaf guest image ID
- either:
  - a digest of the ordered peaks, or
  - enough peak digests to reconstruct that digest

That means this is not just “switch the tree function”. It is a new public
claim contract.

## Why We Are Not Starting Here

The current measured data already shows that:

- final succinct batch receipts stay flat at ~223 KB
- the current Merkle-root design already gives a strong sparse-verification
  story
- nested batch claims are a much smaller extension of what we already built

So MMR-style accumulation is only worth the extra complexity if the protocol
really needs:

- one canonical flat commitment over all original leaves
- append-only chunking without nested roots
- or accumulator semantics that survive long-lived incremental updates better
  than nested batch claims

## Recommended Position

Treat an MMR / frontier / bag-of-peaks accumulator as a v3 direction:

1. validate nested batch claims first
2. see whether chained inclusion proofs are actually a problem in practice
3. only redesign the accumulator if flat-root semantics become a real protocol
   requirement
