// merkle.go implements the domain-separated binary Merkle tree used to
// commit to the ordered leaf set in a batch proof. The tree uses explicit
// domain separation between leaves and interior nodes:
//
//   - leaf_node  = H(0x00 || tag || index_le32 || leaf_claim_bytes)
//   - inner_node = H(0x01 || left || right)
//
// The 0x00/0x01 prefix prevents second-preimage attacks where an attacker
// could present an interior node as a leaf. The tag and index binding
// prevent leaf reordering or cross-batch confusion.
//
// The same Merkle code runs both inside the zkVM guest (using zkvm.SumSHA256)
// and on the host side (using crypto/sha256), which is why HashFunc is
// parameterized rather than hardcoded.
package batchclaim

import (
	"encoding/binary"
	"errors"
)

// leafTag is the domain separator prepended to every leaf hash input. It
// prevents cross-protocol confusion and pins the batch leaf format version.
const leafTag = "bip32-pq-zkp:batch-leaf:v1"

// HashFunc computes one 32-byte digest over the provided bytes. The
// implementation differs between guest (zkvm.SumSHA256) and host
// (crypto/sha256.Sum256).
type HashFunc func([]byte) [32]byte

// Proof holds one ordinary Merkle inclusion proof for a disclosed batch leaf.
// It is generated outside the guest from the full leaf set and can be
// verified against the batch Merkle root without access to any leaf receipt.
type Proof struct {
	// LeafIndex is the disclosed leaf position in the ordered batch.
	LeafIndex uint32

	// LeafCount is the total number of leaves in the batch.
	LeafCount uint32

	// LeafClaim is the disclosed leaf journal bytes.
	LeafClaim []byte

	// Siblings are the sibling hashes from leaf to root.
	Siblings [][32]byte
}

// LeafHash computes the domain-separated leaf hash for one ordered claim.
// The input is: tag || index (LE u32) || raw leaf journal bytes. The tag
// and index binding ensure that each leaf occupies a unique, deterministic
// position in the tree.
func LeafHash(index uint32, leafClaim []byte, hash HashFunc) [32]byte {
	data := make([]byte, 0, len(leafTag)+4+len(leafClaim))
	data = append(data, leafTag...)
	data = binary.LittleEndian.AppendUint32(data, index)
	data = append(data, leafClaim...)
	return hashNode(0x00, data, hash)
}

// InnerHash computes one interior Merkle-node hash from two children.
func InnerHash(left, right [32]byte, hash HashFunc) [32]byte {
	data := make([]byte, 0, 1+32+32)
	data = append(data, left[:]...)
	data = append(data, right[:]...)
	return hashNode(0x01, data, hash)
}

// Root builds the Merkle root over the ordered leaf-claim list.
func Root(leaves [][]byte, hash HashFunc) ([32]byte, error) {
	if len(leaves) == 0 {
		return [32]byte{}, errors.New(
			"merkle root requires at least one leaf",
		)
	}

	nodes := make([][32]byte, len(leaves))
	for i, leaf := range leaves {
		nodes[i] = LeafHash(uint32(i), leaf, hash)
	}

	for len(nodes) > 1 {
		nodes = nextLevel(nodes, hash)
	}

	return nodes[0], nil
}

// BuildProof derives one inclusion proof from the ordered leaf set.
func BuildProof(
	leaves [][]byte, index int, hash HashFunc,
) (Proof, [32]byte, error) {

	if len(leaves) == 0 {
		return Proof{}, [32]byte{}, errors.New(
			"proof requires at least one leaf",
		)
	}
	if index < 0 || index >= len(leaves) {
		return Proof{}, [32]byte{}, errors.New(
			"leaf index out of range",
		)
	}

	nodes := make([][32]byte, len(leaves))
	for i, leaf := range leaves {
		nodes[i] = LeafHash(uint32(i), leaf, hash)
	}

	proof := Proof{
		LeafIndex: uint32(index),
		LeafCount: uint32(len(leaves)),
		LeafClaim: append([]byte(nil), leaves[index]...),
	}

	currentIndex := index
	for len(nodes) > 1 {
		siblingIndex := currentIndex ^ 1
		if siblingIndex >= len(nodes) {
			siblingIndex = currentIndex
		}
		proof.Siblings = append(proof.Siblings, nodes[siblingIndex])
		nodes = nextLevel(nodes, hash)
		currentIndex /= 2
	}

	return proof, nodes[0], nil
}

// VerifyProof checks an ordinary inclusion proof against the expected root.
func VerifyProof(root [32]byte, proof Proof, hash HashFunc) bool {
	if proof.LeafCount == 0 {
		return false
	}

	current := LeafHash(proof.LeafIndex, proof.LeafClaim, hash)
	index := proof.LeafIndex
	for _, sibling := range proof.Siblings {
		if index%2 == 0 {
			current = InnerHash(current, sibling, hash)
		} else {
			current = InnerHash(sibling, current, hash)
		}
		index /= 2
	}

	return current == root
}

// nextLevel reduces one tree level by pairing adjacent nodes. If the
// level has odd length, the last node is paired with itself (the standard
// Bitcoin-style odd-level duplication).
func nextLevel(nodes [][32]byte, hash HashFunc) [][32]byte {
	next := make([][32]byte, 0, (len(nodes)+1)/2)
	for i := 0; i < len(nodes); i += 2 {
		left := nodes[i]
		right := left
		if i+1 < len(nodes) {
			right = nodes[i+1]
		}
		next = append(next, InnerHash(left, right, hash))
	}
	return next
}

// hashNode prepends the domain prefix (0x00 for leaves, 0x01 for interior
// nodes) before hashing. This is the core domain separation mechanism.
func hashNode(prefix byte, data []byte, hash HashFunc) [32]byte {
	buf := make([]byte, 1+len(data))
	buf[0] = prefix
	copy(buf[1:], data)
	return hash(buf)
}
