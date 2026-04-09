# Reduced Variants

This note compares three related proof statements built from the same BIP-32
test vector and measured on the same local Apple Silicon proving lane.

## Statements

1. Full Taproot lane
   - private witness: BIP-32 seed plus full path
   - public claim: final BIP-86 Taproot output key plus path commitment
2. Hardened xpub lane
   - private witness: parent xpriv plus chain code plus one or more hardened
     child steps
   - public claim: child compressed xpub plus chain code
3. Hardened xpriv lane
   - private witness: parent xpriv plus chain code plus exactly one hardened
     child step
   - public claim: child xpriv bytes plus chain code

The reduced variants were motivated by the observation that the zk relation can
be weakened substantially while still being useful for later external wallet
derivation checks.

## Built-In Witness

The reduced variants use the same built-in BIP-32 seed as the full demo, but
they move the first hardened steps outside the guest:

- full lane: `m/86'/0'/0'/0/0`
- reduced parent xpriv: `m/86'/0'`
- reduced child step: `0'`

So the hardened-xpub and hardened-xpriv guests both derive the child at
`m/86'/0'/0'`, but they expose different public material.

## Measured Results

These results were measured with the direct host CLI path:

- host binary: `/tmp/bip32-pq-zkp-host`
- guest binaries:
  - `./bip32-platform-latest.bin`
  - `./hardened-xpub-platform-latest.bin`
  - `./hardened-xpriv-platform-latest.bin`
- host library override:
  - `GO_ZKVM_HOST_LIBRARY_PATH=/Users/roasbeef/gocode/src/github.com/roasbeef/go-zkvm/host-ffi/target/release/libgo_zkvm_host.dylib`

| Statement | Image ID | Receipt | Seal bytes | Prove time | Verify time | Peak RSS |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| Full Taproot | `8a6a2c27dd54d8fa0f99a332b57cb105f88472d977c84bfac077cbe70907a690` | composite | `1797880` | `49.32s` | `0.10s` | `11.91 GB` |
| Full Taproot | `8a6a2c27dd54d8fa0f99a332b57cb105f88472d977c84bfac077cbe70907a690` | succinct | `222668` | `64.30s` | `0.03s` | `11.93 GB` |
| Hardened xpub | `ad4ebc0ef6ce51e0f581cc8d14742a5b97738e9decd3fe2b0f1746de5bad9617` | composite | `513680` | `14.63s` | `0.04s` | `11.78 GB` |
| Hardened xpub | `ad4ebc0ef6ce51e0f581cc8d14742a5b97738e9decd3fe2b0f1746de5bad9617` | succinct | `222668` | `17.29s` | `0.02s` | `11.78 GB` |
| Hardened xpriv | `8401a36e4f54cb2beaf9ac7677603806cf9d775e90ef5a70168045a3c0df0849` | composite | `234568` | `1.98s` | `0.02s` | `3.14 GB` |
| Hardened xpriv | `8401a36e4f54cb2beaf9ac7677603806cf9d775e90ef5a70168045a3c0df0849` | succinct | `222668` | `2.84s` | `0.02s` | `3.15 GB` |

## Interpretation

The three lanes show three different tradeoffs:

- Full Taproot
  - strongest statement
  - highest proving cost
  - `succinct` materially shrinks the artifact from about `1.8 MB` to about
    `223 KB`
- Hardened xpub
  - middle-ground statement
  - still pays for one EC point multiplication
  - much cheaper than the full Taproot lane, but not dramatically smaller than
    the `succinct` floor
- Hardened xpriv
  - weakest statement
  - avoids guest-side EC point multiplication entirely
  - composite is already close to the `succinct` size floor, so recursion adds
    very little artifact-size benefit

The most important result is that once the guest statement becomes simple
enough, the final `succinct` receipt stops being the main optimization lever.
At that point, most of the remaining win comes from simplifying the statement
itself.

## Execute-Only Context

The execute-only runs already hinted at the same shape:

- hardened xpub
  - `2` segments
  - `1784821` rows
  - journal size `73` bytes
- hardened xpriv
  - `1` segment
  - `134166` rows
  - journal size `72` bytes

That aligns with the core intuition:

- hardened xpub still performs one point multiplication
- hardened xpriv performs only hardened CKDpriv plus scalar addition

## Current Conclusion

If the protocol can tolerate the weaker statement, the hardened-xpriv variant
is the best proving-cost reduction found so far. If the protocol needs public
xpub material for later derivation checks, the hardened-xpub variant is still a
useful middle ground. If the protocol needs the strongest direct claim about a
final Taproot output key, the original full lane remains the correct statement,
with `succinct` used to cap the final receipt size.
