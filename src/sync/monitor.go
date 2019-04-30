package sync

import (
	"github.com/mmpei/janus/src/model"
	"github.com/mmpei/janus/src/config"
	"sync"
	log "github.com/sirupsen/logrus"
	"fmt"
	"time"
	"net/http"
	"io/ioutil"
)

type Monitor struct {
	sync.Mutex
	peers       map[string]*model.PeerInfo

	config      *config.SyncConfig

	hookFunc    func(peerId string)

	stop 	    chan bool
}

func NewMonitor(peerAddrs []string, proxiedPort int, c *config.SyncConfig) *Monitor {
	peers := make(map[string]*model.PeerInfo)
	for _, peerAddr := range peerAddrs {
		peer := model.NewPeer(peerAddr, proxiedPort)
		peers[peer.PeerId] = peer
	}
	return &Monitor{
		peers: peers,
		config: c,
		stop: make(chan bool),
	}
}

func (m *Monitor) SetHookFunc(f func(peerId string)) {
	m.hookFunc = f
}

func (m *Monitor) Start() {
}

func (m *Monitor) Stop() {
	for range m.peers {
		m.stop <- true
	}
}

// Run monitoring, not use for now
func (m *Monitor) Run() error {
	client := http.Client{
		Timeout: time.Duration(m.config.Timeout)*time.Second,
	}

	for k := range m.peers {
		peer := m.peers[k]
		go func() {
			ticker := time.NewTicker(time.Duration(m.config.Interval)*time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					url := fmt.Sprintf("http://%s/%s", peer.PeerAddr, m.config.URL)
					request, err := http.NewRequest("GET", url, nil)
					if err != nil {
						log.Errorf("monitoring generate http request error: %v", err)
						continue
					}
					resp, err := client.Do(request)
					if err != nil { // set remote peer failed
						m.Tick(peer.PeerId, false)
						log.Errorf("monitoring http failed: %v", err)
						continue
					}
					if m.config.CheckCode && (resp.StatusCode > 299 || resp.StatusCode < 200) {
						m.Tick(peer.PeerId, false)
						log.Errorf("monitoring http failed: response code = %d", resp.StatusCode)
						continue
					}
					ioutil.ReadAll(resp.Body)
					m.Tick(peer.PeerId, true)
				case stop := <-m.stop:
					if stop {
						log.Infof("stop monitoring peer: %s", peer.PeerId)
						return
					}
				}
			}
		}()
	}
	return nil
}

// GetHealthy returns healthy peers
func (m *Monitor) GetHealthy() []*model.PeerInfo {
	m.Lock()
	defer m.Unlock()
	var healthy []*model.PeerInfo
	for id := range m.peers {
		peer := m.peers[id]
		if peer.Alive {
			healthy = append(healthy, peer)
		}
	}
	return healthy
}

// Get
func (m *Monitor) Get(peerId string) *model.PeerInfo {
	return m.peers[peerId]
}

// GetAll
func (m *Monitor) GetAll() []*model.PeerInfo {
	var ps []*model.PeerInfo
	for k := range m.peers {
		ps = append(ps, m.peers[k])
	}
	return ps
}

// Tick used to modify the peer status when monitoring is triggered external
func (m *Monitor) Tick(peerId string, success bool) error {
	peer, ok := m.peers[peerId]
	if !ok {
		log.Errorf("could not find the specified peer")
		return fmt.Errorf("PeerNotFoundError")
	}

	m.Lock()
	defer m.Unlock()

	alive := peer.Alive
	peer.Tick(success, m.getCount(success))
	if alive != peer.Alive && m.hookFunc != nil { // do hooking
		go m.hookFunc(peerId)
	}

	return nil
}

func (m *Monitor) IsHealth(peerId string) bool {
	peer, ok := m.peers[peerId]
	if !ok {
		return false
	}
	return peer.Alive
}

func (m *Monitor) getCount(success bool) int {
	if success {
		return m.config.Recover.Count
	} else {
		return m.config.Failure.Count
	}
}
