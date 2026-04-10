# Batch Aggregation

## Overview

The batch aggregation lane uses risc0's recursive composition to verify N
leaf receipts inside one aggregation guest and commit a single Merkle root.
The result is one final receipt that proves: "there exist N valid leaf
receipts, all from the same guest image, whose ordered journals hash to
this root."

The target verifier experience is:

1. produce N leaf proofs for N keys or UTXOs
2. aggregate them into one final batch proof
3. publish one final receipt plus one small batch `claim.json`
4. let verifiers check either:
   - the whole batch at once, or
   - one disclosed leaf with a normal Merkle inclusion proof

The important distinction is:

- the zk receipt proves that the committed batch root was derived correctly
- the Merkle branch proves that one disclosed leaf is included in that root
- heterogeneous parents can now mix raw leaves and child batch claims at one
  level, but they still disclose inclusion through the same Merkle-branch
  mechanism

## How It Works

### Step 1: Leaf Proofs

Each leaf proof is generated using one of the single-key lanes (full
Taproot or hardened-xpriv today) and stored as a `receipt + claim.json`
pair. The first nested layer also reuses this same batch guest with
`batch_claim_v1` leaves, where each leaf is itself one serialized child
batch claim. Leaf receipts must be compressed to succinct form because the
risc0 host API requires succinct receipts for assumption resolution.

### Step 2: Batch Guest Execution

The batch host loads the leaf artifacts, verifies each leaf receipt
host-side, and passes them as assumptions to the batch guest. The guest:

1. reads the leaf journals from stdin (private witness)
2. for each leaf, calls `zkvm.Verify(image_id, journal)` to register
   an assumption against the corresponding succinct leaf receipt
3. hashes the ordered leaf journals into a SHA-256 Merkle root using
   domain-separated hashing (0x00 prefix for leaves, 0x01 for interior
   nodes, tagged leaf inputs)
4. commits a fixed-size 84-byte batch claim to the journal

The batch guest never sees the leaf private witnesses. It only sees the
leaf public journals and the host-provided assumption receipts.

### What `guest_batch` And The Runner Each Do

The composition split in the batch lane is:

- runner / host side:
  - load child `receipt + claim.json` artifacts
  - decode and pass the child receipts as `Assumptions`
  - build stdin carrying the ordered public leaf records
- batch guest side:
  - read those public leaf records from stdin
  - call `zkvm.Verify(...)` once per child
  - hash the ordered leaf records into a Merkle root
  - commit one fixed-size batch claim

This means the guest never directly verifies the child receipts itself. It
only registers the exact child claims it depends on.

For homogeneous batches the guest registers:

- `zkvm.Verify(shared_leaf_image_id, leaf_journal)`

For heterogeneous parents the guest registers:

- `zkvm.Verify(envelope.verify_image_id, child_journal)`

Under the hood this creates a conditional top-level receipt whose
`assumptions_digest` commits to all of those child claim digests. The host
supplies the actual succinct child receipts, and risc0's recursion pipeline
resolves them during proving.

So there are two sides to the same dependency set:

- guest:
  - digest-only assumptions list
- runner / host:
  - concrete succinct child receipts

If they disagree, proof generation fails. If they agree, the final batch
receipt is fully resolved and external verifiers do not need the child
receipts anymore.

### Step 3: Final Receipt

The host produces one final batch receipt. This can be composite or
succinct. The succinct form stays at ~223 KB regardless of N.

## Batch Claim Schema (84 bytes)

```text
[0:4]   batch_version      (uint32 LE)
[4:8]   batch_flags        (uint32 LE, currently 0)
[8:12]  leaf_claim_kind    (uint32 LE: 1=taproot, 2=hardened_xpriv,
                            3=batch_claim_v1, 4=heterogeneous_envelope_v1)
[12:16] merkle_hash_kind   (uint32 LE: 1=sha256)
[16:20] leaf_count         (uint32 LE)
[20:52] context_digest      (32 bytes)
[52:84] merkle_root         (32 bytes, SHA-256 Merkle root)
```

The claim is fixed-size regardless of N. The fan-out is captured by the Merkle
root, not by enumerating leaves in the journal.

How the 32-byte context slot is interpreted:

- homogeneous batch (`batch_version = 1`)
  - one shared direct-leaf image ID
- heterogeneous parent (`batch_version = 2`)
  - one pinned policy digest describing the direct-child envelope mode

## Heterogeneous Direct-Child Envelopes (128 bytes)

The mixed direct-parent mode is now implemented with a fixed-size envelope:

```text
[0:4]   envelope_version    (uint32 LE, currently 1)
[4:8]   direct_leaf_kind    (uint32 LE)
[8:40]  verify_image_id     (32 bytes)
[40:44] journal_len         (uint32 LE)
[44:128] padded_journal     (84-byte fixed slot)
```

The current allowed direct child kinds are:

- `taproot`
- `hardened_xpriv`
- `batch_claim_v1`

This means a parent can now directly aggregate a set such as:

- one raw hardened-xpriv claim
- one raw Taproot claim
- one child batch claim

while still keeping the final public claim fixed at 84 bytes.

## Merkle Tree Construction

The tree uses domain-separated binary hashing:

- `leaf_node = SHA256(0x00 || "bip32-pq-zkp:batch-leaf:v1" || index_le32 || leaf_journal)`
- `inner_node = SHA256(0x01 || left || right)`

The 0x00/0x01 prefix prevents second-preimage attacks. The tag and index
binding prevent leaf reordering or cross-batch confusion. Odd-length levels
duplicate the last node (Bitcoin-style).

## Verifier Flow

### Batch-Only Verification

1. load the final batch receipt
2. verify it against the batch guest image ID
3. decode the batch claim from the journal
4. the verifier now knows:
   - homogeneous mode:
     - `N` valid leaf receipts exist, all from the pinned leaf guest image,
       with journals hashing to the committed root
   - heterogeneous mode:
     - `N` valid direct-child receipts exist, all satisfying the pinned
       direct-child envelope policy digest, with envelopes hashing to the
       committed root

### Sparse Verification (One Disclosed Leaf)

1. verify the final batch receipt (as above)
2. receive a Merkle inclusion proof for one leaf
3. recompute the leaf hash from the disclosed journal and index
4. walk the sibling path to recompute the root
5. compare against the `merkle_root` in the batch claim
6. apply application-specific checks to the disclosed leaf

No leaf receipt needs to be distributed. The batch receipt already proves
that the root came from valid leaf receipts.

### Sparse Verification (Heterogeneous Parent)

For a heterogeneous parent, the disclosed Merkle leaf is the 128-byte
direct-child envelope. The verifier:

1. verifies the final heterogeneous receipt
2. checks inclusion of the disclosed envelope
3. inspects the envelope:
   - direct child kind
   - per-child verify image ID
   - embedded journal bytes
4. if the direct child is raw, stops there
5. if the direct child is `batch_claim_v1`, continues with the next
   inclusion-proof level

### Sparse Verification (Nested Batch Chain)

The first hierarchical layer is now implemented without changing the batch
guest binary:

1. child batches are built with `prove-batch`
2. parent batches set `--leaf-kind batch-claim-v1`
3. parent leaves are child batch `claim.json` journals plus their succinct
   receipts
4. `bundle-batch-inclusion-chain` combines one proof per level into one
   verifier artifact
5. `verify-batch` or `verify-nested-batch` can verify that bundled chain

The verifier flow becomes:

1. verify the final parent receipt
2. verify inclusion of one child batch claim in the parent root
3. decode that disclosed child batch claim from the parent inclusion proof
4. verify inclusion of one original leaf in the child batch root

So the final verifier-facing artifact is still one receipt plus small JSON
artifacts, but sparse verification now needs one Merkle branch per level.

### One-Shot Nested Wrapper

The repo now also exposes a manifest-driven wrapper through
`run-nested-batch-plan`. The wrapper is thin orchestration over the same
`Runner` methods used by the lower-level commands:

1. prove inline child batches bottom-up
2. prove the final top-level batch
3. optionally derive a bundled inclusion chain from one `disclosure_path`
4. optionally verify the final receipt plus that bundled chain

That means the current batch lane has both:

- low-level composable commands for debugging
- one ergonomic one-shot wrapper for repeatable nested demos

## Artifact Sizes And What They Mean

The batch lane produces three different artifact classes, and they scale
differently:

### 1. Final Receipt

This is the actual zk proof artifact distributed to verifiers.

- composite mode:
  - grows with the amount of aggregation work
- succinct mode:
  - stays near the ~223 KB floor in the current design

The receipt proves the batch guest execution itself. It is the only artifact
that is cryptographically required for full receipt verification.

### 2. `claim.json`

This is the human-readable decoded view of the fixed-size public batch
journal.

- the true on-proof public claim is only 84 bytes
- `claim.json` is usually about 755-760 bytes because it adds:
  - JSON field names
  - lowercase hex encodings
  - convenience metadata like `proof_seal_bytes`

For a heterogeneous parent, the JSON carries fields like:

- `batch_version`
- `leaf_claim_kind_name`
- `leaf_count`
- `policy_digest`
- `merkle_root`
- `journal_hex`
- `receipt_encoding`

This file does not enumerate all children. It stays essentially flat as the
batch grows because the fan-out is captured by `merkle_root`, not by listing
every child journal in the public claim.

### 3. Inclusion Proof Artifacts

These are the sparse-disclosure artifacts used when a verifier wants to check
one disclosed leaf rather than only accepting the batch root at face value.

- flat homogeneous batch:
  - one Merkle inclusion proof
- nested homogeneous batch:
  - one inclusion proof per level
- heterogeneous parent:
  - one parent-level envelope inclusion proof, plus additional lower-level
    proofs if the disclosed child is itself a nested batch

This is the part that grows with:

- `O(log N)` branch length within one batch
- disclosure depth across nested levels

So the practical size model is:

- final succinct receipt: near-constant
- `claim.json`: near-constant
- inclusion proof JSON: grows with disclosed depth and branch length

That is why a statement like `B = {A, 4}` still has a small final
`B.claim.json`: it records only the top-level batch metadata and root. To
show something under `A`, the verifier additionally needs the inclusion of
`A` into `B`, then the inclusion of the target leaf into `A`.

## Scaling Results

Hardened-xpriv batch scaling (Apple Silicon, Metal-backed local prover):

| N | Kind | Receipt bytes | Seal bytes | Claim JSON | Inclusion JSON | Prove sec | Prove RSS | Verify sec |
|---|------|---------------|------------|------------|----------------|-----------|-----------|------------|
| 2 | composite | 681,214 | 679,904 | 755 | 456 | 2.06 | 3.16 GB | 0.04 |
| 2 | succinct  | 223,343 | 222,668 | 755 | 456 | 5.35 | 3.17 GB | 0.02 |
| 4 | composite | 1,138,062 | 1,135,864 | 756 | 528 | 3.66 | 6.03 GB | 0.06 |
| 4 | succinct  | 223,343 | 222,668 | 755 | 528 | 9.44 | 6.02 GB | 0.02 |
| 8 | composite | 2,042,158 | 2,038,184 | 756 | 600 | 7.27 | 11.21 GB | 0.12 |
| 8 | succinct  | 223,343 | 222,668 | 755 | 600 | 17.74 | 11.20 GB | 0.04 |
| 16 | composite | 4,072,409 | 4,064,720 | 757 | 673 | 11.24 | 11.25 GB | 0.22 |
| 16 | succinct  | 223,343 | 222,668 | 756 | 673 | 33.80 | 11.26 GB | 0.04 |

Full Taproot leaf confirmation:

| N | Kind | Receipt bytes | Prove sec |
|---|------|---------------|-----------|
| 2 | composite | 681,214 | 2.64 |
| 2 | succinct  | 223,343 | 6.43 |
| 8 | composite | 2,042,158 | 10.35 |
| 8 | succinct  | 223,343 | 21.22 |

Key observations:

- succinct batch receipt is flat at ~223 KB across N=2..16
- composite grows linearly (~340 KB per additional leaf)
- prove time scales roughly linearly (~0.7s/leaf composite, ~2s/leaf succinct)
- Taproot and xpriv batch costs are nearly identical, confirming the
  aggregation overhead dominates over leaf semantics

## What The Batch Receipt Buys

For sparse verification, the comparison is:

| N | N separate succinct leaf receipts | Succinct batch + claim + inclusion |
|---|-----------------------------------|------------------------------------|
| 2 | 446,638 B | 224,554 B |
| 4 | 893,276 B | 224,626 B |
| 8 | 1,786,552 B | 224,698 B |
| 16 | 3,573,104 B | 224,772 B |

The batch approach gives nearly flat verifier-facing artifact size while
N separate receipts grow linearly.

## Flat Vs Nested (Hardened XPriv)

The first homogeneous nested layer now has direct measurements at the same
total original-leaf count:

| N | Final kind | Flat prove | Flat peak RSS | Nested total prove | Nested peak RSS | Flat verifier artifact | Nested verifier artifact |
|---|------------|------------|---------------|--------------------|-----------------|------------------------|--------------------------|
| 8 | composite | 7.27s | 11.21 GiB | 24.79s | 5.75 GiB | 2,043,514 B | 1,139,980 B |
| 8 | succinct | 17.74s | 11.20 GiB | 30.69s | 5.74 GiB | 224,698 B | 225,260 B |
| 16 | composite | 11.24s | 11.25 GiB | 45.25s | 5.75 GiB | 4,073,839 B | 1,140,056 B |
| 16 | succinct | 33.80s | 11.26 GiB | 51.82s | 5.75 GiB | 224,772 B | 225,337 B |

Here, `flat verifier artifact` means:

- final receipt
- `claim.json`
- one flat inclusion proof

And `nested verifier artifact` means:

- final top-level receipt
- top-level `claim.json`
- one bundled inclusion-chain JSON artifact

Main takeaway:

- nested batches materially reduce peak RSS and the composite top-level
  receipt size
- nested batches increase total prove time because the child batches must be
  proven first
- once the final parent receipt is `succinct`, the final distributed proof
  still stays on the same ~223 KB scale

## What Grows And What Does Not

Roughly constant with N:

- final succinct receipt size
- final succinct receipt verification cost
- batch `claim.json` size

Grows with N:

- total leaf proving work
- total recursive aggregation work
- proving latency

Grows with number of disclosed leaves:

- Merkle branch count and size (O(log N) per leaf)
- verifier-side branch checking work

## Design Decisions (v1)

1. SHA-256 for the Merkle tree (Bitcoin-adjacent, easy for external verifiers)
2. One pinned direct leaf kind plus one pinned `leaf_guest_image_id` per
   batch; parent `batch_claim_v1` leaves are decoded and must agree on child
   batch version, child flags, child leaf kind, child Merkle hash kind, and
   child leaf guest image ID
3. Fixed-size 84-byte batch claim (root-based, not leaf-enumerating)
4. Both batch-only and sparse-inclusion verification modes
5. Both hardened-xpriv and full Taproot leaf schemas validated
6. First nested `batch_claim_v1` parent layer implemented with a bundled
   inclusion-chain verifier artifact, while repeated `--inclusion-in` files
   remain available as the low-level interface
7. Heterogeneous parent `batch_version = 2` implemented with fixed-size
   direct-child envelopes and a pinned policy digest
8. Manifest-driven `run-nested-batch-plan` wrapper implemented on top of the
   same `Runner` methods
