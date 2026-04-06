# Claim

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
8a6a2c27dd54d8fa0f99a332b57cb105f88472d977c84bfac077cbe70907a690
```

Current reproducibility status:

- moving only the `bip32-pq-zkp` checkout path while keeping the same sibling
  toolchain trees did not change the image ID
- the older workspace-local `make platform` flow in `risc0/examples/c-guest`
  was the remaining source of checkout-path drift
- the recommended `make platform-standalone` flow now produces a matching
  platform archive and matching final guest artifact across different `risc0`
  checkout paths
