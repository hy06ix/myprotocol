package simulation

import (
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	concordia "github.com/hy06ix/myprotocol/service"
	"github.com/hy06ix/onet"
	"github.com/hy06ix/onet/log"
	"github.com/hy06ix/onet/network"
	"github.com/hy06ix/onet/simul/monitor"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
)

// Name is the name of the simulation
var Name = "concordia"

func init() {
	onet.SimulationRegister(Name, NewSimulation)
}

// config being passed down to all nodes, each one takes the relevant
// information
type config struct {
	Seed              int64
	Threshold         int // threshold of the threshold sharing scheme
	BlockSize         int // the size of the block in bytes
	BlockTime         int // blocktime in seconds
	GossipTime        int
	GossipPeers       int // number of neighbours that each node will gossip messages to
	CommunicationMode int // 0 for broadcast, 1 for gossip
	MaxRoundLoops     int // maximum times a node can loop on a round before alerting
	ShardNum          int // Number of shards
}

type Simulation struct {
	onet.SimulationBFTree
	config
	BlockSize     int // in bytes
	AdditionalBit int
}

// type ShardSimulationConfig struct {
// 	onet.SimulationConfig

// 	configs []*onet.SimulationConfig
// }

func NewSimulation(config string) (onet.Simulation, error) {
	s := &Simulation{}
	_, err := toml.Decode(config, s)
	return s, err
}

func (s *Simulation) Setup(dir string, hosts []string) (*onet.SimulationConfig, error) {
	s.AdditionalBit = 33
	log.Lvl1(s)
	log.LLvl1(&s)

	sim := new(onet.SimulationConfig)
	// log.Lvl1("len:", len(hosts))
	s.CreateRoster(sim, hosts, 2000)
	s.CreateTree(sim)

	// sim := new(ShardSimulationConfig)
	// sim.configs = make([]*onet.SimulationConfig, s.ShardNum)

	// for i := 0; i < s.ShardNum; i++ {
	// 	tmp := new(onet.SimulationConfig)
	// 	s.CreateRoster(tmp, hosts, 2000)
	// 	s.CreateTree(tmp)

	// 	sim.configs[i] = tmp
	// 	// log.Lvlf1("-------------------------sim%d.Roster.List: %s", i, tmp.Roster.List)
	// 	// sim = tmp
	// 	sim.SimulationConfig = *tmp
	// }

	// test := (*onet.SimulationConfig)(unsafe.Pointer(&sim))

	// log.Lvlf1("-------------------------s: %s", &s)
	// log.Lvlf1("-------------------------sim: %s", &sim)
	// log.Lvlf1("-------------------------s.BlockSize: %s", s.BlockSize)
	// log.Lvlf1("-------------------------s.configs: %s", s.configs)
	// log.Lvlf1("-------------------------s.configs[0].Roster.List: %s", sim.configs[0].Roster.List)
	// log.Lvlf1("-------------------------s.configs[1].Roster.List: %s", sim.configs[1].Roster.List)

	// log.Lvlf1("-------------------------sim.Tree.Root.List(): %s", sim.configs[0].Tree.Roster.List)
	// log.Lvlf1("-------------------------sim.Tree.Root: %s", sim.Tree.Root)
	// log.Lvlf1("-------------------------sim.Tree.Roster: %s", sim.Tree.Roster)
	// log.Lvlf1("-------------------------sim.Tree.Roster.List: %s", sim.Tree.Roster.List)
	// create the shares manually
	return sim, nil
}

// func (s *Simulation) DistributeConfig(config *onet.SimulationConfig, shardID int, interShard []*network.ServerIdentity) {
func (s *Simulation) DistributeConfig(config *onet.SimulationConfig) (nl []*concordia.Node) {
	// n := len(config.Roster.List)

	shardSize := s.Hosts / s.ShardNum
	// shares, public := dkg(s.Threshold, s.Hosts)
	// _, commits := public.Info()

	nodeList := make([]*concordia.Node, s.ShardNum)
	interShard := make([]*network.ServerIdentity, s.ShardNum)

	rosters := make([]*onet.Roster, s.ShardNum)
	shareList := make([][]*share.PriShare, s.ShardNum)
	publicList := make([]*share.PubPoly, s.ShardNum)
	commitList := make([][]kyber.Point, s.ShardNum)

	for i := 0; i < s.ShardNum; i++ {
		rosters[i] = onet.NewRoster(config.Roster.List[i*shardSize : (i+1)*shardSize])

		shareList[i], publicList[i] = dkg(s.Threshold, s.BF+1)
		_, commitList[i] = publicList[i].Info()
	}

	for i, si := range config.Roster.List {
		c := &concordia.Config{
			Roster:            rosters[i/shardSize],
			Index:             i % shardSize,
			N:                 shardSize,
			Threshold:         s.Threshold,
			CommunicationMode: s.CommunicationMode,
			GossipTime:        s.GossipTime,
			GossipPeers:       s.GossipPeers,
			Public:            commitList[i/shardSize],
			Share:             shareList[i/shardSize][i%shardSize],
			BlockSize:         s.BlockSize,
			MaxRoundLoops:     s.MaxRoundLoops,
			RoundsToSimulate:  s.Rounds,
			ShardID:           i / shardSize,
			InterShard:        interShard,
		}

		// log.Lvlf1("-----------------%s", shardSize)

		if i%shardSize == 0 {
			interShard[i/shardSize] = config.Roster.List[i]
			nodeList[i/shardSize] = config.GetService(concordia.Name).(*concordia.Concordia).SetConfig(c)
		} else {
			config.Server.Send(si, c)
		}
	}
	return nodeList
}

func (s *Simulation) Run(config *onet.SimulationConfig) error {
	log.Lvl1(s)
	log.LLvl1(&s)
	// log.Lvlf1("-------------------------s: %s", &s)
	// log.Lvlf1("-------------------------config: %s", &config)
	// log.Lvlf1("-------------------------s.BlockSize: %s", s.BlockSize)
	// log.Lvlf1("-------------------------s.configs: %s", s.configs)
	log.Lvlf1("-------------------------config.Roster.List: %s", config.Roster.List)
	// sim = shardConfig(config)
	// sim := (*ShardSimulationConfig)(unsafe.Pointer(&config))

	// configs := make([]*onet.SimulationConfig, s.ShardNum)

	// log.Lvl1(config)
	// interShard := make([]*network.ServerIdentity, s.ShardNum)

	log.Lvl1("distributing config to all nodes...")
	nodeList := s.DistributeConfig(config)

	// for i := 0; i < s.ShardNum; i++ {
	// 	s.DistributeConfig(sim.configs[i], i, interShard)
	// }

	log.Lvl1("Sleeping for the config to dispatch correctly")
	time.Sleep(3 * time.Second)
	log.Lvl1("Starting concordia simulation")

	// concordias := make([]*concordia.Concordia, s.ShardNum)
	concordias := make([]*concordia.Concordia, s.ShardNum)

	for i, node := range nodeList {
		log.Lvl1(node)
		concordia := config.GetService(concordia.Name).(*concordia.Concordia)
		concordia.SetNode(node)
		concordias[i] = concordia
		concordias[i].GetInfo()
	}

	// log.Lvl1(concordia.Context)
	// for i := 0; i < s.ShardNum; i++ {
	// 	concordias[i] = concordia
	// }

	// concordias := make([]*concordia.Concordia, s.ShardNum)
	// concordia := config.GetService(concordia.Name).(*concordia.Concordia)

	// Deep copy
	// rosters := make([]*onet.Roster, s.ShardNum)
	// shardSize := len(config.Roster.List) / s.ShardNum

	// for i := 0; i < s.ShardNum; i++ {
	// 	buf := new(bytes.Buffer)
	// 	gob.NewEncoder(buf).Encode(concordia)
	// 	gob.NewDecoder(buf).Decode(concordias[i])

	// 	// var identities []network.ServerIdentity
	// 	identities := make([]*network.ServerIdentity, shardSize)
	// 	for j := 0; j < shardSize; j++ {
	// 		// buf2 := new(bytes.Buffer)
	// 		// gob.NewEncoder(buf2).Encode(config.Roster.List[i*shardSize+j])
	// 		// gob.NewDecoder(buf2).Decode(identities[j])
	// 		identities[j] = config.Roster.List[i*shardSize+j]
	// 	}

	// 	rosters[i] = onet.NewRoster(identities)
	// }

	// // Reconstruct config
	// shardSize := len(config.Roster.List) / s.ShardNum
	// configs := make([]*onet.SimulationConfig, s.ShardNum)

	// interShard := make([]*network.ServerIdentity, s.ShardNum)
	// for i := 0; i < s.ShardNum; i++ {
	// 	configs[i] = new(onet.SimulationConfig)
	// 	tmpServerList := make([]*network.ServerIdentity, shardSize)
	// 	configs[i].PrivateKeys = make(map[network.Address]*onet.SimulationPrivateKey)
	// 	for j := 0; j < shardSize; j++ {
	// 		tmpServerList[j] = config.Roster.List[i*shardSize+j]
	// 		configs[i].PrivateKeys[tmpServerList[j].Address] = config.PrivateKeys[tmpServerList[j].Address]
	// 	}
	// 	configs[i].Roster = onet.NewRoster(tmpServerList)
	// 	configs[i].Tree = onet.NewTree(configs[i].Roster, onet.NewTreeNode(0, configs[i].Roster.List[0]))

	// 	// if i == 0 {
	// 	// 	configs[i].Server = config.Server
	// 	// } else {
	// 	// 	suite := &edwards25519.SuiteEd25519{}
	// 	// 	configs[i].Server = onet.NewServerTCP(configs[i].Roster.List[0], suite)
	// 	// }

	// 	// Need to change Server
	// 	// configs[i].Server = onet.NewServerTCP(configs[i].Roster.List[0], bn256.NewSuite())
	// 	configs[i].Server = config.Server
	// 	configs[i].Overlay = onet.NewOverlay(configs[i].Server)
	// 	configs[i].Config = config.Config

	// 	interShard[i] = configs[i].Roster.List[0]

	// 	// log.Lvl1(configs[i])

	// 	log.Lvlf1("distributing config to all nodes in Shard %d...", i)
	// 	s.DistributeConfig(configs[i], i, interShard)
	// }

	// log.Lvl1("Sleeping for the config to dispatch correctly")
	// time.Sleep(3 * time.Second)
	// log.Lvl1("Starting concordia simulation")

	// log.Lvl1(reflect.TypeOf(configs[0].GetService(concordia.Name).(*concordia.Concordia)))
	// concordias := make([]*concordia.Concordia, s.ShardNum)
	// for i := 0; i < s.ShardNum; i++ {
	// 	concordias[i] = configs[i].GetService(concordia.Name).(*concordia.Concordia)
	// 	log.Lvl1(concordias[i])
	// }

	// log.Lvl1("Sleeping for the config to dispatch correctly")
	// time.Sleep(5 * time.Second)
	// log.Lvl1("Starting concordia simulation")
	// concordia := config.GetService(concordia.Name).(*concordia.Concordia)

	// var roundDone int
	// done := make(chan bool)
	// var fullRound *monitor.TimeMeasure
	// newRoundCb := func(round int, shardNum int) {
	// 	fullRound.Record()
	// 	roundDone++
	// 	log.Lvl1("Simulation Round #", round, "finished")
	// 	if roundDone > s.Rounds {
	// 		done <- true
	// 	} else {
	// 		fullRound = monitor.NewTimeMeasure("fullRound")
	// 	}
	// }

	roundDone := make([]int, s.ShardNum)
	done := make(chan bool)
	fullRound := make([]*monitor.TimeMeasure, s.ShardNum)

	newRoundCb := func(round int, shardID int) {
		fullRound[shardID].Record()
		roundDone[shardID]++
		log.Lvl1("Simulation Round #", round, " in Shard #", shardID, "finished")
		if roundDone[shardID] > s.Rounds {
			done <- true
		} else {
			fullRound[shardID] = monitor.NewTimeMeasure("fullRound Shard " + strconv.Itoa(shardID))
		}
	}

	fullTimes := make([]*monitor.TimeMeasure, s.ShardNum)
	for i := 0; i < s.ShardNum; i++ {
		concordias[i].AttachCallback(newRoundCb)
		fullTimes[i] = monitor.NewTimeMeasure("finalizing Shard " + strconv.Itoa(i))
		fullRound[i] = monitor.NewTimeMeasure("fullRound Shard " + strconv.Itoa(i))
		go concordias[i].Start()
		//concordias[i].Start()
	}

	// concordia.AttachCallback(newRoundCb)
	// fullTime := monitor.NewTimeMeasure("finalizing")
	// fullRound = monitor.NewTimeMeasure("fullRound")
	// concordia.Start()

	for i := 0; i < s.ShardNum; i++ {
		// select {
		// case <-done:
		// 	continue
		// }
		<-done
	}

	// select {
	// case <-done:
	// 	break
	// }

	// fullTime.Record()
	// monitor.RecordSingleMeasure("blocks", float64(roundDone))
	// monitor.RecordSingleMeasure("avgRound", fullTime.Wall.Value/float64(s.Rounds))
	// log.Lvl1(" ---------------------------")
	// log.Lvl1("End of simulation => ", roundDone, " rounds done")
	// log.Lvl1("Last full round = ", fullRound.Wall.Value)
	// log.Lvl1("Total time = ", fullTime.Wall.Value)
	// log.Lvl1("Avg round = ", fullTime.Wall.Value/float64(s.Rounds))
	// log.Lvl1(" ---------------------------")

	for i := 0; i < s.ShardNum; i++ {
		fullTimes[i].Record()
		monitor.RecordSingleMeasure("blocks in Shard "+strconv.Itoa(i), float64(roundDone[i]))
		monitor.RecordSingleMeasure("avgRound", fullTimes[i].Wall.Value/float64(s.Rounds))
		log.Lvl1(" ---------------------------")
		log.Lvl1("End of simulation => ", roundDone, " rounds done")
		log.Lvl1("Last full round = ", fullRound[i].Wall.Value)
		log.Lvl1("Total time = ", fullTimes[i].Wall.Value)
		log.Lvl1("Avg round = ", fullTimes[i].Wall.Value/float64(s.Rounds))
		log.Lvl1(" ---------------------------")
	}

	return nil
}
