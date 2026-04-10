# Recursive Batch Aggregation Design

## Goal

The next protocol direction is to aggregate many leaf proofs into one final
proof artifact while keeping the final verifier-facing claim small.

The target user experience is:

1. produce `N` leaf proofs for `N` keys or UTXOs
2. aggregate them into one final batch proof
3. publish one final receipt plus one small batch `claim.json`
4. let verifiers check either:
   - the whole batch at once, or
   - one disclosed leaf with a normal Merkle inclusion proof

The important distinction is:

- the zk receipt proves that the committed batch root was derived correctly
- the Merkle branch proves that one disclosed leaf is included in that root

## Current Prototype Status

The first v1 batch prototype is now implemented in this repo.

Working pieces:

- a reusable batch guest in `guest_batch/`
- Go host support for supplying assumption receipts through `go-zkvm/host`
- one fixed-size batch claim in `batchclaim/Claim`
- external sparse inclusion proofs in `BatchInclusionProofFile`
- Makefile and CLI flows for:
  - `execute-batch`
  - `prove-batch`
  - `verify-batch`
  - `derive-batch-inclusion`

Currently validated:

- hardened-xpriv leaf batches
- original full Taproot leaf batches
- one leaf schema per batch
- one pinned `leaf_guest_image_id` per batch
- SHA-256 Merkle trees
- batch-only verification
- batch verification plus external Merkle inclusion checking

Current measured hardened-xpriv batch scaling results:

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

The important result is that once the final batch guest itself is proven as
`succinct`, the final batch artifact stays on the same order of magnitude as
the single-leaf succinct receipts, while the public batch claim grows only by
the fixed metadata plus the 32-byte Merkle root.

There is now also a smaller confirmation matrix for the original full Taproot
leaf schema:

| Leaf kind | N | Kind | Receipt bytes | Claim JSON | Inclusion JSON | Prove sec |
|-----------|---|------|---------------|------------|----------------|-----------|
| taproot | 2 | composite | 681,214 | 748 | 449 | 2.64 |
| taproot | 2 | succinct  | 223,343 | 748 | 449 | 6.43 |
| taproot | 8 | composite | 2,042,158 | 749 | 593 | 10.35 |
| taproot | 8 | succinct  | 223,343 | 748 | 593 | 21.22 |

The Taproot confirmation points land very close to the hardened-xpriv numbers.
That strongly suggests the current batch cost is dominated by verifying `N`
succinct leaf receipts plus hashing their journals into the Merkle tree,
rather than by the original leaf-statement semantics.

## What The Batch Receipt Actually Buys

For sparse disclosure, the right comparison is:

- `N` separate succinct leaf receipts
- versus one final succinct batch receipt plus one inclusion proof

Using the hardened-xpriv lane, the single-leaf succinct receipt is `223,319`
bytes on disk. The final succinct batch receipt stayed at `223,343` bytes
across the current matrix.

That means the verifier-facing artifact comparison for “prove one leaf is in
this validated batch” looks like:

| N | Separate leaf receipts only | Final succinct batch + claim + inclusion |
|---|-----------------------------|-------------------------------------------|
| 2 | 446,638 B | 224,554 B |
| 4 | 893,276 B | 224,626 B |
| 8 | 1,786,552 B | 224,698 B |
| 16 | 3,573,104 B | 224,772 B |

This is the main practical win of the current design:

- final proof distribution stays almost flat
- sparse verifier disclosure grows only with the Merkle branch depth
- verifier cost stays tiny even as the batch gets larger

Important caveat:

- this is the right comparison for sparse verification or “show me one leaf”
- it is not the right comparison for full disclosure of every leaf in the
  batch, because in that case you still need to ship all leaf public claims
  somehow

## What RISC Zero Actually Supports

The guest-side composition interface is:

- `env::verify(image_id, journal)`
- `env::verify_integrity(receipt_claim)`
- `env::verify_assumption(...)`
- `env::verify_assumption2(...)`

For ordinary zkVM receipt aggregation, the relevant APIs are
`env::verify(...)` and `env::verify_integrity(...)`. These add assumptions to
the receipt claim during guest execution.

The host-side recursion pipeline then uses internal recursion programs:

- `lift`
- `join`
- `resolve`

The key implication is:

- aggregation is not a single "aggregate these proofs" guest syscall
- aggregation is guest composition plus host-side recursive assumption
  resolution

Another important constraint from the current `risc0` host API:

- `ExecutorEnv::add_assumption(...)` does not accept `CompositeReceipt`
- leaf receipts must already be compressed to `SuccinctReceipt` before they can
  be fed in as assumptions

So the realistic batch pipeline is:

1. generate unconditional leaf receipts
2. compress leaf receipts to succinct form
3. run an aggregation guest that verifies those leaf claims
4. ask the host to return one final receipt, ideally also succinct

## Recommended Batch Architecture

### Stage 1: Leaf Proofs

Each leaf proof should prove one narrow derivation statement.

For the current repo, the candidate leaves are:

- full Taproot claim
- hardened xpub claim
- hardened xpriv claim

For pure proving efficiency, the hardened-xpriv lane is the best current leaf:

- no EC point multiplication in the guest
- composite receipts are already close to the succinct floor
- proving time is dramatically lower than the other two lanes

However, there is a policy caveat:

- if a verifier later needs to see the disclosed leaf data, an xpriv leaf
  reveals xpriv material
- under the specific "EC spending is already broken" threat model this may be
  acceptable
- for a broader public verifier story, hardened-xpub or a future spend-bound
  public leaf may be preferable

So the practical recommendation is:

- use hardened-xpriv leaves first to validate aggregation mechanics and measure
  costs
- then also implement aggregation for the original full Taproot claim so there
  is a privacy-preserving aggregation path that does not reveal xpriv material
- decide later whether the long-term public leaf schema should remain xpriv,
  move to xpub, stay on the full Taproot claim, or move to a future
  spend-bound claim

### Stage 2: Aggregation Guest

The aggregation guest should:

1. read the leaf public claims as private witness bytes
2. for each leaf, call `env::verify(...)` or `env::verify_integrity(...)`
   against the expected leaf claim
3. hash the ordered leaf records into one Merkle root
4. commit only a compact batch claim to the journal

The batch guest does not need to publish every leaf in its journal. It only
needs to commit a root and enough metadata to tell verifiers how to interpret
that root.

### Stage 3: Final Batch Receipt

The host runs the aggregation guest with all leaf receipts supplied as
assumptions.

The result should be:

- one unconditional final receipt
- preferably `ReceiptKind::Succinct`

That final receipt is the standalone proof artifact distributed to verifiers.

## Recommended Batch Claim Shape

The final journal should remain fixed-size.

Recommended `batch_claim_v1` fields:

- `batch_version: u32`
- `batch_flags: u32`
- `leaf_claim_kind: u32`
- `merkle_hash_kind: u32`
- `leaf_count: u32`
- `leaf_guest_image_id: [32]byte`
- `merkle_root: [32]byte`

This keeps the final public claim small while still pinning:

- how leaves should be interpreted
- which leaf guest image produced them
- how many leaves the root commits to
- which Merkle hash algorithm was used

For v1, `leaf_guest_image_id` is pinned once per batch, not per leaf. In
practice that means every leaf in one batch must come from the same guest
program and exact guest artifact. That keeps the final claim fixed-size and
avoids mixed-schema batches in the first implementation.

This is the direct analogue of the current single-leaf `claim.json` contract:

- the receipt proves execution of the aggregator image
- the journal proves one exact batch root claim

## Recommended Leaf Encoding

The Merkle tree should commit to a deterministic leaf encoding.

Recommended `leaf_hash_v1` input:

```text
leaf_hash = H(
  tag ||
  leaf_index_le32 ||
  leaf_claim_bytes
)
```

Where:

- `tag` is a domain separator like `\"bip32-pq-zkp:batch-leaf:v1\"`
- `leaf_index` fixes ordering
- `leaf_claim_bytes` is the exact leaf public claim serialization

The batch claim already carries `leaf_claim_kind` and `leaf_guest_image_id`, so
those do not need to be duplicated in every leaf hash unless we later want
mixed leaf kinds inside one tree.

## Merkle Tree Recommendation

Use a normal binary Merkle tree with explicit domain separation between:

- leaves
- interior nodes

The simplest audit-friendly v1 is:

- `leaf_node = H(0x00 || leaf_hash_input)`
- `inner_node = H(0x01 || left || right)`

There are two plausible hash choices:

- SHA-256
  - easier for external verifiers and existing Bitcoin-adjacent tooling
  - more expensive inside the guest
- Poseidon2
  - cheaper inside the guest
  - requires external verifiers to implement Poseidon2 for inclusion checks

Chosen v1 direction:

1. use SHA-256 for the first design and verifier story
2. revisit Poseidon2 only if Merkle-root construction becomes a measurable
   proving bottleneck

The chosen Merkle hash must be carried in `merkle_hash_kind`.

## Verifier Flow

### Verify The Batch Itself

To verify the batch receipt:

1. load the final batch receipt
2. verify it against the aggregation guest image ID
3. decode the batch claim from the journal
4. record:
   - `leaf_claim_kind`
   - `leaf_guest_image_id`
   - `leaf_count`
   - `merkle_hash_kind`
   - `merkle_root`

At this point the verifier knows:

- there exist valid leaf receipts of the pinned leaf type and image
- the aggregator guest verified them
- the committed Merkle root was built from those verified leaves

### Verify One Disclosed Leaf

For sparse verification of one key, the verifier then receives:

- the final batch receipt
- the batch `claim.json`
- the disclosed leaf claim bytes
- the leaf index
- a Merkle branch for that leaf

The verifier does:

1. verify the final batch receipt
2. recompute the leaf hash from the disclosed leaf bytes and index
3. recompute the Merkle root from the branch
4. compare that root to the `merkle_root` in the batch claim
5. apply application-specific checks to the disclosed leaf claim

No leaf receipt needs to be distributed to that verifier. The final batch
receipt already proves that the committed root came from valid leaf receipts.

This gives two verifier modes:

- batch-only verification
  - verify just the final batch receipt and batch claim
- sparse verification
  - verify the final batch receipt, then check one disclosed leaf with a
    normal Merkle branch
  - the Merkle branch can be produced outside the guest from the batch leaf
    set

## What The Final Verification Cost Looks Like

If the final receipt is succinct and the batch journal is fixed-size, then the
verifier-facing zk artifact stays in the same general size class regardless of
`N`.

That does not mean the final proof is literally byte-for-byte constant. It
means:

- the final receipt size should stay near the succinct floor
- the final receipt verification cost should stay roughly constant
- the extra verifier work per disclosed leaf becomes ordinary Merkle-path
  hashing, i.e. `O(log N)`

So the full verifier cost becomes:

- one final receipt verification
- plus `k` Merkle inclusions for `k` disclosed leaves

This is exactly the property we want for large batches.

## What Grows And What Does Not

### Roughly Constant

- final succinct receipt size, if the batch journal is fixed-size
- final succinct receipt verification API and cost class
- batch `claim.json` size, if it only carries batch metadata and the root

### Grows With `N`

- total leaf proving work
- total recursive aggregation work
- aggregation guest execution time
- proving latency

### Grows With Number Of Disclosed Leaves

- number of leaf records revealed to verifiers
- number of Merkle branches shipped
- verifier-side branch checking work

### Important Trap

If the final journal includes all `N` leaves directly, then:

- the claim grows linearly
- the final receipt file also grows with the journal
- succinct no longer gives the clean constant-size verifier artifact we want

That is why the root-based claim is the right design.

## Which Guest Syscall To Use

For a normal batch of zkVM leaf receipts, use:

- `env::verify(image_id, journal)` when the leaf statement can be described by
  its image ID and journal bytes
- `env::verify_integrity(receipt_claim)` if the aggregator wants to work at the
  full claim level

Do not start with `verify_assumption2` for the ordinary leaf-batch case.

`verify_assumption2` is the lower-level path for advanced composition flows,
such as when a guest needs to materialize an unresolved assumption directly for
later resolution. It is useful, but it is not the simplest path for the
standard "aggregate N zkVM leaves" design.

## Recommended v1 Plan

### Phase 1: Mechanics

Build a batch aggregator around the hardened-xpriv leaf lane.

Why:

- fastest current leaf
- cheapest proving path
- easiest way to validate the recursion/composition architecture

Deliverables:

- one aggregation guest
- one fixed-size batch claim
- one final succinct receipt
- one Merkle inclusion verifier for disclosed leaves

### Phase 2: Public Leaf Policy

Once the mechanics are validated, decide whether the public leaf reveal format
should remain hardened-xpriv or move to:

- hardened-xpub
- the original full Taproot claim
- or a future spend-bound public leaf

This is a protocol-policy question, not a recursion question.

The current intended order is:

1. implement aggregation for hardened-xpriv first
2. then implement the same aggregation shape for the original full Taproot
   scheme so there is a privacy-preserving batch path

### Phase 3: Multi-UTXO Authorization

If the long-term goal is one proof covering many UTXOs, then the batch leaf
should eventually evolve from "derivation only" into a public authorization
record that can be checked against actual spends.

That future leaf likely needs:

- a public key or spend-binding output
- maybe a sighash binding
- maybe an outpoint or UTXO identifier

But the aggregation mechanics can be validated before that semantic expansion.

## Current v1 Design Decisions

1. Use SHA-256 for the first batch Merkle tree.
2. Pin `leaf_guest_image_id` once per batch, which means one guest image per
   batch.
3. Start from the hardened-xpriv leaf lane for the first implementation.
4. Keep one leaf schema per batch for v1; do not support mixed leaf kinds yet.
5. Support both:
   - batch-only verification of the final proof
   - sparse verification of one disclosed leaf via an ordinary Merkle branch
6. After the hardened-xpriv path is working, also implement aggregation for the
   original full Taproot scheme so there is a privacy-preserving batch mode.

## Remaining Open Questions

1. Is hardened-xpriv acceptable as a disclosed sparse-verification leaf under
   the intended threat model, or should it remain an internal proving-only
   comparison lane?
2. What the public batch leaf should become once spend-binding is added later:
   - hardened-xpub
   - full Taproot claim
   - or a future spend-bound authorization claim

## Bottom Line

The right way to think about the recursive design is:

- leaf proofs establish the hard per-key relation
- the aggregation guest verifies those leaf receipts and commits one root
- the final succinct receipt proves the correctness of that root claim
- sparse verification is then handled with ordinary Merkle inclusion proofs,
  not with separate leaf receipts

If the final journal is just the root plus batch metadata, then the final
succinct receipt stays in the same general verifier-facing size class even as
`N` grows, while total prover work scales with the batch size.
