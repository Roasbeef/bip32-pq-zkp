use std::{
    env, fs,
    path::{Path, PathBuf},
};

use anyhow::{bail, ensure, Context, Result};
use borsh::BorshDeserialize;
use clap::{Args, Parser, Subcommand};
use risc0_zkvm::{compute_image_id, default_executor, default_prover, ExecutorEnv, Receipt};
use serde::{Deserialize, Serialize};
use sha2::Digest;

const DEFAULT_BIP32_SEED: [u8; 16] = [
    0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
];
const DEFAULT_BIP32_PATH: [u32; 5] = [0x8000_0056, 0x8000_0000, 0x8000_0000, 0, 0];
const BIP32_HARDENED_KEY_START: u32 = 0x8000_0000;
const BIP86_PURPOSE: u32 = BIP32_HARDENED_KEY_START + 86;
const WITNESS_FLAG_REQUIRE_BIP86: u32 = 1;
const PUBLIC_CLAIM_SIZE: usize = 72;
const PUBLIC_CLAIM_VERSION: u32 = 1;

#[derive(Parser, Debug)]
#[command(name = "bip32-pq-zkp-host")]
#[command(about = "Prove and verify the bip32-pq-zkp demo claim")]
struct Cli {
    #[command(subcommand)]
    command: Command,
}

#[derive(Subcommand, Debug)]
enum Command {
    Execute(WitnessArgs),
    Prove(ProveArgs),
    Verify(VerifyArgs),
}

#[derive(Args, Debug, Clone)]
struct WitnessArgs {
    #[arg(long, default_value = "../bip32-platform-latest.bin")]
    guest: PathBuf,

    #[arg(long)]
    seed_hex: Option<String>,

    #[arg(long)]
    path: Option<String>,

    #[arg(long)]
    use_test_vector: bool,

    #[arg(long)]
    require_bip86: bool,
}

#[derive(Args, Debug)]
struct ProveArgs {
    #[command(flatten)]
    witness: WitnessArgs,

    #[arg(long)]
    receipt_out: PathBuf,

    #[arg(long)]
    claim_out: PathBuf,
}

#[derive(Args, Debug)]
struct VerifyArgs {
    #[arg(long, default_value = "../bip32-platform-latest.bin")]
    guest: PathBuf,

    #[arg(long)]
    receipt_in: PathBuf,

    #[arg(long)]
    claim_in: Option<PathBuf>,

    #[arg(long)]
    expected_pubkey: Option<String>,

    #[arg(long)]
    expected_path_commitment: Option<String>,

    #[arg(long)]
    expected_path: Option<String>,

    #[arg(long, value_parser = clap::builder::BoolishValueParser::new())]
    require_bip86: Option<bool>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct PublicClaim {
    version: u32,
    flags: u32,
    taproot_output_key: [u8; 32],
    path_commitment: [u8; 32],
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
struct ClaimFile {
    schema_version: u32,
    image_id: String,
    claim_version: u32,
    claim_flags: u32,
    require_bip86: bool,
    taproot_output_key: String,
    path_commitment: String,
    journal_hex: String,
    journal_size_bytes: usize,
    proof_seal_bytes: usize,
    receipt_encoding: String,
}

impl PublicClaim {
    fn decode(bytes: &[u8]) -> Result<Self> {
        ensure!(
            bytes.len() == PUBLIC_CLAIM_SIZE,
            "unexpected journal size: got {} bytes, want {}",
            bytes.len(),
            PUBLIC_CLAIM_SIZE
        );

        let version = read_u32_le(bytes, 0)?;
        let flags = read_u32_le(bytes, 4)?;

        let mut taproot_output_key = [0_u8; 32];
        taproot_output_key.copy_from_slice(&bytes[8..40]);

        let mut path_commitment = [0_u8; 32];
        path_commitment.copy_from_slice(&bytes[40..72]);

        Ok(Self {
            version,
            flags,
            taproot_output_key,
            path_commitment,
        })
    }

    fn require_bip86(&self) -> bool {
        self.flags & WITNESS_FLAG_REQUIRE_BIP86 != 0
    }

    fn taproot_output_key_hex(&self) -> String {
        hex::encode(self.taproot_output_key)
    }

    fn path_commitment_hex(&self) -> String {
        hex::encode(self.path_commitment)
    }
}

impl ClaimFile {
    fn from_receipt(image_id: String, receipt: &Receipt, claim: &PublicClaim) -> Self {
        Self {
            schema_version: 1,
            image_id,
            claim_version: claim.version,
            claim_flags: claim.flags,
            require_bip86: claim.require_bip86(),
            taproot_output_key: claim.taproot_output_key_hex(),
            path_commitment: claim.path_commitment_hex(),
            journal_hex: hex::encode(&receipt.journal.bytes),
            journal_size_bytes: receipt.journal.bytes.len(),
            proof_seal_bytes: receipt.seal_size(),
            receipt_encoding: "borsh".to_string(),
        }
    }
}

fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(tracing_subscriber::EnvFilter::from_default_env())
        .init();

    let cli = Cli::parse();
    match cli.command {
        Command::Execute(args) => execute(args),
        Command::Prove(args) => prove(args),
        Command::Verify(args) => verify(args),
    }
}

fn execute(args: WitnessArgs) -> Result<()> {
    let (guest_binary, image_id) = load_guest(&args.guest)?;
    let env = build_witness_env(&args)?;

    println!(
        "✓ Loaded guest binary `{}`: {} bytes",
        args.guest.display(),
        guest_binary.len()
    );
    println!("✓ Image ID: {}", image_id);

    let exec = default_executor();
    let session = exec.execute(env, &guest_binary).context("execute guest")?;
    let claim = PublicClaim::decode(&session.journal.bytes)?;

    println!("✓ Execution successful");
    print_claim(&claim);
    println!("Session info:");
    println!("  Exit code: {:?}", session.exit_code);
    println!("  Journal size: {} bytes", session.journal.bytes.len());
    println!("  Segments: {}", session.segments.len());
    println!("  Rows: {}", session.rows());

    Ok(())
}

fn prove(args: ProveArgs) -> Result<()> {
    let (guest_binary, image_id) = load_guest(&args.witness.guest)?;
    let env = build_witness_env(&args.witness)?;

    println!(
        "✓ Loaded guest binary `{}`: {} bytes",
        args.witness.guest.display(),
        guest_binary.len()
    );
    println!("✓ Image ID: {}", image_id);

    let prover = default_prover();
    println!("✓ Using prover backend: {}", prover.get_name());
    print_acceleration_status();

    let prove_info = prover.prove(env, &guest_binary).context("prove guest")?;
    let receipt = prove_info.receipt;
    receipt
        .verify(image_id)
        .context("self-check receipt against image ID")?;

    let claim = PublicClaim::decode(&receipt.journal.bytes)?;
    ensure!(
        claim.version == PUBLIC_CLAIM_VERSION,
        "unexpected claim version: got {}, want {}",
        claim.version,
        PUBLIC_CLAIM_VERSION
    );
    ensure!(
        claim.require_bip86() == args.witness.require_bip86,
        "claim policy mismatch: journal says require_bip86={}, command used require_bip86={}",
        claim.require_bip86(),
        args.witness.require_bip86
    );

    let claim_file = ClaimFile::from_receipt(image_id.to_string(), &receipt, &claim);
    write_receipt(&args.receipt_out, &receipt)?;
    write_claim(&args.claim_out, &claim_file)?;

    println!("✓ Proof generated and self-verified");
    print_claim(&claim);
    println!("Artifacts:");
    println!("  Receipt: {}", args.receipt_out.display());
    println!("  Claim: {}", args.claim_out.display());
    println!("Receipt info:");
    println!("  Journal size: {} bytes", receipt.journal.bytes.len());
    println!("  Proof seal size: {} bytes", receipt.seal_size());

    Ok(())
}

fn verify(args: VerifyArgs) -> Result<()> {
    let (guest_binary, image_id) = load_guest(&args.guest)?;
    let receipt = read_receipt(&args.receipt_in)?;
    let expected_claim_file = match args.claim_in.as_ref() {
        Some(path) => Some(read_claim(path)?),
        None => None,
    };

    println!(
        "✓ Loaded guest binary `{}`: {} bytes",
        args.guest.display(),
        guest_binary.len()
    );
    println!("✓ Computed image ID: {}", image_id);

    ensure!(
        expected_claim_file.is_some() || has_explicit_verify_expectations(&args),
        "verify requires --claim-in or at least one explicit expectation flag"
    );

    receipt
        .verify(image_id)
        .context("verify receipt against image ID")?;

    let claim = PublicClaim::decode(&receipt.journal.bytes)?;
    let verified_claim = ClaimFile::from_receipt(image_id.to_string(), &receipt, &claim);

    if let Some(claim_file) = expected_claim_file.as_ref() {
        ensure!(
            claim_file.image_id == image_id.to_string(),
            "claim image ID mismatch: claim says {}, guest computes {}",
            claim_file.image_id,
            image_id
        );
        ensure!(
            verified_claim == *claim_file,
            "claim file does not match the verified public receipt output"
        );
    }

    verify_claim_expectations(&args, &claim)?;

    println!("✓ Receipt verified");
    print_claim(&claim);
    println!("Receipt info:");
    println!("  Journal size: {} bytes", receipt.journal.bytes.len());
    println!("  Proof seal size: {} bytes", receipt.seal_size());
    println!("  Receipt file: {}", args.receipt_in.display());
    if let Some(path) = args.claim_in.as_ref() {
        println!("  Claim file: {}", path.display());
    }

    Ok(())
}

fn load_guest(path: &Path) -> Result<(Vec<u8>, risc0_zkvm::sha::Digest)> {
    let guest_binary =
        fs::read(path).with_context(|| format!("read guest binary `{}`", path.display()))?;
    let image_id = compute_image_id(&guest_binary).context("compute image ID")?;
    Ok((guest_binary, image_id))
}

fn build_witness_env(args: &WitnessArgs) -> Result<ExecutorEnv<'static>> {
    let (seed, path, flags, using_test_vector) = load_bip32_witness(
        args.seed_hex.as_deref(),
        args.path.as_deref(),
        args.use_test_vector,
        args.require_bip86,
    )?;

    let witness_desc = if args.require_bip86 {
        "private BIP-32 witness with BIP-86 policy"
    } else {
        "private BIP-32 witness"
    };
    if using_test_vector {
        println!("✓ Sending {witness_desc} (built-in test vector)");
    } else {
        println!("✓ Sending {witness_desc}");
    }

    Ok(ExecutorEnv::builder()
        .write_slice(&[flags])
        .write_slice(&[seed.len() as u32])
        .write_slice(seed.as_slice())
        .write_slice(&[path.len() as u32])
        .write_slice(path.as_slice())
        .build()?)
}

fn load_bip32_witness(
    seed_hex: Option<&str>,
    path_spec: Option<&str>,
    use_test_vector: bool,
    require_bip86: bool,
) -> Result<(Vec<u8>, Vec<u32>, u32, bool)> {
    let (seed, path, using_test_vector) = match (seed_hex, path_spec, use_test_vector) {
        (Some(_), Some(_), true) => {
            bail!("--use-test-vector cannot be combined with --seed-hex/--path")
        }
        (Some(_), None, _) => bail!("--path is required when --seed-hex is set"),
        (None, Some(_), _) => bail!("--seed-hex is required when --path is set"),
        (Some(seed_hex), Some(path_spec), false) => {
            let seed = decode_hex(seed_hex).context("decode --seed-hex")?;
            let path = parse_bip32_path(path_spec).context("parse --path")?;
            (seed, path, false)
        }
        (None, None, true) => (
            DEFAULT_BIP32_SEED.to_vec(),
            DEFAULT_BIP32_PATH.to_vec(),
            true,
        ),
        (None, None, false) => {
            bail!("bip32 guest requires --seed-hex and --path, or --use-test-vector")
        }
    };

    if require_bip86 {
        validate_bip86_path(&path)?;
    }

    let mut flags = 0_u32;
    if require_bip86 {
        flags |= WITNESS_FLAG_REQUIRE_BIP86;
    }

    Ok((seed, path, flags, using_test_vector))
}

fn parse_bip32_path(path_spec: &str) -> Result<Vec<u32>> {
    let trimmed = path_spec.trim();
    let stripped = trimmed
        .strip_prefix("m/")
        .or_else(|| trimmed.strip_prefix("M/"))
        .unwrap_or(trimmed);

    if stripped.is_empty() {
        return Ok(Vec::new());
    }

    let separator = if stripped.contains('/') { '/' } else { ',' };

    stripped
        .split(separator)
        .map(|component| {
            let component = component.trim();
            ensure!(!component.is_empty(), "empty path component");
            let hardened =
                component.ends_with('\'') || component.ends_with('h') || component.ends_with('H');
            let digits = if hardened {
                &component[..component.len() - 1]
            } else {
                component
            };
            let value = digits
                .parse::<u32>()
                .with_context(|| format!("invalid path component `{component}`"))?;
            Ok(if hardened {
                value + BIP32_HARDENED_KEY_START
            } else {
                value
            })
        })
        .collect()
}

fn validate_bip86_path(path: &[u32]) -> Result<()> {
    ensure!(path.len() == 5, "BIP-86 path must have 5 elements");
    ensure!(path[0] == BIP86_PURPOSE, "BIP-86 purpose must be 86'");
    ensure!(
        path[1] >= BIP32_HARDENED_KEY_START,
        "coin_type must be hardened"
    );
    ensure!(
        path[2] >= BIP32_HARDENED_KEY_START,
        "account must be hardened"
    );
    ensure!(
        path[3] < BIP32_HARDENED_KEY_START,
        "change must not be hardened"
    );
    ensure!(
        path[4] < BIP32_HARDENED_KEY_START,
        "index must not be hardened"
    );
    ensure!(path[3] <= 1, "change must be 0 or 1");
    Ok(())
}

fn decode_hex(value: &str) -> Result<Vec<u8>> {
    let trimmed = value
        .strip_prefix("0x")
        .or_else(|| value.strip_prefix("0X"))
        .unwrap_or(value);
    Ok(hex::decode(trimmed)?)
}

fn decode_hex_array_32(label: &str, value: &str) -> Result<[u8; 32]> {
    let bytes = decode_hex(value).with_context(|| format!("decode {label}"))?;
    ensure!(
        bytes.len() == 32,
        "{label} must be 32 bytes, got {}",
        bytes.len()
    );
    let mut out = [0_u8; 32];
    out.copy_from_slice(&bytes);
    Ok(out)
}

fn write_receipt(path: &Path, receipt: &Receipt) -> Result<()> {
    ensure_parent_dir(path)?;
    let bytes = borsh::to_vec(receipt).context("serialize receipt")?;
    fs::write(path, bytes).with_context(|| format!("write receipt `{}`", path.display()))
}

fn read_receipt(path: &Path) -> Result<Receipt> {
    let bytes = fs::read(path).with_context(|| format!("read receipt `{}`", path.display()))?;
    Receipt::try_from_slice(&bytes).context("deserialize receipt")
}

fn write_claim(path: &Path, claim: &ClaimFile) -> Result<()> {
    ensure_parent_dir(path)?;
    let file = fs::File::create(path).with_context(|| format!("create `{}`", path.display()))?;
    serde_json::to_writer_pretty(file, claim).context("serialize claim JSON")
}

fn read_claim(path: &Path) -> Result<ClaimFile> {
    let bytes = fs::read(path).with_context(|| format!("read claim `{}`", path.display()))?;
    serde_json::from_slice(&bytes).context("deserialize claim JSON")
}

fn ensure_parent_dir(path: &Path) -> Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .with_context(|| format!("create parent directory `{}`", parent.display()))?;
    }
    Ok(())
}

fn print_claim(claim: &PublicClaim) {
    println!("Claim:");
    println!("  Version: {}", claim.version);
    println!("  Flags: {}", claim.flags);
    println!("  Require BIP-86: {}", claim.require_bip86());
    println!("  Taproot output key: {}", claim.taproot_output_key_hex());
    println!("  Path commitment: {}", claim.path_commitment_hex());
}

fn print_acceleration_status() {
    #[cfg(all(target_os = "macos", target_arch = "aarch64"))]
    {
        if env::var_os("RISC0_FORCE_CPU_PROVER").is_some() {
            println!("! Metal acceleration disabled by RISC0_FORCE_CPU_PROVER=1");
        } else {
            println!("✓ Metal acceleration compiled in for local proving");
        }
    }
}

fn has_explicit_verify_expectations(args: &VerifyArgs) -> bool {
    args.expected_pubkey.is_some()
        || args.expected_path_commitment.is_some()
        || args.expected_path.is_some()
        || args.require_bip86.is_some()
}

fn verify_claim_expectations(args: &VerifyArgs, claim: &PublicClaim) -> Result<()> {
    ensure!(
        !(args.expected_path_commitment.is_some() && args.expected_path.is_some()),
        "--expected-path and --expected-path-commitment are mutually exclusive"
    );

    if let Some(value) = args.expected_pubkey.as_deref() {
        let expected = decode_hex_array_32("--expected-pubkey", value)?;
        ensure!(
            claim.taproot_output_key == expected,
            "taproot output key mismatch: receipt has {}, expected {}",
            claim.taproot_output_key_hex(),
            hex::encode(expected)
        );
    }

    if let Some(value) = args.expected_path_commitment.as_deref() {
        let expected = decode_hex_array_32("--expected-path-commitment", value)?;
        ensure!(
            claim.path_commitment == expected,
            "path commitment mismatch: receipt has {}, expected {}",
            claim.path_commitment_hex(),
            hex::encode(expected)
        );
    }

    if let Some(path_spec) = args.expected_path.as_deref() {
        let path = parse_bip32_path(path_spec).context("parse --expected-path")?;
        let mut hasher = sha2::Sha256::new();
        hasher.update(b"bip32-pq-zkp:path:v1");
        hasher.update((path.len() as u32).to_le_bytes());
        for component in &path {
            hasher.update(component.to_le_bytes());
        }
        let expected: [u8; 32] = hasher.finalize().into();
        ensure!(
            claim.path_commitment == expected,
            "path commitment mismatch: receipt has {}, expected commitment from path {}",
            claim.path_commitment_hex(),
            hex::encode(expected)
        );
    }

    if let Some(require_bip86) = args.require_bip86 {
        ensure!(
            claim.require_bip86() == require_bip86,
            "claim policy mismatch: receipt says require_bip86={}, expected {}",
            claim.require_bip86(),
            require_bip86
        );
    }

    Ok(())
}

fn read_u32_le(bytes: &[u8], offset: usize) -> Result<u32> {
    let slice = bytes
        .get(offset..offset + 4)
        .with_context(|| format!("read u32 at offset {offset}"))?;
    let mut buf = [0_u8; 4];
    buf.copy_from_slice(slice);
    Ok(u32::from_le_bytes(buf))
}
