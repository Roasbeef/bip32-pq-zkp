# Recursive Proving: How Batch Aggregation Works Under the Hood

## The Problem

Imagine you hold a Bitcoin wallet with a thousand UTXOs, each locked to a
different Taproot key derived from your BIP-32 seed. You can prove ownership of
any single key inside a zkVM: feed the seed and derivation path as private
input, run the BIP-32 math, and the VM produces a receipt showing you know the
seed behind that key.

Now scale that up. A thousand separate proofs means a thousand separate
receipts. A verifier who wants to check all of them — say, as part of a
post-quantum migration — has to download and verify each one individually. The
total bandwidth is a thousand times the cost of a single proof.

The solution is **recursive composition**: one proof that vouches for many
others. A single batch receipt says "I checked a thousand child proofs, and they
were all valid." The verifier checks one receipt. The child proofs never leave
the prover's machine.

This document explains how that works, from the inside out.

## The Recursion Trick

The naive approach to combining proofs would be to run a full STARK verifier
inside the guest program. The guest would take each child receipt, verify it
step by step, and then commit a summary. But a STARK verifier is itself an
expensive computation — millions of VM cycles per receipt. Verifying a thousand
receipts this way would cost billions of cycles.

risc0 uses a much cheaper trick. Instead of verifying child receipts inside the
guest, the guest simply *claims* that valid receipts exist. The actual
verification happens later, outside the guest, in a specialized recursion
circuit that is designed specifically for folding proofs together.

The guest's job is reduced to bookkeeping: compute a cryptographic commitment to
each child claim, register it as an assumption, and let the proving
infrastructure handle the rest.

## Two Sides of the Same Coin

Recursive composition splits the work between the guest and the host, and each
side sees a different representation of the same dependency set.

**The guest sees digests.** When the batch guest calls `zkvm.Verify(imageID,
journal)`, it does not touch the child receipt bytes. Instead, it computes a
SHA-256 digest chain that uniquely identifies the child claim:

```
journalDigest    = SHA256(child_journal_bytes)
postStateDigest  = taggedStruct("risc0.SystemState", [zero], [0])
outputDigest     = taggedStruct("risc0.Output", [journalDigest, zero])
claimDigest      = taggedStruct("risc0.ReceiptClaim",
                     [zero, imageIDWords, postStateDigest, outputDigest], [0, 0])
```

Each `taggedStruct` is risc0's domain-separated hashing convention:
`SHA256(SHA256(tag) || field_0 || field_1 || ... || count)`. The result is a
single 256-bit digest that commits to the child's identity (image ID), its
public output (journal), and the fact that it ran to normal completion.

The guest then passes this digest to `sys_verify_integrity` — a zkVM syscall
that registers the claim — and folds it into a running *assumptions digest*, a
hash-based linked list that accumulates every child claim the guest depends on.

**The host sees concrete receipts.** The caller passes the actual serialized
child receipts in the `Assumptions` field of the prove request:

```go
// Host-side code (Go)
result, err := client.Prove(host.ProveRequest{
    GuestBinary: batchGuestBinary,
    Stdin:       batchWitness,
    Assumptions: []host.AssumptionReceipt{
        childReceiptA,   // []byte: serialized succinct receipt
        childReceiptB,
        childReceiptC,
    },
}, host.WithReceiptKind(host.ReceiptKindSuccinct))
```

These receipt bytes cross the Go/Rust FFI boundary as base64-encoded JSON
fields, get decoded on the Rust side, and are attached to the executor
environment via `builder.add_assumption(receipt)`.

**The contract between the two sides is exact.** The guest builds a linked list
of claim digests. The host supplies a list of concrete receipts. During proof
generation, the risc0 recursion pipeline checks that every guest-side digest
matches a host-side receipt. If a single digest is wrong — because the image ID
was off by a byte, or the journal bytes were not exactly what the child guest
committed — the proof fails.

## Inside the Guest: The Verify Call

Here is what happens when the batch guest calls `zkvm.Verify` for one leaf
(source: `go-zkvm/zkvm/verify.go`):

```go
func Verify(imageID [32]byte, journal []byte) {
    // Ensure the running journal hasher is initialized.
    if !properHasherInitialized {
        InitProperHasher()
    }

    var zeroDigest [8]uint32
    imageIDWords := digestBytesToWords(imageID)

    // Step 1: Hash the child journal.
    journalDigest := shaBuffer(sha256InitStateBE, journal)

    // Step 2: Build the child's post-execution state (all zeros).
    postStateDigest := taggedStructWithData(
        "risc0.SystemState", [][8]uint32{zeroDigest}, []uint32{0},
    )

    // Step 3: Combine journal + empty assumptions into the output.
    outputDigest := taggedStruct(
        "risc0.Output",
        [][8]uint32{journalDigest, zeroDigest},
    )

    // Step 4: Build the full receipt claim digest.
    claimDigest := taggedStructWithData(
        "risc0.ReceiptClaim",
        [][8]uint32{zeroDigest, imageIDWords, postStateDigest, outputDigest},
        []uint32{0, 0},
    )

    // Step 5: Register the assumption with the zkVM.
    C.sys_verify_integrity(
        (*C.uint32_t)(unsafe.Pointer(&claimDigest[0])),
        (*C.uint32_t)(unsafe.Pointer(&zeroDigest[0])),
    )

    // Step 6: Update the running assumptions linked list.
    addAssumptionDigest(claimDigest, zeroDigest)
}
```

Steps 1 through 4 reconstruct the exact digest chain that the Rust guest SDK
(`env::verify`) would produce. The Go implementation must match the Rust one
byte-for-byte, which means getting every detail right: the SHA-256 initial-state
byte-swap convention (risc0 uses little-endian word order), the tagged-struct
field ordering, the scalar encoding. A single bit of divergence produces a
different digest, and the proof fails silently during the resolve phase.

Step 5 is the syscall. On the host side, `sys_verify_integrity` checks that a
matching receipt exists in the session's assumption registry. If it does, the
receipt is marked as accessed. If it does not, execution halts immediately —
the guest crashes.

Step 6 updates the running assumptions digest, which is a cons-cell linked list:

```
after Verify(A):
  list = Assumptions(Assumption(claimA, zero), empty)

after Verify(B):
  list = Assumptions(Assumption(claimB, zero), list)
```

When the guest calls `Halt`, the finalized journal digest and the assumptions
digest are combined into the `risc0.Output` tagged struct, which becomes the
receipt's public claim. This output commits to *both* the guest's own public
journal *and* every child proof it depended on.

## Inside the Host: The Recursion Pipeline

When the host calls `Prove` with assumptions, the risc0 prover runs a
multi-stage pipeline:

### 1. Execute

The RISC-V interpreter runs the batch guest. Each `sys_verify_integrity` call
is intercepted — the host checks that a matching receipt exists, but does not
yet verify it cryptographically. This is fast: the same speed as a non-composed
execution.

### 2. Segment and Lift

The execution trace is split into segments (chunks sized to fit the STARK
circuit), and each segment is "lifted" into a succinct receipt. Lifting is the
first use of the recursion circuit: a separate, non-Turing-complete circuit
optimized for STARK verification. The lifted receipt is constant-size regardless
of the segment length.

### 3. Join

The lifted segment receipts are joined pairwise into a single succinct receipt,
like reducing a tree. The join circuit verifies two receipts and produces one
merged receipt. After `log2(segments)` rounds, there is one receipt — but it
still carries the unresolved assumptions list.

### 4. Resolve

This is where the child receipts are consumed. The resolve circuit takes the
conditional receipt (with assumptions) and one child receipt, verifies that the
child receipt's claim digest matches the head of the assumptions list, and
produces a new receipt with one fewer assumption. This repeats for each
assumption until the list is empty.

The resolve step is LIFO: the *last* assumption registered by the guest is
resolved *first*. If the batch guest called `Verify(A)` then `Verify(B)` then
`Verify(C)`, the resolve order is C, B, A.

### 5. Unconditional Receipt

Once all assumptions are resolved, the final receipt has an empty assumptions
list. It is **unconditional** — any verifier can check it without needing the
child receipts. The child receipts were prover-side inputs. They do not appear
in the final artifact and never need to leave the prover's machine.

## What the Batch Guest Produces

After calling `Verify` for every leaf, the batch guest builds a Merkle tree
over the ordered leaf journals and commits a fixed-size 84-byte batch claim:

```
[version         : u32 LE]  — claim format (1 or 2)
[flags           : u32 LE]  — policy bits (currently 0)
[leaf_claim_kind : u32 LE]  — which leaf schema was batched
[merkle_hash_kind: u32 LE]  — which hash for the tree (1 = SHA-256)
[leaf_count      : u32 LE]  — number of leaves
[context_digest  : 32 bytes] — shared leaf image ID or policy digest
[merkle_root     : 32 bytes] — root of the leaf Merkle tree
```

This claim is the same size whether the batch contains 2 leaves or 200. The
fan-out is captured by the Merkle root, not by listing individual leaves. A
verifier who wants to inspect one specific leaf uses a standard Merkle inclusion
proof to check that the leaf is part of the committed root.

See [batch-merkle-system.md](batch-merkle-system.md) for the full details of
the Merkle tree construction, claim schemas, and the various leaf kinds.

## Why Succinct Receipts Matter

The risc0 prover can produce two kinds of output: **composite** receipts
(cheaper to generate, but larger) and **succinct** receipts (more expensive,
but constant-size). For batch aggregation, the distinction is critical in two
places:

**Leaf receipts must be succinct.** The recursion pipeline requires each
assumption receipt to be a succinct, constant-size proof. Composite receipts —
which are bundles of per-segment proofs — cannot serve as assumptions. This
means every leaf proof must be compressed to succinct form before it can be
aggregated.

**The final batch receipt can be either.** A composite batch receipt is faster
to generate but grows with N. A succinct batch receipt takes longer to produce
but stays at roughly 223 KB regardless of batch size. For distribution to
external verifiers, succinct is almost always the right choice.

In practice, the leaf proving and compression step is a one-time cost. Once
you have succinct leaf receipts, you can re-aggregate them into different batch
configurations without re-proving the leaves.

## The Complete Flow

Putting it all together, a batch proof goes through these stages:

```
1. PROVE LEAVES
   For each key/UTXO:
     seed + path  →  leaf guest  →  72-byte claim + composite receipt
     composite receipt  →  succinct receipt  (~223 KB each)

2. AGGREGATE
   Host:
     load N succinct leaf receipts
     build batch witness stdin (leaf journals + metadata)
   Batch guest:
     for each leaf: zkvm.Verify(leafImageID, leafJournal)
     build Merkle root over ordered leaf journals
     commit 84-byte batch claim
   Prover:
     execute → segment → lift → join → resolve → unconditional receipt

3. VERIFY
   Verifier receives:
     one batch receipt  (~223 KB succinct)
     one batch claim.json  (~755 bytes)
     one Merkle inclusion proof  (O(log N) sibling hashes)
   Verifier checks:
     receipt valid for batch guest image ID?
     batch claim decoded from journal?
     disclosed leaf included in committed Merkle root?
```

The verifier's work is independent of N. Whether the batch contains 2 leaves or
2,000, the verifier checks one receipt, reads one claim, and walks one
logarithmic Merkle branch.

## Nested Hierarchies

The batch guest can aggregate child batch claims as leaves, creating a
hierarchy. A parent batch treats each child's 84-byte batch claim as a leaf,
calls `Verify` against the child batch guest's receipt, and commits a new
parent-level Merkle root.

Verification walks the hierarchy level by level: verify the parent receipt,
prove one child is included in the parent root, decode the child claim, prove
one original leaf is included in the child root. Each level adds one Merkle
branch. The final receipt size stays flat.

Heterogeneous parents can mix raw leaves and child batch claims at the same
level using fixed-size envelopes. See [batch-merkle-system.md](batch-merkle-system.md)
for the full hierarchy and envelope design.
