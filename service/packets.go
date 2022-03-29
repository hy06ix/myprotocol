package service

import (
	"github.com/hy06ix/onet/network"
)

var BlockProposalType network.MessageTypeID
var BootstrapType network.MessageTypeID
var NotarizedBlockType network.MessageTypeID
var TransactionProofType network.MessageTypeID
var BlockHeaderType network.MessageTypeID
var NotarizedRefBlockType network.MessageTypeID

func init() {
	BlockProposalType = network.RegisterMessage(&BlockProposal{})
	BootstrapType = network.RegisterMessage(&Bootstrap{})
	NotarizedBlockType = network.RegisterMessage(&NotarizedBlock{})

	TransactionProofType = network.RegisterMessage(&TransactionProof{})
	BlockHeaderType = network.RegisterMessage(&BlockHeader{})
	NotarizedRefBlockType = network.RegisterMessage(&NotarizedRefBlock{})
}

type BlockProposal struct {
	TrackId int
	*Block
	Count      int                 // count of parital signatures in the array
	Signatures []*PartialSignature // Partial signature from the signer
}

// type for Transaction proof
type TransactionProof struct {
	TxHash      string
	BlockHeight int
	ShardIndex  int
	MerklePath  []string
	validity    bool
}

type PartialSignature struct {
	Signer  int
	Partial []byte
}

type NotarizedBlock struct {
	Round       int
	Hash        string
	Signature   []byte
	BlockHeader *BlockHeader
}

type NotarizedRefBlock struct {
	NotarizedBlock
}

type Bootstrap struct {
	*Block
	Seed int
}
