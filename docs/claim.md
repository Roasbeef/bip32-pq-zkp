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
as a universal constant across all build directories.

Current observed image IDs for that same public output are:

```text
sibling-layout build:
62b563ecceda688696ca9f9e2bb24c4b7e8987647a2d27a960e4d3376bd18082

fresh-clone build:
61a39aca30f96db015a56ea08b6fba8f0cfd43eca4d148c50afa1de60ecb26de
```

The output key stayed identical across those runs. The image ID changed because
the guest ELF currently embeds absolute source paths from the linked
`zkvm-platform` archive, making the artifact build-path-sensitive.
