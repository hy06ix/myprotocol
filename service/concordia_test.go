package service

import (
	"crypto/rand"
	"encoding/binary"
	"testing"
	"time"

	"github.com/hy06ix/onet"
	"github.com/hy06ix/onet/log"
	"github.com/hy06ix/onet/network"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/tbls"
	"go.dedis.ch/kyber/v3/util/random"
)

type networkSuite struct {
	kyber.Group
	pairing.Suite
}

func newNetworkSuite() *networkSuite {
	return &networkSuite{
		Group: Suite.G2(),
		Suite: Suite,
	}
}

func TestSharding(t *testing.T) {
	suite := newNetworkSuite()
	test := onet.NewTCPTest(suite)
	defer test.CloseAll()

	// set topology for inter-shard communications

	done := make(chan bool)

	shardNum := 2

	// Need to increase
	// interShardNum := 1

	interShard := make([]*network.ServerIdentity, shardNum)

	// for i := 0; i < shardNum; i++ {
	// 	interShard[i] = make([]*network.ServerIdentity, interShardNum)
	// }

	for i := 0; i < shardNum; i++ {
		go RunConcordia(t, test, i, interShard)
	}
	<-done

}

func RunConcordia(t *testing.T, test *onet.LocalTest, shardID int, interShard []*network.ServerIdentity) {
	log.Lvl1("Starting test")
	// log.Lvl1(shardID)

	// suite := newNetworkSuite()
	// test := onet.NewTCPTest(suite)
	// defer test.CloseAll()

	// Number of nodes
	n := 10

	// nshard := 2
	// servers := make([][]*onet.Server, nshard)
	// roster := make([]*onet.Roster, nshard)

	// for i := 0; i < nshard; i++ {
	// }
	// servers[0], roster[0], _ = test.GenTree(n, true)
	servers, roster, _ := test.GenTree(n, true)

	shares, public := dkg(n/2, n)
	_, commits := public.Info()
	concordias := make([]*Concordia, n)
	for i := 0; i < n; i++ {
		c := &Config{
			Roster:            roster,
			Index:             i,
			N:                 n,
			Threshold:         n / 2,
			CommunicationMode: 1,
			GossipTime:        150,
			GossipPeers:       3,
			Public:            commits,
			Share:             shares[i], // i have to check this..
			BlockSize:         10000000,
			MaxRoundLoops:     4,
			RoundsToSimulate:  10,
			ShardID:           shardID,
			InterShard:        interShard,
		}
		concordias[i] = servers[i].Service(Name).(*Concordia)
		concordias[i].SetConfig(c)
	}

	// Need to fix - only one si for communicate about header, proof
	// Enroll first si
	interShard[shardID] = concordias[0].c.Roster.List[0]

	done := make(chan bool)
	cb := func(r int, shardID int) {
		if r > 10 {
			done <- true
		}
	}

	// println("--------------------")
	// for i := 0; i < len(interShard); i++ {
	// 	println(interShard[i])
	// }
	// println("--------------------")
	// log.Lvl1(concordias[0].c.Roster)

	concordias[0].AttachCallback(cb)
	time.Sleep(time.Duration(1) * time.Second)
	go concordias[0].Start()
	<-done
	log.Lvl1("finish")

}

func dkg(t, n int) ([]*share.PriShare, *share.PubPoly) {
	allShares := make([][]*share.PriShare, n)
	var public *share.PubPoly
	for i := 0; i < n; i++ {
		priPoly := share.NewPriPoly(G2, t, nil, random.New())
		allShares[i] = priPoly.Shares(n)
		if public == nil {
			public = priPoly.Commit(G2.Point().Base())
			continue
		}
		public, _ = public.Add(priPoly.Commit(G2.Point().Base()))
	}
	shares := make([]*share.PriShare, n)
	for i := 0; i < n; i++ {
		v := G2.Scalar().Zero()
		for j := 0; j < n; j++ {
			v = v.Add(v, allShares[j][i].V)
		}
		shares[i] = &share.PriShare{I: i, V: v}
	}
	return shares, public

}

func benchmarkNotarize(b *testing.B, netSize int, blockSize int) {
	suite := newNetworkSuite()

	blob := make([]byte, blockSize)
	rand.Read(blob)
	hash := rootHash(blob)
	header := &BlockHeader{
		Round:      1,
		Owner:      1,
		Root:       hash,
		Randomness: binary.BigEndian.Uint32([]byte("3605ff73b6faec27aa78e311603e9fe2ef35bad82ccf46fc707814bfbdcc6f9e")),
		PrvHash:    "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447",
		PrvSig:     []byte("3605ff73b6faec27aa78e311603e9fe2ef35bad82ccf46fc707814bfbdcc6f9e"),
	}
	msg := []byte(header.Hash())
	n := netSize
	t := n/2 + 1
	//secret := suite.G1().Scalar().Pick(suite.RandomStream())
	priPoly := share.NewPriPoly(suite.G2(), t, nil, random.New())
	pubPoly := priPoly.Commit(suite.G2().Point().Base())

	Sigs := make(map[int]*PartialSignature)
	sigShares := make([][]byte, 0)
	for i, x := range priPoly.Shares(n) {
		sig, err := tbls.Sign(suite, x, msg)
		require.Nil(b, err)
		partial := &PartialSignature{
			Partial: sig,
		}
		sigShares = append(sigShares, sig)
		Sigs[i] = partial
	}

	// start
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arr := make([][]byte, 0, t)
		for _, val := range Sigs {
			arr = append(arr, val.Partial)
		}
		hash := header.Hash()
		//_, err := tbls.Recover(suite, pubPoly, []byte(hash), arr, t, n)
		_, err := Recover(pubPoly, []byte(hash), arr, t, n)
		require.Nil(b, err)
	}
}

func benchmarkVerifyPartialSignature(b *testing.B, netSize int) {
	suite := newNetworkSuite()

	blob := make([]byte, 1024)
	rand.Read(blob)
	hash := rootHash(blob)
	header := &BlockHeader{
		Round:      1,
		Owner:      1,
		Root:       hash,
		Randomness: binary.BigEndian.Uint32([]byte("3605ff73b6faec27aa78e311603e9fe2ef35bad82ccf46fc707814bfbdcc6f9e")),
		PrvHash:    "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447",
		PrvSig:     []byte("3605ff73b6faec27aa78e311603e9fe2ef35bad82ccf46fc707814bfbdcc6f9e"),
	}
	msg := []byte(header.Hash())
	n := netSize
	t := n/2 + 1
	//secret := suite.G1().Scalar().Pick(suite.RandomStream())
	priPoly := share.NewPriPoly(suite.G2(), t, nil, random.New())
	pubPoly := priPoly.Commit(suite.G2().Point().Base())

	Sigs := make(map[int]*PartialSignature)
	sigShares := make([][]byte, 0)
	for i, x := range priPoly.Shares(n) {
		sig, err := tbls.Sign(suite, x, msg)
		require.Nil(b, err)
		partial := &PartialSignature{
			Partial: sig,
		}
		sigShares = append(sigShares, sig)
		Sigs[i] = partial
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < t; j++ {
			_, err := tbls.SigShare(Sigs[j].Partial).Index()
			require.Nil(b, err)

			err = tbls.Verify(suite, pubPoly, []byte(msg), Sigs[j].Partial)
			require.Nil(b, err)
		}
	}
}

/*
func BenchmarkNotarize_200_1MB(b *testing.B) {benchmarkNotarize(b, 200, 1048576) }
func BenchmarkNotarize_400_1MB(b *testing.B) {benchmarkNotarize(b, 400, 1048576) }
func BenchmarkNotarize_600_1MB(b *testing.B) {benchmarkNotarize(b, 600, 1048576) }
func BenchmarkNotarize_800_1MB(b *testing.B) {benchmarkNotarize(b, 800, 1048576) }
func BenchmarkNotarize_1000_1MB(b *testing.B) {benchmarkNotarize(b, 1000, 1048576) }
*/

func BenchmarkNotarize_1000_1kB(b *testing.B) { benchmarkNotarize(b, 1000, 1024) }

//func BenchmarkVerify(b *testing.B) {benchmarkVerifyPartialSignature(b, 1000)}
