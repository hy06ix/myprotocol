package service

import (
	"github.com/csanti/onet/network"
)

var BlockProposalType network.MessageTypeID
var BootstrapType network.MessageTypeID
var NotarizedBlockType network.MessageTypeID

func init() {
	BlockProposalType = network.RegisterMessage(&BlockProposal{})
	BootstrapType = network.RegisterMessage(&Bootstrap{})
	NotarizedBlockType = network.RegisterMessage(&NotarizedBlock{})
}

type BlockProposal struct {
	TrackId int
	*Block
	Count int // count of parital signatures in the array
	Signatures []*PartialSignature // Partial signature from the signer
}

type PartialSignature struct {
	Signer int
	Partial []byte
}

type NotarizedBlock struct {
	Round int
	Hash string
	Signature []byte
}

type Bootstrap struct {
	*Block
	Seed int
}
