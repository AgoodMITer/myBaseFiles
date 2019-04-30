package sync

import (
	"net/http"
	"time"
	"fmt"
	"bytes"
	"io/ioutil"
	"github.com/mmpei/janus/src/config"
	"github.com/mmpei/janus/src/model"
	"sync"
	log "github.com/sirupsen/logrus"
	"encoding/json"
)

type ElectPeer struct {
	PeerId string
	Time time.Time
	Type string

	// the id of master of endpoints, only send when i am master
	EPMasterId string
}

type SyncManager struct {
	lock sync.Mutex
	client http.Client
	singleMonitor Monitor

	self model.PeerInfo
	master *model.PeerInfo
	electTime time.Time

	initTimes int

	// sentinel, actually syncManager control sentinel
	sentinel *Sentinel
}

func NewSyncManager(selfAddr string, peerAddr string, config *config.SyncConfig, s *Sentinel) *SyncManager {
	return &SyncManager{
		client: http.Client{
			Timeout: time.Duration(config.Timeout)*time.Second,
		},
		singleMonitor: *NewMonitor([]string{peerAddr}, 0, config),
		self: *model.NewPeer(selfAddr, 0),
		sentinel: s,
	}
}

func (sm *SyncManager) Run() {
	ticker := time.NewTicker(time.Duration(config.ProxyConfig.Sync.Interval)*time.Second)
	for range ticker.C {
		sm.Sync()
	}
}

func (sm *SyncManager) Sync() {
	defer sm.handleError()

	electPeer := &ElectPeer{}
	sm.lock.Lock()
	isError := false
	if sm.master == nil && sm.electTime.IsZero() { // init and elect self
		electPeer.Type = model.TypeInit
		electPeer.Time = time.Now()
		electPeer.PeerId = sm.self.PeerId
		sm.electTime = electPeer.Time
	} else if sm.master == nil {
		log.Error("should not sync when election")
		isError = true
	} else {
		electPeer.Type = model.TypeElected
		electPeer.Time = sm.electTime
		electPeer.PeerId = sm.master.PeerId

		// if i am a sentinel master, i should tell slave who is the master of endpoint
		if sm.IsMaster() {
			electPeer.EPMasterId = sm.sentinel.GetMaster()
		}
	}
	sm.lock.Unlock()
	if isError {
		return
	}

	log.Debugf("sync to remote: msg=%+v \n", electPeer)

	remotePeers := sm.singleMonitor.GetHealthy()
	if len(remotePeers) == 0 && (sm.master == nil || sm.master.PeerId != sm.self.PeerId) {
		// remote die, promote self
		log.Infof("no healthy remote peer, elect self")
		sm.lock.Lock()
		if sm.master == nil || (sm.master != nil && sm.master.PeerId != sm.self.PeerId) {
			sm.SetMaster(&sm.self)
			sm.electTime = time.Now()
		}
		sm.lock.Unlock()
		return
	} else if len(remotePeers) == 0 { // remote down and do as check healthy
		remotePeers = sm.singleMonitor.GetAll()
	} else if len(remotePeers) > 1 {
		log.Error("only support one remote peer now")
		return
	}

	// sync with remotePeer
	remotePeer := remotePeers[0]
	url := fmt.Sprintf("http://%s/sync", remotePeer.PeerAddr)
	bytesData, err := json.Marshal(electPeer)
	if err != nil {
		log.Errorf("encode request body error")
		return
	}
	reader := bytes.NewReader(bytesData)
	request, err := http.NewRequest("POST", url, reader)
	if err != nil {
		log.Errorf("generate http request error: %v", err)
		return
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	resp, err := sm.client.Do(request)
	if err != nil { // set remote peer failed
		sm.singleMonitor.Tick(remotePeer.PeerId, false)
		log.Errorf("http failed: %v", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		sm.singleMonitor.Tick(remotePeer.PeerId, false)
		log.Errorf("http failed: response code = %d", resp.StatusCode)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		sm.singleMonitor.Tick(remotePeer.PeerId, false)
		log.Errorf("http failed: read body failed %v", err)
		return
	}

	respPeer := &ElectPeer{}
	if err := json.Unmarshal(body, respPeer); err != nil {
		sm.singleMonitor.Tick(remotePeer.PeerId, false)
		log.Errorf("http failed: decode response body failed %v", err)
		return
	}
	sm.singleMonitor.Tick(remotePeer.PeerId, true)
	sm.Handle(respPeer)
}

func (sm *SyncManager) Handle(respPeer *ElectPeer) {
	if respPeer == nil || len(respPeer.PeerId) == 0 { // exception
		log.Errorf("http failed: empty response body")
		return
	}

	sm.lock.Lock()
	defer sm.lock.Unlock()
	// check endpoint and do hook
	if !sm.IsMaster() {
		sm.sentinel.HookReportMaster(respPeer.EPMasterId)
	}
	if sm.master == nil {
		if sm.self.PeerId == respPeer.PeerId {
			sm.SetMaster(&sm.self)
			sm.electTime = respPeer.Time
		} else if !sm.electTime.IsZero() && sm.electTime.Sub(respPeer.Time) < 0 && respPeer.Type == model.TypeInit {
			log.Errorf("init election show elect the peer doing early")
			return
		} else { // elect remote
			sm.SetMaster(sm.singleMonitor.Get(respPeer.PeerId))
			sm.electTime = respPeer.Time
			if sm.master == nil {
				log.Errorf("remote elected peer not exists")
				return
			}
		}
	} else if sm.master.PeerId != respPeer.PeerId {
		if respPeer.Type == model.TypeElected && !sm.electTime.IsZero() && !respPeer.Time.IsZero() && sm.electTime.Sub(respPeer.Time) > 0 {
			sm.electTime = respPeer.Time
			sm.SetMaster(sm.singleMonitor.Get(respPeer.PeerId))
			if sm.master == nil {
				log.Errorf("remote elected peer not exists")
				return
			}
		} else {
			log.Errorf("waiting for syncing next time")
			return
		}
	}
}

func (sm *SyncManager) Get() *ElectPeer {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	ep := &ElectPeer{}
	if sm.master == nil && !sm.electTime.IsZero() {
		ep.PeerId = sm.self.PeerId
		ep.Type = model.TypeInit
		ep.Time = sm.electTime
	} else if sm.master != nil {
		ep.PeerId = sm.master.PeerId
		ep.Type = model.TypeElected
		ep.Time = sm.electTime

		// if i am a sentinel master, i should tell slave who is the master of endpoint
		if sm.IsMaster() {
			ep.EPMasterId = sm.sentinel.GetMaster()
		}
	} else {
		// error, never reach here
	}
	return ep
}

func (sm *SyncManager) IsMaster() bool {
	if sm.master == nil {
		return false
	}
	return sm.master.PeerId == sm.self.PeerId
}

func (sm *SyncManager) handleError() {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	// checks status and fix unstable
	if sm.master == nil && sm.initTimes < 3 { // init failed, waiting for next time
		sm.electTime = time.Time{}
		sm.initTimes++
	} else if sm.master == nil { // when init failed 3 times, promote to master by self. only one node
		sm.SetMaster(&sm.self)
	}
}

func (sm *SyncManager) SetMaster(peer *model.PeerInfo) {
	sm.master = peer
	// do hook
	sm.sentinel.HookSelfRole(sm.IsMaster())
}

func (sm *SyncManager) GetEPMaster() string {
	return sm.sentinel.GetMaster()
}
