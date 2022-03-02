package service

import (
	"sync"
	"crypto/rand"
	"crypto/sha256"
	"time"
	"encoding/binary"
	"encoding/hex"

	"github.com/csanti/onet"
	"github.com/csanti/onet/log"
	"github.com/csanti/onet/network"
)

type Node struct {
	*sync.Cond
	*onet.ServiceProcessor

	// config
	c *Config
	// current round number
	round int
	// finalized Chain
	chain *BlockChain
	// information of previous rounds
	rounds map[int]*RoundStorage
	// done callback
	callback func(int) // callsback number of finalized blocks
	// node started the Consensus
	isGenesis bool

	broadcast BroadcastFn
	gossip BroadcastFn
}

func NewNodeProcess(c *onet.Context, conf *Config, b BroadcastFn, g BroadcastFn) *Node {
	// need to create chain first
	chain := new(BlockChain)
	n := &Node {
		ServiceProcessor: onet.NewServiceProcessor(c),
		Cond: sync.NewCond(new(sync.Mutex)),
		chain: chain,
		c:     conf,
    	broadcast: b,
    	gossip: g,
    	rounds: make(map[int]*RoundStorage),
	}
	return n
}

func (n *Node) AttachCallback(fn func(int)) {
	// usually only attached to one of the nodes to notify a higher layer of the progress
	n.callback = fn
}

func (n *Node) StartConsensus() {
	log.Lvl1("Staring consensus")
	n.isGenesis = true
	packet := &Bootstrap{
		Block: n.chain.CreateGenesis(),
		Seed:  1234,
	}
	log.Lvl2("Starting consensus, sending bootstrap..")
	// send bootstrap message to all nodes
	go n.broadcast(n.c.Roster.List, packet)
	n.ReceivedBootstrap(packet)
}

func (n *Node) Process(e *network.Envelope) {
	n.Cond.L.Lock()
	defer n.Cond.L.Unlock()
	defer n.Cond.Broadcast()
	switch inner := e.Msg.(type) {
		case *BlockProposal:
			n.ReceivedBlockProposal(inner)
		case *Bootstrap:			
			n.ReceivedBootstrap(inner)
		case *NotarizedBlock:
			n.ReceivedNotarizedBlock(inner)
		default:
			log.Lvl2("Received unidentified message")
	}
}

func (n *Node) NewRound(round int) {
	if round > n.c.RoundsToSimulate + 1 {
		return
	}
	// new round can only be called after previous round is finished, so this is safe
	n.round = round
	// generate round randomness (sha256 - 32 bytes size)
	roundRandomness := n.generateRoundRandomness(round) // should change... seed should be based on prev block sign
	log.Lvlf3("%d - Round randomness: %s",n.c.Index, hex.EncodeToString(roundRandomness))
	// create the round storage
	if (n.rounds[round] == nil) {
		n.rounds[round] = NewRoundStorage(n.c, round)
	}
	n.rounds[round].Randomness = binary.BigEndian.Uint32(roundRandomness)
	// pick block proposer
	proposerPosition := n.pickBlockProposer(binary.BigEndian.Uint32(roundRandomness), n.c.N)
	log.Lvlf3("Block proposer picked - position %d of %d", proposerPosition, n.c.N)
	n.rounds[round].ProposerIndex = proposerPosition
	
	// start round loop which will periodically check round end conditions
	go n.roundLoop(round)

	// check if node is proposer, if not: returns
	if (proposerPosition == n.c.Index) {
		log.Lvlf1("%d - I am block proposer for round %d !", n.c.Index, round)
	} else {
		return
	}

	// generate block proposal
	oldBlock := n.chain.Head()
	blob := make([]byte, n.c.BlockSize)
	rand.Read(blob)
	hash := rootHash(blob)
	header := &BlockHeader {
		Round:      round,
		Owner:      n.c.Index,
		Root:       hash,
		Randomness: binary.BigEndian.Uint32(roundRandomness),
		PrvHash:    oldBlock.BlockHeader.Hash(),
		PrvSig:     oldBlock.BlockHeader.Signature,
	}
	blockProposal := &Block {
		BlockHeader: header,
		Blob:        blob,
	}
	n.rounds[round].StoreValidBlock(blockProposal)
	n.rounds[round].SignBlock(n.c.Index)
	n.rounds[round].ReceivedValidBlock = true
	sigs, _ := n.rounds[round].ProcessBlockProposals()
	packet := &BlockProposal {
		Block: blockProposal,
		Signatures: sigs,
		Count: 1,
	}
	log.Lvl1("Sending block of size ",len(packet.Block.Blob))
	log.Lvlf3("Broadcasting block proposal for round %d", round)
	go n.gossip(n.c.Roster.List, packet)
}

func (n *Node) ReceivedBlockProposal(p *BlockProposal) {
	blockRound := p.Block.BlockHeader.Round
	if blockRound < n.round {
		log.Lvl3("received too old block")
		return
	}
	_, exists := n.rounds[blockRound]
	if (!exists) {
		log.Lvlf2("%d - BPrecv, Round storage for round %d does not exist", n.c.Index, blockRound)
		n.rounds[blockRound] = NewRoundStorage(n.c, blockRound)
	}

	// we store all received block proposal received, they will be checked and processed on roundloop
	n.rounds[blockRound].StoreBlockProposal(p)
}

func (n *Node) ReceivedNotarizedBlock(nb *NotarizedBlock) {
	// check if rs exists
	if (nb.Round < n.round) {
		log.Lvl3("received too old notarized block")
		return
	}
	if n.rounds[nb.Round] != nil && n.rounds[nb.Round].Finalized {
		return
	}
	log.Lvl2("Received Notarized Block for round ", nb.Round)
	// we can finalize all previous blocks
	for i := n.round; i <= nb.Round; i++ {
		if n.rounds[i] == nil {
			n.rounds[i] = NewRoundStorage(n.c, i)
		}
		n.rounds[i].Finalized = true
		n.rounds[i].FinalSig = nb.Signature
	}
	go n.gossip(n.c.Roster.List, nb)
}

func (n *Node) ReceivedBootstrap(b *Bootstrap) {
	log.Lvl2("Processing bootstrap message... starting consensus")
	// add genesis and start new round
	if(n.chain.Append(b.Block, true) != 1) {
		panic("this should never happen")
	}
	n.NewRound(0)
}

func (n *Node) roundLoop(round int) {
	log.Lvlf3("Starting round %d loop",round)
	// 
	defer func() {
		if n.isGenesis {
			log.Lvlf1("Round %d finished",round)
		}
		log.Lvlf3("%d - Exiting round %d loop",n.c.Index,round)
		n.NewRound(round+1)
		if n.callback != nil {
			n.callback(round)
		}
		// TODO append notarized block to the blockchain
		//delete(n.rounds, round)
	}()
	n.Cond.L.Lock()
	defer n.Cond.L.Unlock()

	var times int = 0
	for {
		time.Sleep(time.Duration(n.c.GossipTime) * time.Millisecond)

		_, exists := n.rounds[round]
		if !exists {
			log.Lvlf2("Round storage for round %d does not exist", round)
			continue
		}

		if n.rounds[round].Finalized {
			log.Lvl3("Exiting round loop, is finalized")
			// we received notarized block
			return
		}

		if times > n.c.MaxRoundLoops { // max
			log.Lvlf1("Node %d Reached max round loops!!", n.c.Index)
			//return
		}
		
		// keep waiting for a valid block
		/*
		for !n.rounds[round].ReceivedValidBlock {
			log.Lvlf1("n:%d r: %d - Waiting 1", n.c.Index, round)
			n.Cond.Wait()
		}
		*/

		var combinedSigs []*PartialSignature
		var haveNewSigs bool
		for {
			if n.rounds[round].Finalized {
				// round is finished
				// we should check the validity of notarized block
				return
			}
			combinedSigs, haveNewSigs = n.rounds[round].ProcessBlockProposals()
			if !haveNewSigs {
				// we dont have new info to send
				log.Lvlf3("n:%d r: %d - Waiting 2", n.c.Index, round)
				n.Cond.Wait()	
			} else {
				break
			}
		}

		// check round finish conditions
		if n.rounds[round].SigCount >= n.c.Threshold {
			// enugh signatures, we need to recover sig and send
			log.Lvlf2("We have enough signatures for round %d", round)
			nb, err := n.rounds[round].NotarizeBlock()
			if err != nil {
				log.Lvlf1("Error generating notarized block: %s", err)
				continue
			}
			go n.gossip(n.c.Roster.List, nb)
			return
		}		
		log.Lvlf3("n:%d r: %d - Gossiping new bp", n.c.Index, round)
		// we dont have enough signatures
		// send new block proposal with newly collected signatures
		block := n.rounds[round].Block
		iteration := n.rounds[round].SentBlockProposals
		newBp := n.generateBlockProposal(block,combinedSigs,iteration)
		if times > 0 {
			// each node only full block one time
			newBp.Block.Blob = []byte("hi")
		}
		go n.gossip(n.c.Roster.List, newBp)
		times++
	}
}

// generates round randomness as a byte array based on a given seed
func (n *Node) generateRoundRandomness(seed int) []byte {
	rHash := Suite.Hash()
	//log.Lvl1(rHash)
	err := binary.Write(rHash, binary.BigEndian, uint32(seed + 1)) //TODO for testing... must change
	if err != nil {
		log.Lvl1("Error writing to hash buffer")
	}
	//log.Lvl1(rHash)
	buff := rHash.Sum(nil)
	//log.Lvl1(buff)
	return buff
} 

func (n *Node) pickBlockProposer(randomness uint32, listSize int) int {
	return int(randomness) % listSize
}

func rootHash(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func (n *Node) generateBlockProposal(block *Block, sigs []*PartialSignature, iteration int) *BlockProposal {
	trackId := n.c.Index * 10 + iteration
	bp := &BlockProposal {
		Block: block,
		TrackId: trackId,
	}
	if (sigs != nil) {
		bp.Signatures = sigs
		bp.Count = len(sigs)
	} else {
		log.Lvl2("Generating BP without signatures")
	}
	return bp
}