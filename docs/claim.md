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

The image ID should be treated as part of the exact built guest artifact, not
as a universal constant across all build environments.

Current sibling-layout image ID:

```text
e9177de911f48092749d50e17368e83a26207b016c3fe95a2efc49135e45c4eb
```

Current caveat:

- moving only the `bip32-pq-zkp` checkout path while keeping the same sibling
  toolchain trees did not change this image ID
- rebuilding the linked `libzkvm_platform.a` from a different `risc0` checkout
  path did change the image ID while preserving the same public output key

So the remaining checkout-path sensitivity is currently in the linked
platform-archive build.
