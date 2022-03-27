package simulation

import (
	"time"

	"github.com/BurntSushi/toml"
	"github.com/csanti/onet"
	"github.com/csanti/onet/log"
	"github.com/csanti/onet/network"
	"github.com/csanti/onet/simul/monitor"
	concordia "github.com/hy06ix/myprotocol/service"
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
}

type Simulation struct {
	onet.SimulationBFTree
	config
	BlockSize int // in bytes
}

func NewSimulation(config string) (onet.Simulation, error) {
	s := &Simulation{}
	_, err := toml.Decode(config, s)
	return s, err
}

func (s *Simulation) Setup(dir string, hosts []string) (*onet.SimulationConfig, error) {
	sim := new(onet.SimulationConfig)
	s.CreateRoster(sim, hosts, 2000)
	s.CreateTree(sim)
	log.Lvlf1("-------------------------%s", sim.Roster.List)
	// create the shares manually
	return sim, nil
}

func (s *Simulation) DistributeConfig(config *onet.SimulationConfig) {
	interShard := make([][]*network.ServerIdentity, 1)
	for i := 0; i < 1; i++ {
		interShard[i] = make([]*network.ServerIdentity, 1)
	}

	shares, public := dkg(s.Threshold, s.Hosts)
	n := len(config.Roster.List)
	_, commits := public.Info()

	for i, si := range config.Roster.List {
		c := &concordia.Config{
			Roster:            config.Roster,
			Index:             i,
			N:                 n,
			Threshold:         s.Threshold,
			CommunicationMode: s.CommunicationMode,
			GossipTime:        s.GossipTime,
			GossipPeers:       s.GossipPeers,
			Public:            commits,
			Share:             shares[i],
			BlockSize:         s.BlockSize,
			MaxRoundLoops:     s.MaxRoundLoops,
			RoundsToSimulate:  s.Rounds,
			ShardID:           0, // for test
			// InterShard:        interShard,
		}
		if i == 0 {
			config.GetService(concordia.Name).(*concordia.Concordia).SetConfig(c)
		} else {
			config.Server.Send(si, c)
		}
	}
}

func (s *Simulation) Run(config *onet.SimulationConfig) error {

	log.Lvl1("distributing config to all nodes...")
	s.DistributeConfig(config)
	log.Lvl1("Sleeping for the config to dispatch correctly")
	time.Sleep(3 * time.Second)
	log.Lvl1("Starting concordia simulation")
	concordia := config.GetService(concordia.Name).(*concordia.Concordia)

	var roundDone int
	done := make(chan bool)
	var fullRound *monitor.TimeMeasure
	newRoundCb := func(round int) {
		fullRound.Record()
		roundDone++
		log.Lvl1("Simulation Round #", round, "finished")
		if roundDone > s.Rounds {
			done <- true
		} else {
			fullRound = monitor.NewTimeMeasure("fullRound")
		}
	}

	concordia.AttachCallback(newRoundCb)
	fullTime := monitor.NewTimeMeasure("finalizing")
	fullRound = monitor.NewTimeMeasure("fullRound")
	concordia.Start()
	select {
	case <-done:
		break
	}
	fullTime.Record()
	monitor.RecordSingleMeasure("blocks", float64(roundDone))
	monitor.RecordSingleMeasure("avgRound", fullTime.Wall.Value/float64(s.Rounds))
	log.Lvl1(" ---------------------------")
	log.Lvl1("End of simulation => ", roundDone, " rounds done")
	log.Lvl1("Last full round = ", fullRound.Wall.Value)
	log.Lvl1("Total time = ", fullTime.Wall.Value)
	log.Lvl1("Avg round = ", fullTime.Wall.Value/float64(s.Rounds))
	log.Lvl1(" ---------------------------")
	return nil
}
