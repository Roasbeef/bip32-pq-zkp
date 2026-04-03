package main

import (
	"github.com/roasbeef/bip32-pq-zkp/bip32"
	"github.com/roasbeef/go-zkvm/zkvm"
)

const (
	maxSeedBytes     = 64
	maxPathDepth     = 16
	flagRequireBIP86 = 1
)

func main() {
	zkvm.Debug("bip32: start\n")

	var flags uint32
	zkvm.ReadValue(&flags)

	var seedLen uint32
	zkvm.ReadValue(&seedLen)
	if seedLen < 16 || seedLen > maxSeedBytes {
		zkvm.Debug("invalid seed length\n")
		zkvm.Halt(1)
	}

	var seed [maxSeedBytes]byte
	zkvm.Read(seed[:int(seedLen)])

	var pathLen uint32
	zkvm.ReadValue(&pathLen)
	if pathLen > maxPathDepth {
		zkvm.Debug("invalid path length\n")
		zkvm.Halt(1)
	}

	var path [maxPathDepth]uint32
	for i := uint32(0); i < pathLen; i++ {
		zkvm.ReadValue(&path[i])
	}

	pathSlice := path[:int(pathLen)]

	var opts []bip32.TaprootDeriveOption
	if flags&flagRequireBIP86 != 0 {
		opts = append(opts, bip32.WithBIP86PathVerification())
	}

	outputKey, err := bip32.DeriveTaprootOutputKey(seed[:int(seedLen)], pathSlice, opts...)
	if err != nil {
		zkvm.Debug("DeriveTaprootOutputKey failed\n")
		zkvm.Halt(1)
	}

	zkvm.Debug("bip32: derived\n")
	zkvm.Commit(outputKey)
	zkvm.Debug("bip32: committed\n")
	zkvm.Halt(0)
}
