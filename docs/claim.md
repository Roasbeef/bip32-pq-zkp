# Claim

## Public Claim

The current proof statement is:

- there exists a private BIP-32 seed
- there exists a private derivation path
- deriving the child key for that witness inside the guest
- then applying the Taproot key-path tweak
- yields the exact 32-byte Taproot output key committed to the journal

Equivalently:

```text
∃ seed, path : TaprootKey(BIP32(seed, path)) == journal_output
```

## Public Material

- final Taproot output key only

That is the verifier-facing claim material.

## Private Witness

- seed
- derivation path

The current design keeps both private unless a higher-level application chooses
to reveal or separately commit path metadata.

## Optional Policy Layer

The proof logic supports an optional policy bit that requires the private path
to satisfy the BIP-86 shape:

```text
m / 86' / coin_type' / account' / change / index
```

That policy is intentionally optional.

Why:

- some uses only care about “this Taproot key came from private BIP-32 witness
  data”
- some uses specifically care about “this key came from a BIP-86 path”

The current implementation supports both.

## Current Known-Good Vector

For the built-in test vector, the current public output key is:

```text
00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6
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
b154913927df91257436ddb91567d46a28018c03bfb3848c3d7d7a774e840a79
```

Current reproducibility status:

- moving only the `bip32-pq-zkp` checkout path while keeping the same sibling
  toolchain trees did not change this image ID
- the older workspace-local `make platform` flow in `risc0/examples/c-guest`
  was the remaining source of checkout-path drift
- the recommended `make platform-standalone` flow now produces a matching
  platform archive and matching final guest artifact across different `risc0`
  checkout paths
