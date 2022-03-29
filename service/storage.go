package service

import (
	"errors"
	"time"

	"github.com/hy06ix/onet/log"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/tbls"
)

type RoundStorage struct {
	c                      *Config
	Round                  int
	Randomness             uint32
	ProposerIndex          int
	ReceivedValidBlock     bool // true if we received a block from valid proposer
	Block                  *Block
	BlockHash              string
	FinalSig               []byte                    // when notarization happenned
	Sigs                   map[int]*PartialSignature // all signatures for the blob received so far
	SigCount               int
	Finalized              bool // true if already notarized
	ReceivedBlockProposals int
	SentBlockProposals     int
	StoredBlockProposals   int
	TmpBlockProposals      map[int]*BlockProposal
	pub                    *share.PubPoly
}

func NewRoundStorage(c *Config, round int) *RoundStorage {
	return &RoundStorage{
		c:                 c,
		Round:             round,
		Sigs:              make(map[int]*PartialSignature),
		TmpBlockProposals: make(map[int]*BlockProposal),
		pub:               share.NewPubPoly(G2, G2.Point().Base(), c.Public),
	}
}

func (rs *RoundStorage) StoreValidBlock(b *Block) {
	rs.Block = b
	rs.BlockHash = b.BlockHeader.Hash()
	rs.ReceivedValidBlock = true
}

// stores block proposal after checking that it is the latest iteration of a particular sender
// also deletetes previous block proposal iterations from the sender
func (rs *RoundStorage) StoreBlockProposal(p *BlockProposal) {
	rs.ReceivedBlockProposals++
	//source := int(p.TrackId / 10)

	/*
		// check if a newer iteration from the same sender was previously received
		for i := p.TrackId; i < source*10 + 10; i++ {
			_, exists := rs.TmpBlockProposals[p.TrackId]
			if exists {
				// later iteration exists
				return
			}
		}
		// delete all previsou iterations, this will make round loop faster
		for i := source*10 ; i < p.TrackId ; i++ {
			_, exists := rs.TmpBlockProposals[i]
			if exists {
				delete(rs.TmpBlockProposals, i)
				rs.StoredBlockProposals--
			}
		}
	*/
	rs.StoredBlockProposals++
	rs.TmpBlockProposals[p.TrackId] = p
}

func (rs *RoundStorage) ProcessBlockProposals() ([]*PartialSignature, bool) {
	initialSigCount := rs.SigCount
	for _, bp := range rs.TmpBlockProposals {
		// TODO check validity of block proposal
		if bp.Block.BlockHeader.Owner != rs.ProposerIndex {
			log.Lvl1("received block with invalid proposer")
			continue
		}
		if !rs.ReceivedValidBlock {
			// first valid block we process, we store it and sign it
			rs.StoreValidBlock(bp.Block)
			rs.SignBlock(rs.c.Index)
		} /*
			if bp.Block.BlockHeader.Hash() != rs.BlockHash {
				log.Lvl1("received two different blocks from valid proposer")
				continue
			}*/
		for _, ps := range bp.Signatures {
			// we can make this more efficient if everytime we check if we have enough signatures to finish
			// so we dont add unnecessary sigs
			err := rs.AddPartialSig(ps)
			if err != nil {
				log.Lvl1("Error validating partial signature")
			}
			if rs.SigCount >= rs.c.Threshold {
				break
			}
		}
	}
	// TODO i could save one map conversion if i save the array in memory and use it again when there is no info change

	var sigsArray []*PartialSignature

	if (rs.SigCount - initialSigCount) > 0 {
		log.Lvlf2("n:%d r:%d - Finished processing block proposals - sign count = %d (%d new)", rs.c.Index, rs.Round, rs.SigCount, rs.SigCount-initialSigCount)
		sigsArray = rs.mapToArray(rs.Sigs)
		return sigsArray, true
	} else {
		return sigsArray, false
	}
}

// AddPartialSig appends a new tbls signature to the list of already received signature
// for this block. It returns an error if the signature is invalid.
func (rs *RoundStorage) AddPartialSig(p *PartialSignature) error {

	i, err := tbls.SigShare(p.Partial).Index()
	if err != nil {
		return err
	}
	if rs.Sigs[i] != nil {
		return nil
	}

	err = tbls.Verify(Suite, rs.pub, []byte(rs.BlockHash), p.Partial)
	if err != nil {
		return err
	}

	rs.Sigs[i] = p
	rs.SigCount++

	return nil
}

// Sign block creates the partial signature and adds it to the round storage
func (rs *RoundStorage) SignBlock(index int) *PartialSignature {
	sig, err := tbls.Sign(Suite, rs.c.Share, []byte(rs.BlockHash))
	if err != nil {
		panic("this should not happen")
	}
	ps := &PartialSignature{
		Signer:  index,
		Partial: sig,
	}
	rs.Sigs[index] = ps
	rs.SigCount++
	return ps
}

func (rs *RoundStorage) NotarizeBlock() (*NotarizedBlock, error) {
	if rs.SigCount < rs.c.Threshold {
		return nil, errors.New("not enough signatures")
	}

	arr := make([][]byte, 0, rs.c.Threshold)
	for _, val := range rs.Sigs {
		arr = append(arr, val.Partial)
	}
	start := time.Now()
	hash := rs.BlockHash
	//signature, err := tbls.Recover(Suite, rs.pub, []byte(hash), arr, rs.c.Threshold, rs.c.N)
	signature, err := Recover(rs.pub, []byte(hash), arr, rs.c.Threshold, rs.c.N)
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(start)
	if rs.c.Index == 1 {
		log.Lvl1("********************************************")
		log.Lvl1("NotarizeBlock elapsed time = ", elapsed, rs.c.Threshold)
		log.Lvl1("********************************************")
	}
	rs.Finalized = true
	rs.FinalSig = signature
	return &NotarizedBlock{
		Round:     rs.Round,
		Hash:      hash,
		Signature: signature,
	}, nil
}

func (rs *RoundStorage) mapToArray(m map[int]*PartialSignature) []*PartialSignature {
	array := make([]*PartialSignature, len(m))
	for _, p := range m {
		array = append(array, p)
	}
	return array
}

func Recover(public *share.PubPoly, msg []byte, sigs [][]byte, t, n int) ([]byte, error) {
	pubShares := make([]*share.PubShare, 0)
	for _, sig := range sigs {
		s := tbls.SigShare(sig)
		i, err := s.Index()
		if err != nil {
			return nil, err
		}
		//if err = bls.Verify(suite, public.Eval(i).V, msg, s.Value()); err != nil {
		//	return nil, err
		//}
		point := Suite.G1().Point()
		if err := point.UnmarshalBinary(s.Value()); err != nil {
			return nil, err
		}
		pubShares = append(pubShares, &share.PubShare{I: i, V: point})
		if len(pubShares) >= t {
			break
		}
	}
	commit, err := share.RecoverCommit(Suite.G1(), pubShares, t, n)
	if err != nil {
		return nil, err
	}
	sig, err := commit.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return sig, nil
}
