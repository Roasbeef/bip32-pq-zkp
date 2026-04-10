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

## How It Works

### Step 1: Leaf Proofs

Each leaf proof is generated using one of the single-key lanes (full
Taproot, hardened-xpub, or hardened-xpriv) and stored as a
`receipt + claim.json` pair. Leaf receipts must be compressed to succinct
form because the risc0 host API requires succinct receipts for assumption
resolution.

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

### Step 3: Final Receipt

The host produces one final batch receipt. This can be composite or
succinct. The succinct form stays at ~223 KB regardless of N.

## Batch Claim Schema (84 bytes)

```text
[0:4]   batch_version      (uint32 LE, currently 1)
[4:8]   batch_flags        (uint32 LE, currently 0)
[8:12]  leaf_claim_kind    (uint32 LE: 1=taproot, 2=hardened_xpriv)
[12:16] merkle_hash_kind   (uint32 LE: 1=sha256)
[16:20] leaf_count         (uint32 LE)
[20:52] leaf_guest_image_id (32 bytes, pinned once per batch)
[52:84] merkle_root         (32 bytes, SHA-256 Merkle root)
```

The claim is fixed-size regardless of N. The fan-out is captured by the
Merkle root, not by enumerating leaves in the journal.

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
4. the verifier now knows: N valid leaf receipts exist, all from the
   pinned leaf guest image, with journals hashing to the committed root

### Sparse Verification (One Disclosed Leaf)

1. verify the final batch receipt (as above)
2. receive a Merkle inclusion proof for one leaf
3. recompute the leaf hash from the disclosed journal and index
4. walk the sibling path to recompute the root
5. compare against the `merkle_root` in the batch claim
6. apply application-specific checks to the disclosed leaf

No leaf receipt needs to be distributed. The batch receipt already proves
that the root came from valid leaf receipts.

## Scaling Results

Hardened-xpriv batch scaling (Apple Silicon, Metal-backed local prover):

| N | Kind | Receipt bytes | Seal bytes | Claim JSON | Inclusion JSON | Prove sec | Prove RSS | Verify sec |
|---|------|---------------|------------|------------|----------------|-----------|-----------|------------|
| 2 | composite | 681,214 | 679,904 | 755 | 456 | 2.06 | 3.16 GB | 0.04 |
| 2 | succinct  | 223,343 | 222,668 | 755 | 456 | 5.35 | 3.17 GB | 0.02 |
| 4 | composite | 1,138,062 | 1,135,864 | 756 | 528 | 3.66 | 6.03 GB | 0.06 |
| 4 | succinct  | 223,343 | 222,668 | 755 | 528 | 9.44 | 6.02 GB | 0.02 |
| 8 | composite | 2,042,158 | 2,038,184 | 756 | 600 | 7.31 | 11.73 GB | 0.10 |
| 8 | succinct  | 223,343 | 222,668 | 755 | 600 | 18.35 | 11.73 GB | 0.02 |
| 16 | composite | 4,072,409 | 4,064,720 | 757 | 673 | 11.50 | 11.82 GB | 0.21 |
| 16 | succinct  | 223,343 | 222,668 | 756 | 673 | 34.91 | 11.81 GB | 0.02 |

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
2. One pinned `leaf_guest_image_id` per batch (no mixed-schema batches)
3. Fixed-size 84-byte batch claim (root-based, not leaf-enumerating)
4. Both batch-only and sparse-inclusion verification modes
5. Both hardened-xpriv and full Taproot leaf schemas validated
