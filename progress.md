# Progress

Date: 2026-04-05

## 2026-04-05 Go Host E2E

The demo now runs end to end through the new Go host boundary instead of the
old demo-local Rust CLI.

Cleanup status on top of that host migration:
- the stale Rust `host/` crate has been removed from the repo
- `cmd/bip32-pq-zkp-host/` is now a thin CLI layer
- reusable claim/witness/run logic now lives in normal Go package files at the
  module root
- the repo now carries local style tooling:
  - `make fmt`
  - `make fmt-check`
  - `make tidy`
  - `make tidy-check`
  - `make lint-native`
  - `make lint`
  - `make native-check`
- those native host-side checks were run successfully after the refactor

Current host shape:
- `cmd/bip32-pq-zkp-host/`
  - demo-specific Go command
- `../go-zkvm/host`
  - typed Go host API
- `../go-zkvm/host-ffi`
  - Rust `cdylib`
- `../go-zkvm/host-core`
  - shared Rust host logic
- `../go-zkvm/go-guest-host`
  - Rust reference CLI kept for now, but no longer the primary demo surface

Current validated result through that path:
- `go build ./cmd/bip32-pq-zkp-host`
  - passed
- `go test ./hostcheck -v`
  - passed
- `make prove GO_GOROOT=/Users/roasbeef/sdk/go1.24.4 PRIV_SEED_HEX=000102030405060708090a0b0c0d0e0f BIP32_PATH="86',0',0',0,0" REQUIRE_BIP86=1`
  - passed
- `make verify GO_GOROOT=/Users/roasbeef/sdk/go1.24.4 BIP32_PATH="86',0',0',0,0" REQUIRE_BIP86=1 PUBKEY=00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
  - passed

Current runtime notes:
- bare `make prove` still uses the built-in BIP-32 test vector
- bare `make verify` still uses the default receipt and claim artifact paths
- the live proving process was sampled with `vmmap` and showed:
  - `libgo_zkvm_host.dylib`
  - `Metal.framework`
  - `MetalPerformanceShaders.framework`
- so the new Go-host path is still on the Metal-enabled local proving lane
- latest explicit-witness wall-clock through the Go host wrapper:
  - `54.76s`

## Current Extracted Demo Status

This file started as the prototype log from the monorepo-style workspace. It is
now also tracking the extracted `bip32-pq-zkp` demo state.

Current claim shape:
- public material:
  - claim version
  - claim flags
  - final Taproot output key
  - path commitment
- private witness:
  - seed
  - derivation path
- optional policy:
  - require the path to satisfy BIP-86, exposed as a public claim flag

Current prover / verifier flow:
- `make execute`
  - runs the guest with private witness input and prints the public claim
- `make prove`
  - builds the guest, proves it locally, and writes:
    - `artifacts/bip32-test-vector.receipt`
    - `artifacts/bip32-test-vector.claim.json`
- `make verify`
  - verifies the receipt against the guest image ID
  - and then verifies either:
    - the emitted claim JSON
    - or direct public expectations such as `PUBKEY`, `BIP32_PATH`, and
      `REQUIRE_BIP86`

Verifier-artifact policy update:
- the canonical verifier artifact set is now:
  - the binary receipt
  - the emitted `claim.json`
- direct `PUBKEY`, `PATH_COMMITMENT`, and `BIP32_PATH` checks remain supported,
  but are now treated as the advanced/manual path
- the v1 compatibility guarantees are now written down in:
  - `docs/claim.md`
  - with `proof_seal_bytes` explicitly treated as informative metadata rather
    than a stability guarantee

Current known-good built-in vector:
- final Taproot output key:
  - `00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
- path commitment:
  - `4c7de33d397de2c231e7c2a7f53e5b581ee3c20073ea79ee4afaab56de11f74b`
- current journal size:
  - `72` bytes
- current deterministic image ID:
  - `8a6a2c27dd54d8fa0f99a332b57cb105f88472d977c84bfac077cbe70907a690`
- current artifact caveat:
  - moving only the `bip32-pq-zkp` checkout path while reusing the same sibling
    `risc0`, `tinygo-zkvm`, and `go-zkvm` trees kept the image ID stable
  - the older workspace-local `make platform` flow in `risc0/examples/c-guest`
    changed the image ID when the `risc0` checkout path changed
  - the published `make platform-standalone` flow now produces a matching
    platform archive and matching final guest artifact across different `risc0`
    checkout paths

Current measured local prove+verify result on this Mac:
- split claim-artifact `make prove` run with explicit witness:
  - command:
    - `/usr/bin/time -lp make prove GO_GOROOT=/Users/roasbeef/sdk/go1.24.4 PRIV_SEED_HEX=000102030405060708090a0b0c0d0e0f BIP32_PATH="86',0',0',0,0"`
  - image ID:
    - `8a6a2c27dd54d8fa0f99a332b57cb105f88472d977c84bfac077cbe70907a690`
  - proof seal size:
    - `1797880` bytes
  - wall-clock:
    - `54.38s`
  - peak resident set size:
    - about `11.9 GB`
- split `make verify` run with explicit public expectations:
  - command:
    - `make verify GO_GOROOT=/Users/roasbeef/sdk/go1.24.4 CLAIM= PUBKEY=00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6 BIP32_PATH="86',0',0',0,0"`
  - result:
    - verified the receipt directly against:
      - guest image ID
      - expected Taproot output key
      - expected path commitment recomputed from the supplied path
      - expected `require_bip86=true` policy bit
- clean-room deterministic rerun:
  - path:
    - `/tmp/roasbeef-deterministic-repro.AUHw8G`
  - image ID:
    - `b154913927df91257436ddb91567d46a28018c03bfb3848c3d7d7a774e840a79`
  - proof seal size:
    - `1797880` bytes
  - wall-clock:
    - `58.93s`
  - fresh-clone prerequisites discovered during that rerun:
    - `tinygo-zkvm`: `git submodule update --init --recursive`
    - `risc0`: `git lfs pull`
- deterministic standalone-archive run:
  - command:
    - `/usr/bin/time -lp cargo run --release -- /tmp/bip32-standalone-local.bin --raw-journal --use-test-vector --require-bip86`
  - image ID:
    - `b154913927df91257436ddb91567d46a28018c03bfb3848c3d7d7a774e840a79`
  - proof seal size:
    - `1797880` bytes
  - wall-clock:
    - `51.51s`
- latest sibling-layout rerun:
  - command:
    - `/usr/bin/time -lp make prove GO_GOROOT=/Users/roasbeef/sdk/go1.24.4`
  - image ID:
    - `e9177de911f48092749d50e17368e83a26207b016c3fe95a2efc49135e45c4eb`
  - proof seal size:
    - `1797880` bytes
  - wall-clock:
    - `54.88s`
- sibling-layout run:
  - command:
    - `cargo run --release -- ../../bip32-pq-zkp/bip32-platform-latest.bin --raw-journal --use-test-vector --require-bip86`
  - wall-clock:
    - `65.24s`
- clean-room fresh-clone run:
  - command:
    - `/usr/bin/time -lp make prove GO_GOROOT=/Users/roasbeef/sdk/go1.24.4`
  - wall-clock:
    - `85.65s`
- peak resident set size:
  - about `11.9 GB`
- operator-side GPU confirmation:
  - `asitop` showed about `40%` GPU usage at roughly `338 MHz` during the
    fresh-clone proof
- current host build confirmation:
  - `cargo tree` for `host/` includes the `metal` crate via `risc0-zkp`
  - `otool -L host/target/release/bip32-pq-zkp-host` shows
    `Metal.framework`
  - important nuance:
    - Metal is for local proving
    - TinyGo guest compilation itself remains CPU work

Targeted reproducibility experiments after the repo split:
- changing only the `bip32-pq-zkp` checkout path:
  - changed the raw ELF and packed `.bin` hashes
  - did not change the computed image ID
- rebuilding only `risc0/examples/c-guest` from a different checkout path:
  - changed the `libzkvm_platform.a` hash
  - changed the final guest image ID to:
    - `99c5fd320c1427e704eea22c84e9bd6ae2d0f6aa27ffb2a19b379189a4fba249`
  - kept the public Taproot output key identical
- adding explicit path remaps to the platform build:
  - normalized visible `/risc0-src/...` and `/cargo/registry/src/...` strings
  - but did not yet make cross-checkout platform archives produce the same
    guest image ID
- standalone archive builder milestone:
  - `roasbeef/risc0` now publishes `examples/c-guest make platform-standalone`
  - that target pins the platform crate dependencies to the published git
    commit instead of local workspace path dependencies
  - across two different `risc0` checkout paths it produced the same platform
    archive hash:
    - `925833d290f462302d9fd72e9cd37569c52f49a91a46ff4cc18e1405468aab08`
  - rebuilding the real `bip32` guest against those two standalone archives
    produced matching guest artifact hashes:
    - ELF: `42c672f595643cea00573de14614df7b7e78122bdca8702bae790f1b89baefee`
    - BIN: `4d8a41a78a726941e469bfad5ad77524a20f39b5c02313fa80351205857ded9c`

Current repo-split direction:
- `roasbeef/risc0`
- `roasbeef/tinygo-zkvm`
- `roasbeef/go-zkvm`
- `roasbeef/bip32-pq-zkp`

Later remote-proving note:
- Boundless repo:
  - `https://github.com/boundless-xyz/boundless`
- kept for later investigation only; the current validated path is local proving

Latest publication-quality verification:
- cloned the private repos fresh under:
  - `/tmp/roasbeef-publish-check.xtVcmy`
- verified from those fresh clones:
  - `tinygo-zkvm` required `make llvm-source` before the documented external
    LLVM build command
  - `risc0/examples/c-guest` rebuilt `libzkvm_platform.a`
  - `go-zkvm` rebuilt `simple.bin` and executed it successfully
  - `bip32-pq-zkp` host reference tests passed
  - `bip32-pq-zkp` proved and verified successfully, producing the same public
    Taproot output key as the sibling-layout build

## Goal

Primary goal:
- compile a TinyGo guest for the RISC Zero zkVM
- prove it with the existing risc0 host/prover flow
- verify the receipt with the normal risc0 toolchain

End goal:
- generate a proof that a Taproot output public key was derived from a BIP-32 seed/path, without revealing the seed

## Current Repo State

Top-level relevant directories:
- `risc0/`: upstream risc0 repo
- `tinygo/`: risc0 TinyGo fork / local TinyGo work
- `go-zkvm-tests/`: Go guest samples, custom guest library, packaging helpers
- `go-guest-host/`: small Rust host harness

Local working state observations:
- `risc0/` is on `main` at `e0ff1fc26`, but live `origin/main` is `fee66d04f527c8c04678a355ab916bbb837634bd` as of 2026-04-03.
- `tinygo/` is on local branch `zkvm-support` at `ab8a8900`.
- `tinygo` remote `origin` points at `risc0/tinygo`, whose HEAD is currently `release` at `b72dc818`.
- `substrate` CLI is available as an async backchannel to the user; it can be used to send periodic status mail and poll for replies without blocking the main coding flow.
- registered substrate agent for this work:
  - `risc0-zkp-codex`
- practical note:
  - explicit `--agent risc0-zkp-codex` works for `status`, `inbox`, `poll`, and `status-update`
  - project-default/session binding is a little inconsistent in this shell, so explicit `--agent` is the safest way to use it here

## Latest-Upstream Lane

To avoid disturbing the old dirty checkouts, a separate "latest deps" lane now exists in fresh git worktrees:
- latest risc0 worktree:
  - path: `/Users/roasbeef/gocode/src/github.com/risc0/risc0-main-latest`
  - branch: `codex/latest-origin-main-20260403`
  - head: `fee66d04f527`
- latest TinyGo zkVM worktree:
  - path: `/Users/roasbeef/gocode/src/github.com/risc0/tinygo-v0.40.1-zkvm`
  - branch: `codex/zkvm-v0.40.1`
  - head: `09cce54238af`
  - base upstream TinyGo release: `db9f1182f5f2` (`v0.40.1`)

Important implications:
- the original `risc0/` and `tinygo/` directories remain intact as the known-working/manual-path reference
- new work should prefer the fresh worktrees above when the goal is "latest upstream deps"
- this avoids `stash` / `stash pop` across dirty repos and submodules

What has already been verified in the latest-upstream lane:
- `risc0-main-latest` still contains `examples/c-guest/`
- `risc0-main-latest/risc0/cargo-risczero/src/commands/guest.rs` still unpacks `risc0-zkvm-platform.a`
- `risc0-main-latest/semver-baselines.lock` now shows `risc0-zkvm-platform = "2.2.2"`
- `tinygo-v0.40.1-zkvm` was created from upstream TinyGo `v0.40.1`
- the local zkVM patch stack from `zkvm-support` was replayed onto it:
  - `zkvm start`
  - `e2e workign impl`
  - `start of sha`
  - `sha2 syscall`
  - `test commit`
  - `codex bug fix`
- the uncommitted linker-script change that matched the current risc0 guest memory map was also reapplied as:
  - `zkvm: update linker layout for current risc0 memory map`
- submodules in the fresh TinyGo worktree now initialize cleanly, unlike the odd dirty state in the older checkout
- `llvm-project` source was fetched in the fresh TinyGo worktree via `make llvm-source`
- the local machine already had Homebrew LLVM and LLD installed:
  - `/opt/homebrew/opt/llvm`
  - `/opt/homebrew/lib/liblld*.dylib`
- Homebrew's `llvm` formula is actually LLVM `20.1.8` here
- TinyGo `v0.40.1` builds successfully against that external LLVM install by using:
  - `LLVM_BUILDDIR=/opt/homebrew/opt/llvm`
  - `CGO_LDFLAGS_EXTRA='-L/opt/homebrew/lib'`
  - a small local `GNUmakefile` hook that allows appending extra clang component libraries for external LLVM layouts
  - `CLANG_EXTRA_LIB_NAMES='clangARCMigrate clangStaticAnalyzerCore clangStaticAnalyzerFrontend clangStaticAnalyzerCheckers'`
- validated result:
  - `/Users/roasbeef/gocode/src/github.com/risc0/tinygo-v0.40.1-zkvm/build/tinygo version`
  - reports `tinygo version 0.40.1 darwin/arm64 (using go version go1.26.1 and LLVM version 20.1.8)`

Latest-upstream lane validation now completed:
- with `GOROOT=/Users/roasbeef/sdk/go1.24.4`, the rebased TinyGo binary compiles zkVM guests successfully
- a simple guest built with the new TinyGo tree:
  - `/tmp/simple-v040.elf`
  - packaged to `/tmp/simple-v040.bin`
  - prints `Hello from Go zkVM!` under `r0vm`
- the real `bip32` guest also built successfully with the new TinyGo tree:
  - `/tmp/bip32-v040.elf`
  - packaged to `/tmp/bip32-v040.bin`
- execute-only validation using the existing Rust harness produced the exact known-good journal:
  - raw journal hex: `8724f200544f593846c3a868faa13cfc47fb29842e010758c0b95e6f79896434`
  - exit code: `Halted(0)`
  - segments: `11`
  - cycles: `10477314`
- this matches the earlier working manual-path result, so the TinyGo upgrade itself did not change guest semantics for the current `bip32` program

Practical next step for the latest-upstream lane:
- stop treating TinyGo/LLVM version skew as the main issue
- move on to replacing the handwritten guest runtime/syscall path with the newer `risc0-zkvm-platform.a` / `examples/c-guest` model

Latest-upstream lane findings after the first real migration push:
- direct TinyGo linking against upstream `libzkvm_platform.a` is now working for a real Go guest, not just a smoke test
- the `bip32` guest builds with `-target=zkvm-platform` and links the upstream archive successfully
- on the old harness, that archive-linked `bip32` guest already matched the manual path journal exactly:
  - raw journal hex: `8724f200544f593846c3a868faa13cfc47fb29842e010758c0b95e6f79896434`
  - but that result still mixed "latest guest bits" with an older host / kernel lane

The next set of fixes were required to make the *whole* lane consistent with latest upstream risc0:
- the installed `r0vm` in this shell is only `risc0-r0vm 4.0.0`
- that meant a host linked to `risc0-main-latest` but using `default_prover()` without `prove` was silently delegating to an older external prover binary
- `go-guest-host` has now been repointed to `risc0-main-latest/risc0/zkvm` with:
  - `default-features = false`
  - `features = ["prove"]`
- this forces the in-process latest prover path and avoids the external `r0vm 4.0.0` skew entirely

ABI/runtime fixes discovered during that migration:
- the latest local executor/prover immediately exposed a stale ABI assumption in our TinyGo guest/runtime:
  - current upstream `sys_cycle_count` returns `u64`
  - our TinyGo guest/runtime still declared it as `uint32`
- fixed locally in:
  - `go-zkvm-tests/zkvm/zkvm.go`
  - `tinygo-v0.40.1-zkvm/src/runtime/runtime_zkvm.go`
  - `tinygo-v0.40.1-zkvm/src/zkvm/zkvm.go`
- after that, the next failure showed the packaging was still wrong:
  - we were packing the guest ELF with the older top-level `go-zkvm-tests/kernels/v1compat.elf`
  - current latest risc0 needs its own kernel half from:
    - `risc0-main-latest/risc0/zkos/v1compat/elfs/v1compat.elf`

Current latest-consistent execution milestone:
- guest ELF:
  - `/tmp/bip32-platform.elf`
- latest-compatible packed binary:
  - `/tmp/bip32-platform-latest.bin`
- built from:
  - latest TinyGo `v0.40.1`
  - upstream `libzkvm_platform.a`
  - latest risc0 `v1compat.elf` from `risc0-main-latest`
- executed under:
  - `go-guest-host`
  - linked to `risc0-main-latest`
  - using the in-process `local` executor/prover path
- result:
  - image ID: `7e4bcecacf04b5a9cc053cc7b3f5938f34eaf534418484425a0cba34eb23d269`
  - raw journal hex: `8724f200544f593846c3a868faa13cfc47fb29842e010758c0b95e6f79896434`
  - exit code: `Halted(0)`
  - segments: `6`
  - rows: `5263756`
- this is materially better than the original handwritten/manual lane and is the first confirmed run that is consistent across:
  - latest TinyGo branch
  - latest risc0 Rust host libs
  - latest packaged kernel half
  - upstream platform archive instead of handwritten syscall assembly

Metal / Apple Silicon proving note:
- sampling the in-process local prover on macOS shows it entering:
  - `risc0_circuit_rv32im::prove::ProverContext::new_metal`
  - `risc0::getGpuHal()`
  - Metal pipeline creation APIs such as `newComputePipelineStateWithFunction`
- so the latest local prover on this machine is in fact taking the Metal-backed GPU path
- current upstream risc0 on Apple Silicon enables this path by default; there was not a missing Metal feature flag to turn on
- the first run is paying shader / pipeline initialization cost, so initial proving latency is much higher than execute-only latency
- later release-profile confirmation on the real `bip32` guest showed the live stack in:
  - `risc0_circuit_rv32im_prover_new_gpu`
  - `risc0::getGpuHal()`
  - `MetalHal::MetalHal()`
  - `-[_MTLDevice newComputePipelineStateWithFunction:error:]`
  - `AGXG16XFamilyDevice newComputePipelineStateWithDescriptor:error:`
- that is direct evidence that the release `bip32` run is using Metal rather than silently falling back to CPU
- for local debugging, a temporary escape hatch was added in the latest worktree:
  - `RISC0_FORCE_CPU_PROVER=1`
  - this forces both the segment prover and recursion prover onto CPU so progress is not blocked on first-run Metal warmup
- sampling the CPU-forced prover shows:
  - the prover is active in `CpuHal::hashRows`
  - it uses `risc0::parallel_map(...)` worker threads, so the CPU prover path is multi-threaded
  - resident memory during this phase is already about `10.7 GiB` for the current `bip32` example

Current status at time of writing:
- the fully latest-consistent local prove+verify of `/tmp/bip32-platform-latest.bin` now completes successfully
- execution and receipt verification are both validated end-to-end on the new lane
- the latest-upstream lane milestone is now real:
  - TinyGo guest -> upstream platform archive -> latest local prove -> receipt verify

Repeatability improvement:
- `go-zkvm-tests/Makefile` now has explicit latest-lane targets:
  - `simple-platform-latest`
  - `bip32-platform-latest`
- those targets codify the exact working recipe:
  - use the rebased TinyGo binary from `tinygo-v0.40.1-zkvm`
  - force Go `1.24.4` via `PATH` and `GOROOT`
  - build with `-target=zkvm-platform`
  - link by passing the absolute `libzkvm_platform.a` path as:
    - `-ldflags='-extldflags=/abs/path/to/libzkvm_platform.a'`
  - package with the latest `risc0-main-latest/risc0/zkos/v1compat/elfs/v1compat.elf`
- important TinyGo quirk:
  - passing the platform archive as a single `-extldflags=/abs/path/to/libzkvm_platform.a` value works reliably
  - trying to split it into separate `-L...` / `-lzkvm_platform` linker flags did not survive TinyGo's `-ldflags` handling cleanly in this setup

Small control guest on the same latest lane:
- `make -C go-zkvm-tests simple-platform-latest` now works and emits:
  - `go-zkvm-tests/simple-platform-latest.elf`
  - `go-zkvm-tests/simple-platform-latest.bin`
- execute-only validation via `go-guest-host` succeeds with:
  - image ID: `11f39978f6219f5d9db053f5d3e41e7ce1587af6b634f528f05b82e54ab07d31`
  - guest output: `Hello from Go zkVM!`
  - empty journal
  - segments: `1`
  - rows: `42488`
- this is useful as a control case while the much heavier `bip32` proof is still in flight

Host-side correctness check for the current `bip32` vector:
- added a normal Go test at:
  - `go-zkvm-tests/hostcheck/bip32_vector_test.go`
- validated with:
  - `PATH=/Users/roasbeef/sdk/go1.24.4/bin:$PATH GOROOT=/Users/roasbeef/sdk/go1.24.4 go test ./hostcheck`
- result:
  - pass
- what it proves:
  - the hardcoded seed/path currently embedded in `go-zkvm-tests/bip32/main.go`
  - derives the exact x-only public key bytes:
    - `8724f200544f593846c3a868faa13cfc47fb29842e010758c0b95e6f79896434`
  - which matches the guest journal observed under both the old manual lane and the new latest/archive-linked lane

Important proving-path correction:
- the first long-running local proofs in this pass were launched with plain `cargo run`, which means the host/prover stack was in Cargo's debug/dev profile
- for any meaningful local proving work, the right command is:
  - `cargo run --release -- ...`
- after switching to release mode, the small latest-lane control proof completed successfully on the first try:
  - command:
    - `RISC0_FORCE_CPU_PROVER=1 cargo run --release -- /tmp/simple-platform-latest.bin --raw-journal`
  - result:
    - receipt verification passed
    - journal size: `0`
    - raw journal hex: empty
    - image ID: `11f39978f6219f5d9db053f5d3e41e7ce1587af6b634f528f05b82e54ab07d31`
- conclusion:
  - the latest local prove+verify path is now confirmed end-to-end for an archive-linked Go guest
  - the remaining open proving item is the heavier `bip32` guest on the same release-profile path

Latest `bip32` prove+verify result on the latest lane:
- command:
  - `cargo run --release -- /tmp/bip32-platform-latest.bin --raw-journal`
- this run used the default Apple Silicon Metal path, not the CPU override
- result:
  - receipt verification passed
  - raw journal hex: `8724f200544f593846c3a868faa13cfc47fb29842e010758c0b95e6f79896434`
  - journal size: `32` bytes
  - image ID: `7e4bcecacf04b5a9cc053cc7b3f5938f34eaf534418484425a0cba34eb23d269`
- significance:
  - this is the first confirmed end-to-end proof on the fully updated lane for the real `bip32` guest
  - it uses:
    - latest TinyGo `v0.40.1`
    - upstream `libzkvm_platform.a`
    - latest `v1compat.elf`
    - latest Rust host/prover libs
  - local prove + receipt verify
  - Apple Silicon Metal GPU acceleration

Private-input milestone on the same latest lane:
- the `bip32` guest no longer hardcodes the seed or derivation path
- `go-guest-host` now acts as the normal host-side launcher for this proof flow:
  - it loads the packaged guest binary
  - computes the guest image ID
  - builds `ExecutorEnv`
  - writes the private seed and derivation path into zkVM stdin
  - executes or proves the guest
  - verifies the receipt against the image ID
- `go-zkvm-tests/bip32/main.go` now reads:
  - `u32 seed_len`
  - `seed_len` raw seed bytes
  - `u32 path_len`
  - `path_len` raw `u32` path components
- the guest validates those private inputs, derives the BIP-32 key internally, and commits only the public x-only key bytes
- rebuilt latest-lane artifact:
  - `go-zkvm-tests/bip32-platform-latest.bin`
  - new image ID: `aa3804809bad0666b9a1887b47c1d2a9676c18382a813bc3b4463a562ccd5c4e`
- execute-only validation with host-supplied private defaults:
  - command:
    - `cargo run --release -- ../go-zkvm-tests/bip32-platform-latest.bin --raw-journal --execute-only`
  - result:
    - raw journal hex: `8724f200544f593846c3a868faa13cfc47fb29842e010758c0b95e6f79896434`
    - exit code: `Halted(0)`
    - segments: `6`
    - rows: `5265276`
- prove+verify validation with the same private defaults:
  - command:
    - `cargo run --release -- ../go-zkvm-tests/bip32-platform-latest.bin --raw-journal`
  - result:
    - receipt verification passed
    - raw journal hex: `8724f200544f593846c3a868faa13cfc47fb29842e010758c0b95e6f79896434`
    - journal size: `32` bytes
- significance:
  - the secret BIP-32 material is now host-supplied private input rather than guest-embedded constants
  - at this intermediate step, the public claim was still the x-only internal key journal
  - this established the correct host/guest shape for the next Taproot-output-key proof step

BIP-86 Taproot-output milestone:
- the `bip32` guest now derives the final BIP-86 Taproot output key instead of committing the intermediate x-only internal key
- public journal material is now exactly one 32-byte Taproot output public key
- the implementation intentionally mirrors the `btcd/txscript` logic in:
  - `ComputeTaprootOutputKey`
  - `ComputeTaprootKeyNoScript`
- for the current fixture seed/path (`m/86'/0'/0'/0/0`), the committed public key is:
  - `00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
- rebuilt latest-lane artifact:
  - `go-zkvm-tests/bip32-platform-latest.bin`
  - new image ID: `b5e182fdb7a57c02bd039111583453f2c1868c28c0c6caac0eca52540ae88516`
- execute-only validation:
  - command:
    - `cargo run --release -- ../go-zkvm-tests/bip32-platform-latest.bin --raw-journal --execute-only`
  - result:
    - raw journal hex: `00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
    - exit code: `Halted(0)`
    - segments: `7`
    - rows: `7079371`
- full prove+verify validation:
  - command:
    - `cargo run --release -- ../go-zkvm-tests/bip32-platform-latest.bin --raw-journal`
  - result:
    - receipt verification passed
    - raw journal hex: `00324bf6fa47a8d70cb5519957dd54a02b385c0ead8e4f92f9f07f992b288ee6`
    - journal size: `32` bytes
- host-side correctness validation:
  - `go-zkvm-tests/hostcheck/bip32_vector_test.go` now:
    - keeps the old internal x-only check as an intermediate sanity test
    - checks the final BIP-86 output key against `btcd/txscript.ComputeTaprootKeyNoScript`
    - pins the known-good output key hex above
- significance:
  - the public claim is now the actual Taproot output key
  - the seed and derivation path remain private host input
  - this is the first local end-to-end proof in the repo with the intended BIP-32-to-BIP-86 public-output shape

## Latest Working Result

The highest-signal change in this pass is that the TinyGo `bip32` guest now proves the final BIP-86 Taproot output key end-to-end with the seed kept as private host input.

Confirmed locally:
- TinyGo 0.39.0 here must be run with Go 1.24.4 (`GOROOT=/Users/roasbeef/sdk/go1.24.4`), otherwise builds fail due toolchain-version mismatch.
- baseline TinyGo guest execution still works for simple samples under that pinned toolchain.
- a new local module exists at `github.com/roasbeef/btcd-zkp` and is consumed from `go-zkvm-tests` via a local `replace`.
- that module currently contains a minimal BIP-32 private-derivation helper instead of depending on `btcutil/hdkeychain`.
- the previous `btcec/v2/schnorr` dependency was the main TinyGo blocker for this guest shape:
  - it dragged in `chainhash`, `fmt`, `sync.Pool`, and a much heavier package-init path
  - TinyGo guests that imported it faulted before `main`
- replacing x-only serialization with `secp256k1` compressed pubkey serialization (`SerializeCompressed()[1:]`) removed that startup failure.

Current runtime status:
- `secp_smoke` now builds, executes, halts, and proves under `r0vm`.
- the real `bip32` TinyGo guest now:
  - prints `bip32: start`
  - derives the child key
  - computes the BIP-86 Taproot output key
  - commits only that final 32-byte public key
  - halts with a non-zero output digest
- `r0vm` reports for `/tmp/bip32.bin`:
  - `output_digest = 8d72c831edcfbc1ebc9981721882752018e984b0dbca2e5cf948f0f5ba9958d9`
  - `journal = 8724f200544f593846c3a868faa13cfc47fb29842e010758c0b95e6f79896434`
  - `segments = 11`
- important clarification: the long apparent "hang" after guest output was not a stuck guest; `r0vm` had already accepted the halt and was busy generating/proving segments.

Remote-proving note:
- the high CPU usage observed during `r0vm` and Rust-host runs was expected: those runs were performing real proof generation, not only guest execution
- latest official remote-proving docs now point at Boundless as the recommended remote proving path
- Boundless repo for later reference:
  - `https://github.com/boundless-xyz/boundless`
- the Rust SDK in this repo still includes a `BonsaiProver` path and `default_prover()` selection logic for `RISC0_PROVER=bonsai` or `BONSAI_API_URL` + `BONSAI_API_KEY`
- however, remote proving changes the privacy model: for the real BIP-32 seed proof, the remote prover would see the secret input
- conclusion for now:
  - remote proving is useful for public test vectors and offloaded iteration
  - local proving remains the safer default for the final secret-seed proof flow
- host-side prep completed:
  - `risc0/examples/go-guest-test` now accepts an arbitrary guest path plus `--raw-journal`
  - it also prints the backend chosen by `default_prover()`
  - compile-only validation for that harness passed
- current environment state:
  - no `BONSAI_API_URL`
  - no `BONSAI_API_KEY`
  - no explicit `RISC0_PROVER`
  - meaning Bonsai/remote proving is not yet configured in this shell even though the code path exists

## What Already Exists Here

There is already a custom TinyGo zkVM path in-tree:
- `tinygo/targets/zkvm.json`
- `tinygo/targets/riscv32im-risc0-zkvm-elf.ld`
- `tinygo/src/runtime/runtime_zkvm.go`
- `tinygo/src/runtime/sys_zkvm.S`
- `go-zkvm-tests/zkvm/zkvm.go`
- `go-zkvm-tests/zkvm/sha256_proper.go`

This path manually implements guest-side syscall glue and journal-digest handling. The local notes and test harnesses indicate:
- TinyGo guests can be built into ELF/R0BF artifacts.
- At least a dev-mode Rust host flow was previously made to work for simple examples like `multiply`.
- The current Go path still carries custom syscall/runtime code that was added because the earlier approach assumed we had to implement everything ourselves.

## Most Important New Finding

The newer risc0 tree already contains a better model for non-Rust guests.

Relevant evidence:
- `risc0/risc0/zkvm/platform/src/syscall.rs`
- `risc0/risc0/zkvm/platform/Cargo.toml`
- `risc0/risc0/build/src/lib.rs`
- `risc0/examples/c-guest/`

Key points:
- `risc0-zkvm-platform` has an `export-syscalls` feature and exports `sys_read`, `sys_write`, `sys_halt`, `sys_sha_*`, allocator/runtime pieces, and related guest helpers as C symbols.
- `risc0-build` has `build_rust_runtime_with_features(...)`, which explicitly builds `risc0-zkvm-platform` as a `staticlib`.
- `cargo-risczero` packages this archive as `risc0-zkvm-platform.a`.
- `risc0/examples/c-guest` proves that current risc0 supports a non-Rust guest flow by:
  - building a small Rust guest wrapper crate on top of `risc0-zkvm-platform`
  - linking that archive into a C guest
  - proving and verifying the resulting guest with the standard risc0 host flow

Conclusion:
- the thing to link is not a "kernel `.a`"
- the right reusable base is `risc0-zkvm-platform`, usually consumed through a thin guest wrapper archive
- the kernel still belongs at the binary packaging / image construction layer

Status clarification as of 2026-04-03:
- this newer non-Rust guest model has been identified and documented, but it has not been adopted in the working TinyGo pipeline yet
- current working builds still use the custom TinyGo runtime and syscall path:
  - `tinygo/src/runtime/runtime_zkvm.go`
  - `tinygo/src/runtime/sys_zkvm.S`
  - `tinygo/targets/riscv32im-risc0-zkvm-elf.ld`
  - `go-zkvm-tests/Makefile`
  - `convert_to_r0bf.go`
- meaning: the local `bip32` proof-of-execution/proof-of-proving result so far is still on the handwritten TinyGo integration, not on `risc0-zkvm-platform.a`

## Why This Matters For TinyGo

This changes the strategy.

Old strategy:
- keep extending handwritten TinyGo syscall assembly until Go guests cover enough of the risc0 ABI

Better strategy now:
- make TinyGo-produced guest ELFs consume `risc0-zkvm-platform.a`
- reuse upstream guest runtime/syscall/export logic
- keep only the minimum TinyGo-specific glue needed for startup, calling conventions, and Go runtime integration

Expected benefits:
- less duplicated syscall logic
- fewer version skew problems
- closer alignment with current risc0 guest semantics
- simpler path to supporting more syscalls and features

## Practical Integration Hypothesis

The likely target architecture is:

1. TinyGo emits a guest ELF.
2. The final link step pulls in `risc0-zkvm-platform.a` or a very thin Rust wrapper built on top of it.
3. The resulting guest is packaged with the risc0 kernel using the normal program-binary flow.
4. The host proves and verifies exactly like the C guest example does.

Open technical question:
- whether TinyGo can directly link the Rust `staticlib` cleanly, or whether we should introduce a tiny Rust/C shim library with a narrower C ABI for TinyGo to call

My current bias:
- prefer the shim if direct linking is awkward
- prefer direct linking if TinyGo's link step can consume the archive without fighting its runtime

## Recommended Next Steps

1. Rebase the work against newer risc0 guest/runtime assumptions.
2. Reproduce `risc0/examples/c-guest` locally as the reference non-Rust path.
3. Build `risc0-zkvm-platform.a` directly and inspect the exported symbols we need.
4. Attempt a minimal TinyGo guest that links against the platform archive instead of using `tinygo/src/runtime/sys_zkvm.S`.
5. Keep the guest tiny: read two numbers, multiply, commit result, halt, prove, verify.
6. Once that works, decide which pieces of the existing manual TinyGo path can be deleted.

Update-management note:
- if we choose to update the nested upstream repos before attempting the platform-archive migration, there are local changes in both `risc0/` and `tinygo/`
- safest approach is not `stash` if we can avoid it; use local WIP branches or local commits first, then rebase/merge/cherry-pick as needed
- `stash`/`stash pop` is acceptable for a quick experiment, but it is more fragile here because the relevant changes span both repos and some generated/example files

## BIP-32 To Taproot Proof Plan

Once the TinyGo guest path is stable, the actual proof program should be split into phases.

Phase 1:
- prove BIP-32 child private/public derivation for a fixed derivation path

Phase 2:
- compute the internal secp256k1 public key inside the guest

Phase 3:
- compute the Taproot output key inside the guest
  - x-only key handling
  - BIP-340 tagged hash tweak
  - scriptless key-path case first

Phase 4:
- commit the public output key and any agreed public metadata
- verify the receipt against the guest image ID

Likely crypto requirements in the guest:
- HMAC-SHA512 for BIP-32
- secp256k1 scalar and point arithmetic
- tagged hashing for Taproot tweaks
- x-only public key normalization

Chosen Go-side implementation direction:
- use a TinyGo-friendly local subset rather than the full high-level btcsuite stack
- current local module:
  - `github.com/roasbeef/btcd-zkp`
- current contents:
  - minimal BIP-32 private derivation
  - secp256k1 private/public key handling via `github.com/decred/dcrd/dcrec/secp256k1/v4`
- current rule:
  - avoid `btcutil/hdkeychain` in the guest because it pulls in `btcutil`, `crypto/x509`, and `net`
  - avoid `btcec/v2/schnorr` in the guest because its import/init surface is too heavy for the current TinyGo zkVM path
- x-only public keys are currently obtained from compressed secp256k1 serialization by dropping the first byte
- keep guest inputs and outputs binary-only, not string/address oriented

Validation milestone before proving:
- compile a TinyGo guest that derives a known child key from a BIP-32 test vector using the local slimmed-down module
- move that guest into the prove/verify loop
- then add Taproot tweak/key derivation on top of the now-working BIP-32 guest

Important design choice still open:
- exactly what should remain private versus be committed publicly
- likely private: seed / master secret / child secret
- likely public: final Taproot output key, path commitment or restricted path metadata, maybe master fingerprint depending on the use case

## Current Working Assumption

The shortest path is no longer "finish the custom syscall implementation."

The shortest path is:
- use current risc0's non-Rust guest pattern as the base
- adapt TinyGo to consume that runtime/platform archive
- then port the guest logic needed for BIP-32 and Taproot

## Not Yet Verified In This Turn

I have not yet completed these validation steps in this turn:
- direct TinyGo linking against `risc0-zkvm-platform.a`
- a full waited-through composite receipt completion for the 11-segment `bip32` run in this exact note-taking pass
- receipt verification against the existing Rust host/tooling outside direct `r0vm` runs
- whether current `go-zkvm-tests` still works unchanged against newer risc0 `main`
