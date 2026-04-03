module github.com/roasbeef/bip32-pq-zkp

go 1.22

toolchain go1.24.4

replace github.com/roasbeef/go-zkvm => ../go-zkvm

require (
	github.com/btcsuite/btcd v0.24.3-0.20250318170759-4f4ea81776d6
	github.com/btcsuite/btcd/btcec/v2 v2.3.4
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.1
	github.com/roasbeef/go-zkvm v0.0.0
)

require (
	github.com/btcsuite/btcd/btcutil v1.1.5 // indirect
	github.com/btcsuite/btcd/chaincfg/chainhash v1.1.0 // indirect
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f // indirect
	github.com/decred/dcrd/crypto/blake256 v1.1.0 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
)
