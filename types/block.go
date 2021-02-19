package types

// block.go defines the Block type for Sia, and provides some helper functions
// for working with blocks.

import (
	"bytes"
    "encoding/hex"
	"hash"
	"unsafe"

	"gitlab.com/NebulousLabs/Sia/crypto"
	"gitlab.com/NebulousLabs/encoding"
)

const (
	// BlockHeaderSize is the size, in bytes, of a block header.
	// 32 (ParentID) + 8 (Nonce) + 8 (Timestamp) + 32 (MerkleRoot)
	BlockHeaderSize = 80
)

type (
	// A Block is a summary of changes to the state that have occurred since the
	// previous block. Blocks reference the ID of the previous block (their
	// "parent"), creating the linked-list commonly known as the blockchain. Their
	// primary function is to bundle together transactions on the network. Blocks
	// are created by "miners," who collect transactions from other nodes, and
	// then try to pick a Nonce that results in a block whose BlockID is below a
	// given Target.
	Block struct {
		ParentID     BlockID         `json:"parentid"`
		Nonce        BlockNonce      `json:"nonce"`
		Timestamp    Timestamp       `json:"timestamp"`
		MinerPayouts []SiacoinOutput `json:"minerpayouts"`
		Transactions []Transaction   `json:"transactions"`
	}

    BlockTemplate struct {
		ParentID     BlockID         `json:"parentid"`
		Nonce        BlockNonce      `json:"nonce"`
		Timestamp    Timestamp       `json:"timestamp"`
		MinerPayouts []SiacoinOutput `json:"minerpayouts"`
		Transactions []TransactionID   `json:"transactions"`
        Target Target `json:"target"`
        MerkleBranches []string `json:"merklebranches"`
        Height BlockHeight `json:"height"`
    }

	// A BlockHeader contains the data that, when hashed, produces the Block's ID.
	BlockHeader struct {
		ParentID   BlockID     `json:"parentid"`
		Nonce      BlockNonce  `json:"nonce"`
		Timestamp  Timestamp   `json:"timestamp"`
		MerkleRoot crypto.Hash `json:"merkleroot"`
	}

	// BlockHeight is the number of blocks that exist after the genesis block.
	BlockHeight uint64
	// A BlockID is the hash of a BlockHeader. A BlockID uniquely
	// identifies a Block, and indicates the amount of work performed
	// to mine that Block. The more leading zeros in the BlockID, the
	// more work was performed.
	BlockID crypto.Hash
	// The BlockNonce is a "scratch space" that miners can freely alter to produce
	// a BlockID that satisfies a given Target.
	BlockNonce [8]byte
)

// CalculateCoinbase calculates the coinbase for a given height. The coinbase
// equation is:
//
//     coinbase := max(InitialCoinbase - height, MinimumCoinbase) * SiacoinPrecision
func CalculateCoinbase(height BlockHeight) Currency {
	base := InitialCoinbase - uint64(height)
	if uint64(height) > InitialCoinbase || base < MinimumCoinbase {
		base = MinimumCoinbase
	}
	return NewCurrency64(base).Mul(SiacoinPrecision)
}

// CalculateNumSiacoins calculates the number of siacoins in circulation at a
// given height.
func CalculateNumSiacoins(height BlockHeight) (total Currency) {
	total = numGenesisSiacoins
	deflationBlocks := BlockHeight(InitialCoinbase - MinimumCoinbase)
	avgDeflationSiacoins := CalculateCoinbase(0).Add(CalculateCoinbase(height)).Div64(2)
	if height <= deflationBlocks {
		total = total.Add(avgDeflationSiacoins.Mul64(uint64(height + 1)))
	} else {
		total = total.Add(avgDeflationSiacoins.Mul(NewCurrency64(uint64(deflationBlocks + 1))))
		total = total.Add(CalculateCoinbase(height).Mul64(uint64(height - deflationBlocks)))
	}
	if height >= FoundationHardforkHeight {
		total = total.Add(InitialFoundationSubsidy)
		perSubsidy := FoundationSubsidyPerBlock.Mul64(uint64(FoundationSubsidyFrequency))
		subsidies := (height - FoundationHardforkHeight) / FoundationSubsidyFrequency
		total = total.Add(perSubsidy.Mul64(uint64(subsidies)))
	}
	return
}

var numGenesisSiacoins = func() Currency {
	// Sum all the values for the genesis siacoin outputs.
	numGenesisSiacoins := NewCurrency64(0)
	for _, transaction := range GenesisBlock.Transactions {
		for _, siacoinOutput := range transaction.SiacoinOutputs {
			numGenesisSiacoins = numGenesisSiacoins.Add(siacoinOutput.Value)
		}
	}
	return numGenesisSiacoins
}()

// ID returns the ID of a Block, which is calculated by hashing the header.
func (h BlockHeader) ID() BlockID {
	return BlockID(crypto.HashObject(h))
}

// CalculateSubsidy takes a block and a height and determines the block
// subsidy.
func (b Block) CalculateSubsidy(height BlockHeight) Currency {
	subsidy := CalculateCoinbase(height)
	for _, txn := range b.Transactions {
		for _, fee := range txn.MinerFees {
			subsidy = subsidy.Add(fee)
		}
	}
	return subsidy
}

// Header returns the header of a block.
func (b Block) Header() BlockHeader {
	return BlockHeader{
		ParentID:   b.ParentID,
		Nonce:      b.Nonce,
		Timestamp:  b.Timestamp,
		MerkleRoot: b.MerkleRoot(),
	}
}

// ID returns the ID of a Block, which is calculated by hashing the
// concatenation of the block's parent's ID, nonce, and the result of the
// b.MerkleRoot(). It is equivalent to calling block.Header().ID()
func (b Block) ID() BlockID {
	return b.Header().ID()
}

// MerkleRoot calculates the Merkle root of a Block. The leaves of the Merkle
// tree are composed of the miner outputs (one leaf per payout), and the
// transactions (one leaf per transaction).
func (b Block) MerkleRoot() crypto.Hash {
	tree := crypto.NewTree()
	var buf bytes.Buffer
	e := encoding.NewEncoder(&buf)
	for _, payout := range b.MinerPayouts {
		payout.MarshalSia(e)
		tree.Push(buf.Bytes())
		buf.Reset()
	}
	for _, txn := range b.Transactions {
		txn.MarshalSia(e)
		tree.Push(buf.Bytes())
		buf.Reset()
	}
	return tree.Root()
}

// MinerPayoutID returns the ID of the miner payout at the given index, which
// is calculated by hashing the concatenation of the BlockID and the payout
// index.
func (b Block) MinerPayoutID(i uint64) SiacoinOutputID {
	return SiacoinOutputID(crypto.HashAll(
		b.ID(),
		i,
	))
}

func (b Block) BlockTemplate() BlockTemplate {
    var txs []TransactionID

    for _, txn := range b.Transactions {
        txs = append(txs, txn.ID())
    }

    return BlockTemplate{
        ParentID: b.ParentID,
        Nonce: b.Nonce,
        Timestamp: b.Timestamp,
        MinerPayouts: b.MinerPayouts,
        Transactions: txs,
        // MerkleBranches: b.MerkleBranches(),
    }
}

// FoundationSubsidyID returns the ID of the Foundation subsidy, which is
// calculated by hashing the concatenation of the BlockID and
// SpecifierFoundation.
func (bid BlockID) FoundationSubsidyID() SiacoinOutputID {
	return SiacoinOutputID(crypto.HashAll(bid, SpecifierFoundation))
}

func (b Block) MerkleBranches() []string {
	mbranch := crypto.NewTree()
	var buf bytes.Buffer
	e := encoding.NewEncoder(&buf)
	for _, payout := range b.MinerPayouts {
		payout.MarshalSia(e)
		mbranch.Push(buf.Bytes())
		buf.Reset()
	}

	for _, txn := range b.Transactions {
		txn.MarshalSia(e)
		mbranch.Push(buf.Bytes())
		buf.Reset()
	}
	//
	// This whole approach needs to be revisited.  I basically am cheating to look
	// inside the merkle tree struct to determine if the head is a leaf or not
	//
	type SubTree struct {
		next   *SubTree
		height int // Int is okay because a height over 300 is physically unachievable.
		sum    []byte
	}

	type Tree struct {
		head         *SubTree
		hash         hash.Hash
		currentIndex uint64
		proofIndex   uint64
		proofSet     [][]byte
		cachedTree   bool
	}
	tr := *(*Tree)(unsafe.Pointer(mbranch))

	var merkle []string
	//	h.log.Debugf("mBranch Hash %s\n", mbranch.Root().String())
	for st := tr.head; st != nil; st = st.next {
		//		h.log.Debugf("Height %d Hash %x\n", st.height, st.sum)
		merkle = append(merkle, hex.EncodeToString(st.sum))
	}
	return merkle
}
