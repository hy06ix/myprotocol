package service

import (
	"encoding/binary"
	"encoding/hex"
	"sync"
	//"github.com/csanti/onet/log"
)

type BlockChain struct {
	sync.Mutex
	all    []*Block
	last   *Block
	length int
}

// BlockHeader represents all the information regarding a block
type BlockHeader struct {
	Round      int    // round of the block
	Owner      int    // index of the owner of the block
	Root       string // hash of the data
	Randomness uint32  // randomness of the round
	PrvHash    string // hash of the previous block
	PrvSig     []byte // signature of the previous block (i.e. notarization)
	Signature  []byte
}

// Block represents how a block is stored locally
// Block is first sent from a block maker
type Block struct {
	*BlockHeader
	Blob []byte // the actual content
}

// Appends add a new block to the head of the chain
func (bc *BlockChain) Append(b *Block, isGenesis bool) int {
	bc.Lock()
	defer bc.Unlock()
	// we can not append a block that does not point to he latest block
	/*
	if !isGenesis && bc.length > 0 && b.BlockHeader.PrvHash != bc.last.BlockHeader.Hash() {
		panic("that should never happen")
	}
	*/
	bc.last = b
	bc.length++

	bc.all = append(bc.all, b)
	return bc.length
}

// length returns the length of the finalized chain
func (bc *BlockChain) Length() int {
	bc.Lock()
	defer bc.Unlock()
	return bc.length
}

//
func (bc *BlockChain) Head() *Block {
	bc.Lock()
	defer bc.Unlock()
	return bc.last
}

// Hash returns the hash in hexadecimal of the header
func (h *BlockHeader) Hash() string {
	hash := Suite.Hash()
	binary.Write(hash, binary.BigEndian, h.Owner)
	binary.Write(hash, binary.BigEndian, h.Round)
	hash.Write([]byte(h.PrvHash))
	hash.Write([]byte(h.Root))
	hash.Write(h.PrvSig)
	buff := hash.Sum(nil)
	return hex.EncodeToString(buff)
}

//
func (bc *BlockChain) CreateGenesis() *Block {
	header := &BlockHeader {
		Round: 0,
		Owner: -1,
		Root:  "6afbc27f4ae8951a541be53038ca20d3a9f18f60a38b1dc2cd48a46ff5d26ace",
		// sha256("hello world")
		PrvHash: "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447",
		// echo "hello world" | sha256sum | sha256sum
		PrvSig: []byte("3605ff73b6faec27aa78e311603e9fe2ef35bad82ccf46fc707814bfbdcc6f9e"),
		Signature: []byte("6585ff73b6faec27aa78e311603e9fe2ef35bad82ccf46fc707814bfbdcc6f9e"),
	}
	return &Block{
		BlockHeader: header,
		Blob: []byte("Hello Genesis"),
	}
}
