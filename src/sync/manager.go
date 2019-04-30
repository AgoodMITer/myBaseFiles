package sync

import (
	"sync"
	"github.com/mmpei/janus/src/config"
	"time"
	"net/http"
	"fmt"
	"io/ioutil"
	log "github.com/sirupsen/logrus"
	"github.com/mmpei/janus/src/model"
	"encoding/json"
)

type EndpointInfo struct {
	Master bool
}

type MonitorManager struct {
	sync.Mutex
	monitor Monitor
	config  *config.SyncConfig

	// store whether ep is a master
	epStatus map[string]bool
	hookFunc func(peerId string, master bool)

	stopped bool
	stopChan map[string]chan bool
}

func NewMonitorManager(endpoints []string, proxiedPort int, monitorConfig *config.SyncConfig) *MonitorManager {
	return &MonitorManager{
		monitor: *NewMonitor(endpoints, proxiedPort, monitorConfig),
		stopChan: make(map[string]chan bool, len(endpoints)),
		stopped: true,
		epStatus: make(map[string]bool, len(endpoints)),
		config: monitorConfig,
	}
}

func (mm *MonitorManager) Get(peerId string) *model.PeerInfo {
	return mm.monitor.Get(peerId)
}

func (mm *MonitorManager) Run() error {
	client := http.Client{
		Timeout: time.Duration(mm.config.Timeout)*time.Second,
	}
	peers := mm.monitor.GetAll()
	for index := range peers {
		peer := peers[index]
		if _, ok := mm.stopChan[peer.PeerId]; !ok {
			mm.stopChan[peer.PeerId] = make(chan bool, 1)
		}
		go func() {
			ticker := time.NewTicker(time.Duration(mm.config.Interval)*time.Second)
			stop := mm.stopChan[peer.PeerId]
			defer ticker.Stop()
Loop:
			for {
				select {
				case <-ticker.C:
					url := fmt.Sprintf("http://%s%s", peer.PeerAddr, mm.config.URL)
					log.Debugf("monitoring manager : %s", url)
					request, err := http.NewRequest("GET", url, nil)
					if err != nil {
						log.Errorf("monitoring manager generate http request error: %v", err)
						continue
					}
					resp, err := client.Do(request)
					if err != nil { // set remote peer failed
						mm.monitor.Tick(peer.PeerId, false)
						log.Errorf("monitoring manager http failed: %v", err)
						continue
					}
					if resp.StatusCode != http.StatusOK {
						mm.monitor.Tick(peer.PeerId, false)
						log.Errorf("monitoring manager http failed: response code = %d", resp.StatusCode)
						continue
					}
					body, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						mm.monitor.Tick(peer.PeerId, false)
						log.Errorf("monitoring manager http failed: read body failed %v", err)
						continue
					}

					respInfo := &EndpointInfo{}
					if err := json.Unmarshal(body, respInfo); err != nil {
						mm.monitor.Tick(peer.PeerId, false)
						log.Errorf("monitoring manager http failed: decode response body failed %v", err)
						continue
					}
					mm.monitor.Tick(peer.PeerId, true)
					mm.CheckEPStatus(peer.PeerId, respInfo)
				case <-stop:
					log.Infof("stop monitor %s", peer.PeerId)
					break Loop
				}
			}
		}()
	}
	return nil
}

func (mm *MonitorManager) Stop() error {
	mm.Lock()
	defer mm.Unlock()

	if mm.stopped {
		log.Warningf("endpoints monitoring already in stopped status.")
		return nil
	}
	for _, v := range mm.stopChan {
		v<-true
	}
	mm.stopped = true
	return nil
}

func (mm *MonitorManager) Start() error {
	mm.Lock()
	defer mm.Unlock()
	if !mm.stopped {
		log.Warningf("endpoints monitoring already in running status.")
		return nil
	}
	mm.stopped = false
	return mm.Run()
}

func (mm *MonitorManager) SetHealthHookFunc(f func(peerId string)) {
	mm.monitor.hookFunc = f
}

func (mm *MonitorManager) SetStatusHookFunc(f func(peerId string, master bool)) {
	mm.hookFunc = f
}

func (mm *MonitorManager) CheckEPStatus(peerId string, epInfo *EndpointInfo) {
	master := epInfo.Master
	ms, ok := mm.epStatus[peerId]
	if (!ok || ms != master) && mm.monitor.IsHealth(peerId) {
		mm.epStatus[peerId] = master
		go mm.hookFunc(peerId, master)
		return
	}
}

func (mm *MonitorManager) SetEPStatus(peerId string, master bool) {
	mm.Lock()
	defer mm.Unlock()
	mm.epStatus[peerId] = master
}

func (mm *MonitorManager) GetHealthy() []*model.PeerInfo {
	return mm.monitor.GetHealthy()
}
