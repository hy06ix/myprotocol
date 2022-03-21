package service

import (
	"github.com/csanti/onet/network"
)

var BlockProposalType network.MessageTypeID
var BootstrapType network.MessageTypeID
var NotarizedBlockType network.MessageTypeID
var TransactionProofType network.MessageTypeID

func init() {
	BlockProposalType = network.RegisterMessage(&BlockProposal{})
	BootstrapType = network.RegisterMessage(&Bootstrap{})
	NotarizedBlockType = network.RegisterMessage(&NotarizedBlock{})

	TransactionProofType = network.RegisterMessage(&TransactionProof{})
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
	BlockHeader string
}

type Bootstrap struct {
	*Block
	Seed int
}
