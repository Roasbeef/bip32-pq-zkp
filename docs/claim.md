# Claim

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
