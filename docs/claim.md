# Claim

## Proof Statement

In plain language, the proof says:

> I know a BIP-32 seed and a derivation path such that, following BIP-32 child
> key derivation and BIP-86 Taproot output-key construction, the result is this
> specific 32-byte Taproot output key.

The seed and derivation path are private witness data that never appear in the
proof artifacts. The Taproot output key and a commitment to the derivation path
are public claim data embedded in the proof journal.

## Canonical Verifier Artifacts

The canonical verifier artifact set for this repo is:

- a serialized receipt file
- a `claim.json` file

The receipt is the actual proof artifact. `claim.json` is the stable,
human-readable description of the public statement that the receipt is meant to
prove.

The intended default verification flow is:

1. load the receipt
2. load `claim.json`
3. compute or pin the expected image ID for the exact guest artifact
4. verify the receipt against that image ID
5. compare the verified public journal output to `claim.json`

Direct verification against explicit `PUBKEY`, `PATH_COMMITMENT`, or
`BIP32_PATH` expectations is still supported, but it is the advanced/manual
path rather than the canonical verifier UX.

## Public Claim

The current proof statement is:

- there exists a private BIP-32 seed
- there exists a private derivation path
- deriving the child key for that witness inside the guest
- then applying the Taproot key-path tweak
- yields the exact public claim committed to the journal

Equivalently:

```text
∃ seed, path :
  PublicClaim(
    version,
    flags,
    TaprootKey(BIP32(seed, path)),
    CommitPath(path),
  ) == journal_output
```

## Public Material

- claim version
- claim flags
- final Taproot output key
- 32-byte path commitment

The journal is currently a fixed 72-byte record:

```text
offset  size  field
0       4     claim version (little endian)
4       4     claim flags (little endian)
8       32    Taproot output key
40      32    path commitment
```

The current flag layout is:

- bit `0`: require the private path to satisfy the BIP-86 shape

The path commitment is:

```text
SHA256("bip32-pq-zkp:path:v1" || len(path)_le32 || each_component_le32)
```

## Private Witness

- seed
- derivation path

The current design keeps both private. The verifier only learns the Taproot
output key, the path commitment, and the policy/version fields.

## claim.json Schema

The current `claim.json` artifact contains:

- `schema_version`
- `image_id`
- `claim_version`
- `claim_flags`
- `require_bip86`
- `taproot_output_key`
- `path_commitment`
- `journal_hex`
- `journal_size_bytes`
- `proof_seal_bytes`
- `receipt_encoding`

Field meaning:

- `schema_version`
  - version of the JSON envelope itself
- `image_id`
  - exact guest image ID the proof was generated against
- `claim_version`
  - version of the structured 72-byte journal format
- `claim_flags`
  - raw verifier-visible policy bitfield from the journal
- `require_bip86`
  - decoded convenience view of the current public BIP-86 policy bit
- `taproot_output_key`
  - final x-only Taproot output key as lowercase hex
- `path_commitment`
  - 32-byte commitment to the private path as lowercase hex
- `journal_hex`
  - raw committed public journal bytes as lowercase hex
- `journal_size_bytes`
  - byte length of the committed journal
- `proof_seal_bytes`
  - measured proof seal size in bytes
- `receipt_encoding`
  - serialization encoding of the receipt artifact

## v1 Compatibility Guarantees

For `schema_version = 1` and `claim_version = 1`, the intended compatibility
contract is:

- stable:
  - the `claim.json` field names listed above
  - the meaning of those fields
  - the 72-byte public journal layout
  - `claim_flags` bit `0` meaning "require BIP-86 path shape"
  - `taproot_output_key` as the final x-only output key
  - `path_commitment` as the SHA-256 commitment over the private path encoding
  - `journal_hex` being the raw bytes of the committed public claim
- stable for the current documented lane:
  - `receipt_encoding = "borsh"`
- informational only:
  - `proof_seal_bytes`
  - proof wall-clock times published in the docs

Image ID expectations:

- the image ID is artifact-specific and should be treated as part of the exact
  built guest
- the image ID is expected to change when any of these change:
  - guest code
  - packaged kernel ELF
  - platform archive
  - TinyGo toolchain / linker output
  - any other input that changes the final guest artifact bytes
- the image ID is not expected to change when only the private witness changes

Version-bump expectations:

- bump `schema_version` if the `claim.json` field names or meanings change
- bump `claim_version` if the structured 72-byte journal layout or public claim
  semantics change
- adding new optional JSON fields can remain backward-compatible if existing
  fields keep the same names and meanings

## Optional Policy Layer

The proof logic supports an optional public policy bit that requires the
private path to satisfy the BIP-86 shape:

```text
m / 86' / coin_type' / account' / change / index
```

That policy is intentionally optional.

Why:

- some uses only care about "this Taproot key came from private BIP-32 witness
  data"
- some uses specifically care about "this key came from a BIP-86 path"

The current implementation supports both.

Current demo policy:

- the documented `bip32-pq-zkp` demo lane defaults to `require_bip86 = true`
- callers can still opt out explicitly for non-BIP-86 derivations
- this remains a single guest image
- the BIP-86 requirement is carried as a verifier-visible public claim flag,
  not as a separate image identity

## Future Claim v2 Sketch

The current `claim_version = 1` proves key derivation only. It does not yet
bind that derived Taproot key to an actual spend authorization.

If the next step is a spend-authorizing proof, then the important public binding
target is the BIP-341 sighash digest, not just a `txid` or `wtxid`.

Why:

- `txid` does not include witness data and is not itself the Taproot
  authorization message
- `wtxid` includes witness data, which makes it awkward as the primary thing to
  sign or prove over
- Taproot key-path authorization is over the BIP-341 sighash message, which
  also depends on prevout amounts, scriptPubKeys, input index, and sighash type

So the natural `claim v2` direction is:

- keep:
  - `taproot_output_key`
  - `path_commitment`
- add:
  - `sighash_digest`
  - `sighash_type`
  - `input_index`
- optionally add:
  - the actual BIP-340 Schnorr signature
  - `txid`
  - `wtxid`

In that model, the proof statement becomes:

```text
∃ seed, path :
  TaprootKey(BIP32(seed, path)) = taproot_output_key
  CommitPath(path) = path_commitment
  SignTaproot(sighash_digest) = signature
```

where `sighash_digest` is the exact BIP-341 authorization digest for the spend
being proven.

There are three natural implementation levels:

- minimal:
  - the host computes the BIP-341 sighash externally
  - the guest proves derivation, tweak, and signature over that digest
- better:
  - same as above, but the public claim also carries the actual Schnorr
    signature
- strongest:
  - the guest computes the BIP-341 sighash internally from the transaction
    template, prevouts, amounts, scriptPubKeys, input index, and sighash flag
  - then proves derivation, tweak, sighash construction, and signature together

In that future design, `txid` and `wtxid` are still useful verifier-facing
metadata in `claim.json`, but they are not sufficient by themselves as the core
authorization binding.

## Future Recursive Composition Sketch

If the demo ever grows past a single guest into a modular proof pipeline, then
the natural risc0 mechanism is proof composition via assumptions, not "run the
verifier inside the guest".

At a high level, that would look like:

- proof A:
  - private witness:
    - seed
    - derivation path
  - public output:
    - internal key
    - path commitment
- proof B:
  - private witness:
    - whatever additional material is needed for the tweak or spend step
  - public output:
    - Taproot output key
    - optional sighash digest
    - optional Schnorr signature
  - assumption:
    - proof A's claim about the derived internal key
- proof C:
  - public output:
    - the final verifier-facing spend claim
  - assumptions:
    - proof A
    - proof B

In risc0 terms, the guest-side hooks for that are `sys_verify_integrity` and
`sys_verify_integrity2`, which add unresolved assumptions that later recursive
proof steps can resolve.

The cleanest decomposition would probably be:

- step 1:
  - prove BIP-32 derivation from private seed/path to the internal key
- step 2:
  - prove BIP-86 tweak from the internal key to the Taproot output key and,
    later, Taproot spend authorization over a specific BIP-341 sighash
- step 3:
  - resolve those assumptions into one final verifier-facing claim

Potential benefits:

- smaller individual guest programs
- lower per-proof memory pressure for each subclaim
- better modularity and reusability of subproofs
- a path toward one final recursively compressed receipt

Important caveat:

- this does not mean the first composed version will automatically be smaller
  or faster than the current single-guest proof
- for a claim as small as the current BIP-32 to BIP-86 demo, recursive
  composition would likely increase total proving work at first
- the size benefit only really appears once the system is actually resolving
  and recursively compressing multiple assumptions into a final receipt, or
  when the modularity/reuse value outweighs the recursion overhead

So recursion is a plausible future architecture if we split the demo into
multiple claims, but it is not the obvious next optimization for the current
one-shot proof.

There is one setting where composition becomes much more compelling: proving a
whole set of derivation claims and publishing one final aggregated receipt.

That would look like:

- leaf proof `i`:
  - private witness:
    - `seed_i`
    - `path_i`
  - public output:
    - `taproot_output_key_i`
    - optional `path_commitment_i`
- aggregation proof:
  - assumptions:
    - the set of leaf proofs
  - public output:
    - either the full list of output keys
    - or a Merkle root / commitment to the set of claims
    - or a higher-level statement such as "these `N` outputs were all derived
      correctly"

In that model, each leaf proof keeps its own private witness local. The final
aggregation proof only needs the public outputs or commitments from those leaf
claims. That means the seed and path material can remain secret even while the
system publishes a single verifier-facing receipt for the whole set.

This is the more realistic place where recursive composition may help with size:

- not by making one small proof smaller
- but by replacing `N` separate published receipts with one final aggregated
  receipt
- and by letting the final public claim expose either the full set of `N`
  derived keys or just a commitment to that set

The tradeoff remains the same:

- total proving work usually increases
- the verifier-facing artifact size and verification UX can improve
- the best fit is many related claims, not one already-small derivation proof

## Current Known-Good Vector

For the built-in test vector, the current public claim material is:

```text
claim_version = 1
claim_flags = 1
output_key = 00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6
path_commitment = 4c7de33d397de2c231e7c2a7f53e5b581ee3c20073ea79ee4afaab56de11f74b
```

Latest measured local proof size for that vector on this Mac:

```text
1797880 bytes
```

The image ID is still part of the exact built guest artifact, but the
recommended standalone-archive flow now reproduces that artifact across
different `risc0` checkout paths.

Current deterministic image ID:

```text
b823d67c3ec46ce8434369dcce609fae92dd0c826ec2781ff7cccb6d91793d23
```

Current reproducibility status:

- moving only the `bip32-pq-zkp` checkout path while keeping the same sibling
  toolchain trees did not change the image ID
- the older workspace-local `make platform` flow in `risc0/examples/c-guest`
  was the remaining source of checkout-path drift
- the recommended `make platform-standalone` flow now produces a matching
  platform archive and matching final guest artifact across different `risc0`
  checkout paths
