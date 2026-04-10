# Batch Aggregation: Future Directions

## Beyond The First Nested Layer

The first nested batch-of-batches layer is implemented. Child batches can be
built over chunks of leaves, then aggregated into a parent batch via
`batch_claim_v1`.

See `nested-batching.md` for the implemented first cut and its current
limitations. This file keeps the broader future-work context beyond that
initial layer.

For the design rationale behind the two most recent extensions, see:

- `heterogeneous-parent-plan.md`
- `nested-wrapper-plan.md`

For example, 15 leaves with chunk size 5:

- child batch A proves leaves 0..4
- child batch B proves leaves 5..9
- child batch C proves leaves 10..14
- parent batch P proves that batch claims A, B, and C are all valid

This gives:

- bounded fan-out per proving step
- incremental construction (add chunks over time)
- reuse of prior child batch receipts
- a path to parallel proving
- final succinct receipt that should still stay near the current floor

### What Changes For Verifiers

Sparse verification becomes a chain of inclusion proofs:

1. verify the final parent batch receipt
2. check inclusion of the child batch claim in the parent root
3. check inclusion of the original leaf in the child batch root

So the verifier sees: one receipt, one inclusion proof per hierarchy level,
and one disclosed leaf at the bottom.

### Remaining Open Questions

1. Whether the current heterogeneous direct-child envelope should remain
   narrow or expand beyond the current allowed kinds:
   - `hardened-xpriv`
   - `taproot`
   - `batch_claim_v1`
2. Whether a flatter append-only accumulator design becomes preferable once
   the hierarchy gets deeper or batches are produced incrementally over time
3. Whether the current one-shot wrapper should grow a faster path that skips
   top-level rebuild checks across repeated runs when the caller already
   knows the dependencies are current

## Leaf Schema Evolution

The current batch supports two leaf schemas: full Taproot and
hardened-xpriv. Open questions about the long-term disclosed leaf format:

- **hardened-xpriv**: most efficient (~2s/leaf), but discloses the child
  private key. Under the "EC spending is already broken" threat model this
  may be acceptable.
- **hardened-xpub**: discloses only the compressed public key. Better
  privacy, but costs more to prove (~14s/leaf due to EC point
  multiplication).
- **full Taproot**: strongest statement (final output key + path
  commitment), most expensive (~49s/leaf).
- **spend-bound leaf**: a future format that includes sighash binding or
  outpoint identifiers for direct spend authorization.

The aggregation mechanics are leaf-schema-agnostic. The `leaf_claim_kind`
field in the batch claim pins the schema, so different leaf types can be
supported without changing the batch guest or Merkle tree code.

## Accumulator Alternatives

The current design uses a flat binary Merkle tree. If incremental batching
requires a single flat root over all original leaves (not nested), the
current tree does not compose across separately-built chunks.

See `mmr-accumulator-sketch.md` for the shorter append-only accumulator sketch.

Alternatives for that requirement:

- Merkle Mountain Range (MMR) for append-only accumulation
- Merkle frontier / bag-of-peaks accumulator
- Power-of-two subtree discipline

Any of these would require a new batch claim format and new guest logic.
The recommendation is to validate hierarchical batching first, since it
works with the current tree and claim design.

## Poseidon2 Merkle Trees

The current tree uses SHA-256 for Bitcoin-adjacent verifier compatibility.
Poseidon2 would be cheaper inside the zkVM guest but requires external
verifiers to implement Poseidon2 for inclusion checking.

Revisit only if Merkle-root construction becomes a measurable proving
bottleneck at large N.

## Multi-UTXO Authorization

If the long-term goal is one proof covering many UTXOs for direct spend
authorization, the batch leaf should evolve from "derivation only" into
an authorization record that includes:

- a public key or spend-binding output
- a sighash binding (BIP-341 sighash digest)
- an outpoint or UTXO identifier

The aggregation mechanics can be validated before that semantic expansion.
See `claim.md` for the v2 single-leaf sighash-binding sketch.

## risc0 Composition Internals

For reference, the guest-side composition interface is:

- `env::verify(image_id, journal)` -- the standard path for verifying a
  prior receipt by image ID and journal content
- `env::verify_integrity(receipt_claim)` -- for working at the full claim
  level
- `env::verify_assumption(...)` / `env::verify_assumption2(...)` -- lower-
  level paths for advanced composition flows

The host-side recursion pipeline uses internal programs (`lift`, `join`,
`resolve`) to turn unresolved assumptions into an unconditional receipt.

Important constraint: `ExecutorEnv::add_assumption(...)` requires succinct
receipts. Composite receipts cannot be supplied as assumptions. This is why
the batch pipeline requires leaf receipts to be compressed to succinct form
before aggregation.
