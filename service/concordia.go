package service

import (
	"github.com/hy06ix/onet"
	"github.com/hy06ix/onet/log"
	"github.com/hy06ix/onet/network"
	"go.dedis.ch/kyber/v3/pairing/bn256"
)

var Suite = bn256.NewSuite()
var G2 = Suite.G2()
var Name = "concordia"

func init() {
	onet.RegisterNewService(Name, NewConcordiaService)
}

// Concordia service is either a beacon a notarizer or a block maker
type Concordia struct {
	*onet.ServiceProcessor
	context       *onet.Context
	c             *Config
	node          *Node
	blockChain    *BlockChain
	backboneChain *BlockChain
}

// NewConcordiaService
func NewConcordiaService(c *onet.Context) (onet.Service, error) {
	n := &Concordia{
		context:          c,
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	c.RegisterProcessor(n, ConfigType)
	c.RegisterProcessor(n, BootstrapType)
	c.RegisterProcessor(n, BlockProposalType)
	c.RegisterProcessor(n, NotarizedBlockType)

	c.RegisterProcessor(n, TransactionProofType)
	c.RegisterProcessor(n, NotarizedRefBlockType)
	c.RegisterProcessor(n, BlockHeaderType)

	return n, nil
}

func (n *Concordia) SetNode(node *Node) {
	n.node = node
}

func (n *Concordia) GetInfo() {
	log.Lvl1(n.context)
	log.Lvl1(n.c)
	log.Lvl1(n.node)
	log.Lvl1(n.blockChain)
	log.Lvl1(n.backboneChain)
}

func (n *Concordia) SetConfig(c *Config) (node *Node) {
	log.Lvl3("Calling SetConfig")
	n.c = c
	if n.c.CommunicationMode == 0 {
		n.node = NewNodeProcess(n.context, c, n.broadcast, n.gossip, n.send)
	} else if n.c.CommunicationMode == 1 {
		n.node = NewNodeProcess(n.context, c, n.broadcast, n.gossip, n.send)
	} else {
		panic("Invalid communication mode")
	}
	log.Lvl3("Finish SetConfig")

	return n.node
}

func (n *Concordia) AttachCallback(fn func(int, int)) {
	// attach to something.. haha lol xd
	if n.node != nil {
		n.node.AttachCallback(fn)
	} else {
		log.Lvl1("Could not attach callback, node is nil")
	}
}

func (n *Concordia) Start() {
	// send a bootstrap message
	if n.node != nil {
		n.node.StartConsensus()
	} else {
		panic("that should not happen")
	}
}

// Process
func (n *Concordia) Process(e *network.Envelope) {
	switch inner := e.Msg.(type) {
	case *Config:
		n.SetConfig(inner)
	case *Bootstrap:
		n.node.Process(e)
	case *BlockProposal:
		n.node.Process(e)
	case *BlockHeader:
		n.node.Process(e)
	case *NotarizedRefBlock:
		n.node.Process(e)
	case *NotarizedBlock:
		n.node.Process(e)
	case *TransactionProof:
		n.node.Process(e)
	default:
		log.Lvl1("Received unidentified message")
	}
}

// depreciated
func (n *Concordia) getRandomPeers(numPeers int) []*network.ServerIdentity {
	var results []*network.ServerIdentity
	for i := 0; i < numPeers; {
		posPeer := n.c.Roster.RandomServerIdentity()
		if n.ServerIdentity().Equal(posPeer) {
			// selected itself
			continue
		}
		results = append(results, posPeer)
		i++
	}
	return results
}

type BroadcastFn func(sis []*network.ServerIdentity, msg interface{})

func (n *Concordia) broadcast(sis []*network.ServerIdentity, msg interface{}) {
	for _, si := range sis {
		if n.ServerIdentity().Equal(si) {
			continue
		}
		log.Lvlf4("Broadcasting from: %s to: %s", n.ServerIdentity(), si)
		if err := n.ServiceProcessor.SendRaw(si, msg); err != nil {
			log.Lvl1("Error sending message")
			//panic(err)
		}
	}
}

func (n *Concordia) gossip(sis []*network.ServerIdentity, msg interface{}) {
	//targets := n.getRandomPeers(n.c.GossipPeers)
	targets := n.c.Roster.RandomSubset(n.ServerIdentity(), n.c.GossipPeers).List
	for k, target := range targets {
		if k == 0 {
			continue
		}
		log.Lvlf4("Gossiping from: %s to: %s", n.ServerIdentity(), target)
		if err := n.ServiceProcessor.SendRaw(target, msg); err != nil {
			log.Lvl1("Error sending message")
		}
	}
}

type DirectSendFn func(sis *network.ServerIdentity, msg interface{})

// function for send header to reference shard directly
func (n *Concordia) send(si *network.ServerIdentity, msg interface{}) {
	// log.Lvl1(reflect.TypeOf(msg))
	log.Lvlf4("Sending from: %s to: %s", n.ServerIdentity(), si)
	if err := n.ServiceProcessor.SendRaw(si, msg); err != nil {
		log.Lvl1("Error sending message")
	}
}
