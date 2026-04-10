# The Batch Merkle System: Claims, Trees, and Envelopes

## What This Document Covers

The [recursive proving guide](recursive-proving.md) explains *how* the batch
guest verifies child proofs and produces a single receipt. This document
explains *what* goes into that receipt: the Merkle tree that commits to the
ordered leaf set, the claim schemas that describe each leaf, the envelope
format that enables mixed parents, and the verification flow that lets a
verifier inspect one leaf without seeing the rest.

## The Batch Claim: 84 Bytes That Commit to Everything

Every batch receipt carries a fixed-size 84-byte public claim in its journal.
Regardless of whether the batch contains 2 leaves or 2,000, the claim is always
the same size. Here is the layout:

```
Offset  Size  Field              Description
──────  ────  ─────              ───────────
 0       4    version            1 for homogeneous, 2 for heterogeneous parent
 4       4    flags              Policy bits (currently 0)
 8       4    leaf_claim_kind    Which leaf schema: 1=taproot, 2=hardened-xpriv,
                                 3=batch-claim-v1, 4=heterogeneous-envelope-v1
12       4    merkle_hash_kind   Which hash for the tree: 1=SHA-256
16       4    leaf_count         Number of direct children
20      32    context_digest     Shared leaf image ID (v1) or policy digest (v2)
52      32    merkle_root        Root of the leaf Merkle tree
```

All integers are unsigned 32-bit little-endian. The 32-byte `context_digest`
field serves double duty: for homogeneous batches (version 1), it holds the
single guest image ID shared by all leaves; for heterogeneous parents (version
2), it holds a policy digest that commits to the envelope schema and allowed
child kinds.

The corresponding Go type lives in `batchclaim/claim.go`:

```go
type Claim struct {
    Version          uint32
    Flags            uint32
    LeafClaimKind    uint32
    MerkleHashKind   uint32
    LeafCount        uint32
    LeafGuestImageID [32]byte   // context_digest: image ID or policy digest
    MerkleRoot       [32]byte
}
```

The `Encode` method serializes the claim into its 84-byte wire format. `Decode`
parses it back. `UsesPolicyDigest` reports whether the context field should be
interpreted as a policy digest rather than an image ID.

## The Leaf Kinds

Each batch aggregates leaves of one schema. The `leaf_claim_kind` field in the
batch claim identifies which schema every leaf conforms to.

### Taproot (`leaf_claim_kind = 1`, 72 bytes)

The original full Taproot claim. The guest proves: "I know a BIP-32 seed and
derivation path that produce this Taproot output key."

```
[version : u32 LE]           — claim format
[flags : u32 LE]             — policy bits (bit 0 = require BIP-86)
[taproot_output_key : 32]    — the derived x-only public key
[path_commitment : 32]       — SHA-256 commitment to the derivation path
```

This is the strongest statement: it commits to the final on-chain key. It is
also the most expensive to prove (~49 seconds) because it requires full
elliptic-curve point multiplication for BIP-86 Taproot tweaking.

### Hardened XPriv (`leaf_claim_kind = 2`, 72 bytes)

The reduced hardened-xpriv claim. The guest proves: "I know a parent extended
private key and chain code, and I can derive one hardened child step."

```
[version : u32 LE]           — claim format
[flags : u32 LE]             — policy bits
[child_private_key : 32]     — the derived child private key scalar
[chain_code : 32]            — the derived child chain code
```

This is the cheapest leaf proof (~2 seconds). It avoids elliptic-curve
arithmetic entirely — the derivation is pure HMAC-SHA512 plus a scalar
addition. The trade-off is that the claim discloses the child private key, which
is acceptable under the threat model where EC-based spending is already broken.

### Batch Claim V1 (`leaf_claim_kind = 3`, 84 bytes)

A serialized child batch claim used as a leaf inside a parent batch. This is
what enables **nested hierarchies**: the parent batch aggregates child batch
claims rather than raw leaf journals.

The leaf payload is simply the 84-byte `Claim.Encode()` output from the child
batch. The parent guest calls `zkvm.Verify` against the child batch guest's
image ID with the 84-byte child claim as the expected journal.

When building nested batches, the parent verifier enforces that all
`batch_claim_v1` children share the same **subtree policy**: the same child
batch version, flags, leaf kind, Merkle hash kind, and leaf guest image ID.
This enforcement happens both inside the guest (provably, as part of the STARK)
and on the host side (as an early sanity check during leaf loading).

### Heterogeneous Envelope V1 (`leaf_claim_kind = 4`, 128 bytes)

A fixed-size direct-child envelope that enables **heterogeneous parents** —
batches that mix different child kinds at one level. For example, a parent can
aggregate one raw hardened-xpriv leaf, one raw Taproot leaf, and one child
batch claim under a single Merkle root.

The envelope layout:

```
Offset  Size  Field              Description
──────  ────  ─────              ───────────
 0       4    envelope_version   Currently 1
 4       4    direct_leaf_kind   The embedded child's schema (1, 2, or 3)
 8      32    verify_image_id    Image ID for this child's receipt
40       4    journal_len        Actual byte length of the embedded journal
44      84    padded_journal     Child journal padded to maximum size
```

Total: 128 bytes. The padding ensures every leaf in the Merkle tree has the
same fixed size, regardless of whether the embedded child journal is 72 bytes
(Taproot or xpriv) or 84 bytes (batch claim).

The corresponding Go type lives in `batchclaim/envelope.go`:

```go
type HeterogeneousEnvelopeV1 struct {
    Version        uint32
    DirectLeafKind uint32
    VerifyImageID  [32]byte
    JournalLen     uint32
    PaddedJournal  [84]byte   // max size = PublicClaimSize
}
```

`NewHeterogeneousEnvelopeV1` constructs an envelope and validates that the
direct child kind is in the allowed set. `Encode` serializes to 128 bytes.
`DecodeHeterogeneousEnvelopeV1` parses and validates. `JournalBytes` extracts
the real, unpadded child journal.

**Allowed direct child kinds** (defined in `IsAllowedHeterogeneousDirectLeafKindV1`):
- Taproot (`1`)
- Hardened XPriv (`2`)
- Batch Claim V1 (`3`)

When the batch claim version is 2 (heterogeneous parent), the 32-byte context
digest becomes a **policy digest** computed from:

```go
func HeterogeneousPolicyDigestV1() [32]byte {
    data := "bip32-pq-zkp:heterogeneous-parent-policy:v1"
    data += le32(EnvelopeVersionV1)
    data += le32(EnvelopeSizeV1)         // 128
    data += le32(MaxJournalSizeV1)       // 84
    data += le32(LeafKindTaproot)        // 1
    data += le32(LeafKindHardenedXPriv)  // 2
    data += le32(LeafKindBatchClaimV1)   // 3
    return SHA256(data)
}
```

This digest gives verifiers a stable anchor. When a verifier sees
`batch_version = 2` and recognizes the policy digest, they know exactly which
envelope schema and child kinds the parent is allowed to contain, without
needing to inspect every leaf.

## The Merkle Tree

The batch guest commits to the ordered leaf set by building a binary Merkle
tree and writing only the root into the batch claim. The tree uses
domain-separated hashing to prevent structural attacks.

### Leaf Hashing

Each leaf is hashed with a `0x00` prefix, a domain tag, and its position index:

```go
func LeafHash(index uint32, leafClaim []byte, hash HashFunc) [32]byte {
    var buf []byte
    buf = append(buf, 0x00)                              // leaf prefix
    buf = append(buf, "bip32-pq-zkp:batch-leaf:v1"...)   // domain tag
    buf = binary.LittleEndian.AppendUint32(buf, index)   // position
    buf = append(buf, leafClaim...)                       // leaf payload
    return hash(buf)
}
```

The `0x00` prefix distinguishes leaf nodes from inner nodes (which use `0x01`).
Without this prefix, an attacker could construct an inner node whose hash
collides with a leaf hash, creating a second-preimage attack.

The domain tag `"bip32-pq-zkp:batch-leaf:v1"` prevents cross-protocol
confusion — a Merkle proof from one system cannot be replayed against another.

The index binding prevents leaf reordering — swapping leaves 0 and 1 produces
different hashes even if the leaf payloads are identical.

### Inner Node Hashing

Inner nodes hash their two children with a `0x01` prefix:

```go
func InnerHash(left, right [32]byte, hash HashFunc) [32]byte {
    var buf [65]byte
    buf[0] = 0x01                       // inner prefix
    copy(buf[1:33], left[:])
    copy(buf[33:65], right[:])
    return hash(buf[:])
}
```

### Tree Construction

The `Root` function builds the tree bottom-up:

1. Hash each leaf with `LeafHash(index, payload, hash)`.
2. If the level has an odd number of nodes, duplicate the last one
   (Bitcoin-style padding).
3. Pair adjacent nodes with `InnerHash(left, right, hash)`.
4. Repeat until one node remains. That is the root.

The `hash` parameter is a function `func([]byte) [32]byte`. Inside the zkVM
guest, this is `zkvm.SumSHA256`, which uses the RISC-V SHA acceleration
syscalls. On the host side, it is `crypto/sha256.Sum256`. Both produce
identical results for the same input — the guest's accelerated path is just
faster in terms of VM cycles.

### Building an Inclusion Proof

`BuildProof` derives a Merkle branch for one leaf:

```go
proof, root, err := batchclaim.BuildProof(leaves, targetIndex, sha256Sum)
```

The proof contains:
- `LeafIndex`: position of the disclosed leaf
- `LeafCount`: total leaves in the tree
- `LeafClaim`: the disclosed leaf's raw payload
- `Siblings`: sibling hashes from leaf to root (one per tree level)

### Verifying an Inclusion Proof

`VerifyProof` recomputes the root from the disclosed leaf and siblings:

```go
ok := batchclaim.VerifyProof(committedRoot, proof, sha256Sum)
```

The verifier:
1. Computes `LeafHash(proof.LeafIndex, proof.LeafClaim, hash)`.
2. Walks up the sibling path, combining with `InnerHash` at each level.
3. Compares the recomputed root against the committed root from the batch
   claim.

If they match, the disclosed leaf is part of the committed batch. The verifier
did not need to see any other leaf — just the sibling hashes, which reveal
nothing about the other leaves' contents.

## Sparse Verification Flows

### Flat Batch: One Leaf from N

The simplest case. The verifier receives:

- The batch receipt (succinct, ~223 KB).
- The batch `claim.json` (human-readable, ~755 bytes).
- One inclusion proof JSON (log N sibling hashes).

The verification steps:

1. Verify the batch receipt against the batch guest image ID.
2. Decode the batch claim from the receipt's journal.
3. Optionally compare against the `claim.json` artifact.
4. Load the inclusion proof: disclosed leaf journal, index, siblings.
5. Recompute the Merkle root from the disclosed leaf.
6. Check that the recomputed root matches `merkle_root` in the batch claim.
7. Apply application-specific checks to the disclosed leaf (e.g., check that
   the Taproot output key matches a known UTXO).

### Nested Batch: One Leaf Through a Hierarchy

When batches are nested — a parent batch aggregates child batch claims — sparse
verification walks the hierarchy level by level.

The verifier receives:

- The parent batch receipt (one succinct receipt).
- A bundled inclusion-chain JSON artifact (one proof per hierarchy level).

The chain contains, in order from top to bottom:

1. A proof that child batch B is included in the parent Merkle root.
2. A proof that original leaf X is included in child batch B's Merkle root.

The verification code in `VerifyBatchInclusionChain` processes the chain:

```go
func VerifyBatchInclusionChain(
    rootClaim batch.Claim, inclusions []BatchInclusionProofFile,
) ([]batch.Claim, error) {

    currentClaim := rootClaim
    for idx, inclusion := range inclusions {
        // Verify this level's Merkle proof.
        verifyBatchInclusionProof(currentClaim, inclusion)

        if idx == len(inclusions)-1 {
            break  // Final level: disclosed leaf is a raw claim.
        }

        // Non-final level: decode the disclosed leaf as a child batch claim.
        childClaim := decodeNestedBatchClaim(inclusion.LeafJournalHex)
        currentClaim = childClaim
    }
}
```

At each non-final level, the disclosed leaf is itself a batch claim. The
verifier decodes it and uses it as the claim for the next level's inclusion
check. At the final level, the disclosed leaf is a raw claim (Taproot,
hardened-xpriv, etc.) that the application can inspect directly.

### Heterogeneous Parent: Mixed Children

When the parent is a heterogeneous batch (version 2, leaf kind 4), the
disclosed "leaf" in the Merkle proof is a 128-byte envelope, not a raw
journal. The verifier must decode the envelope to extract the embedded child
journal:

1. Verify the Merkle proof against the parent's root (the leaf record is the
   full 128-byte envelope).
2. Decode the envelope to get `direct_leaf_kind`, `verify_image_id`, and the
   unpadded journal.
3. If the embedded child is a `batch_claim_v1`, decode it as a `Claim` and
   continue to the next level.
4. If the embedded child is a raw leaf (Taproot or xpriv), apply
   application-specific checks.

The inclusion proof JSON for heterogeneous parents carries extra fields:

```json
{
  "leaf_claim_kind": 4,
  "leaf_journal_hex": "...",
  "direct_leaf_kind": 3,
  "direct_leaf_kind_name": "batch_claim_v1",
  "direct_leaf_image_id": "8401a36e..."
}
```

These fields let the verifier identify what kind of child is embedded without
re-parsing the envelope from scratch.

## The Verifier's Artifact Set

For each verification scenario, the verifier needs a different set of
artifacts:

### Batch-Only (No Leaf Disclosure)

| Artifact | Size | Notes |
|----------|------|-------|
| Batch receipt | ~223 KB (succinct) | The STARK proof |
| Batch claim.json | ~755 B | Human-readable claim |

The verifier learns: "N valid leaf proofs exist, all from the pinned guest
image, with journals hashing to this root." They do not learn which specific
leaves are in the batch.

### Flat Sparse Disclosure

| Artifact | Size | Notes |
|----------|------|-------|
| Batch receipt | ~223 KB (succinct) | The STARK proof |
| Batch claim.json | ~755 B | Human-readable claim |
| Inclusion proof | ~450–700 B | O(log N) sibling hashes |

The verifier learns one specific leaf's public journal and confirms it is part
of the committed batch.

### Nested Sparse Disclosure

| Artifact | Size | Notes |
|----------|------|-------|
| Top-level receipt | ~223 KB (succinct) | One STARK proof |
| Top-level claim.json | ~755 B | Human-readable claim |
| Bundled inclusion chain | ~1.3–1.7 KB | One proof per hierarchy level |

The verifier walks the hierarchy and learns one original leaf's journal at the
bottom of the tree.

## Design Decisions

Several choices in the Merkle system are deliberate trade-offs:

**SHA-256 over Poseidon2.** SHA-256 is more expensive inside the zkVM guest,
but it aligns with Bitcoin-adjacent tooling and is trivially implementable by
external verifiers. Poseidon2 would reduce guest cycle count but would require
every verifier to implement a specialized hash function. The `merkle_hash_kind`
field in the batch claim makes this choice explicit and upgradeable.

**Fixed-size leaves over variable-size.** Every leaf in a batch has the same
byte size. This simplifies the guest code (no length prefixes, no variable
reads) and makes the Merkle tree straightforward to index. The heterogeneous
envelope achieves mixed child kinds by padding smaller journals up to the
maximum size within a fixed 128-byte envelope.

**Bitcoin-style odd-level duplication.** When a tree level has an odd number of
nodes, the last node is duplicated before pairing. This matches Bitcoin's Merkle
tree convention and avoids the complexity of balanced-tree padding schemes.

**Domain separation with prefixes and tags.** The `0x00`/`0x01` leaf/inner
prefix prevents second-preimage attacks. The `"bip32-pq-zkp:batch-leaf:v1"`
tag prevents cross-protocol confusion. The index binding prevents leaf
reordering. Together, these ensure that a valid inclusion proof can only be
constructed from the genuine ordered leaf set.

**Homogeneous-per-level enforcement for v1 nested batches.** When batching
`batch_claim_v1` leaves, both the guest and host enforce that all child batch
claims share the same subtree policy. This prevents a parent from silently
mixing children built from different guest binaries or leaf schemas. The
enforcement is provable — it runs inside the zkVM — not just advisory.

## Code Pointers

| Component | File | Key Functions |
|-----------|------|---------------|
| Batch claim | `batchclaim/claim.go` | `Encode`, `Decode`, `UsesPolicyDigest`, `HeterogeneousPolicyDigestV1` |
| Merkle tree | `batchclaim/merkle.go` | `LeafHash`, `InnerHash`, `Root`, `BuildProof`, `VerifyProof` |
| Envelope | `batchclaim/envelope.go` | `NewHeterogeneousEnvelopeV1`, `Encode`, `Decode`, `JournalBytes` |
| Batch guest | `guest_batch/main.go` | `main` (the proof program) |
| Host runner | `batch_runner.go` | `ProveBatch`, `VerifyBatch`, `DeriveBatchInclusionProof` |
| Host support | `batch_support.go` | `loadBatchLeaves`, `buildBatchWitnessStdin`, `VerifyBatchInclusionChain` |
| Leaf kind parser | `batch_leaf_kind.go` | `ParseBatchLeafKindName` |
| Nested wrapper | `nested_plan.go` | `RunNestedBatchPlan` |
