package sync

import (
	log "github.com/sirupsen/logrus"
	"github.com/mmpei/janus/src/model"
	"fmt"
	"sync"
	"github.com/mmpei/janus/src/config"
	"time"
	"net/http"
)

// do monitoring and control the endpoint status.
type Sentinel struct {
	sync.Mutex
	monitor *MonitorManager
	//election *election.Election

	master string
	onDuty bool
}

func NewSentinel(m *MonitorManager) *Sentinel {
	s := &Sentinel{
		monitor: m,
	}
	s.monitor.SetHealthHookFunc(s.HookEndpointHealth)
	s.monitor.SetStatusHookFunc(s.HookEndpointStatus)
	return s
}

func (s *Sentinel) GetMaster() string {
	return s.master
}

func (s *Sentinel) GetMasterPeer() *model.PeerInfo {
	if len(s.master) == 0 {
		return nil
	}
	return s.monitor.Get(s.master)
}

func (s *Sentinel) HookEndpointHealth(peerId string) {
	log.Infof("health change %s", peerId)
	if !s.onDuty {
		log.Warningf("not on duty, something error")
		return
	}
	s.Lock()
	defer s.Unlock()
	if len(s.master) == 0 {
		//log.Warningf("waiting for init")
		s.Elect()
		return
	}

	if peerId != s.master {
		log.Infof("a slave status changes, do nothing")
		return
	}

	// todo master down, re-elect
	s.Elect()
}

// HookEndpointStatus handles monitoring report status change
func (s *Sentinel) HookEndpointStatus(peerId string, master bool) {
	log.Infof("endpoint status change %s:%t", peerId, master)
	if !s.onDuty {
		log.Warningf("not on duty, should not monitoring by me, something error")
		return
	}

	s.Lock()
	ret := false
	if len(s.master) == 0 {
		if master {
			s.master = peerId
		}
		ret = true
	}
	s.Unlock()
	if ret {
		return
	}

	if master && peerId != s.master { // the master monitored is different from elected
		log.Errorf("there are another master that not my elect %s", peerId)
		// downgrade
		if err := s.changeEPRole(s.monitor.Get(peerId), false); err != nil {
			log.Errorf("downgrade peer %s failed: +v", peerId, err)
		}
	} else if !master && peerId == s.master { // master downgrade to slave
		// upgrade again
		if err := s.changeEPRole(s.monitor.Get(peerId), true); err != nil {
			log.Errorf("upgrade peer %s failed: +v", peerId, err)
		}
	}
}

// HookReportMaster handles sync report status change
func (s *Sentinel) HookReportMaster(peerId string) {
	// i am on duty. not process
	if s.onDuty {
		log.Errorf("i am on duty, ep status changed by other should never happen")
		return
	}

	// sentinel slave, do nothing, just accept
	s.master = peerId
}

// HookSelfRole will take the duty of master sentinel or downgrade to slave
func (s *Sentinel) HookSelfRole(master bool) {
	log.Infof("sentinel role change:%t", master)
	if s.onDuty == master {
		log.Warningf("node role change, now is [%t](true:master, false:slave), but has the same status", master)
		return
	}

	// todo start or stop the duty

	s.onDuty = master
	if s.onDuty {
		s.monitor.Start()
		// wait for monitor sync the endpoint status
		time.Sleep(3*time.Second)
		// check and do election if needed
		s.Lock()
		defer s.Unlock()
		if len(s.master) == 0 { // should init
			s.Elect()
		} else { // only promote to sentinel master, do nothing
			log.Infof("promote to sentinel master, waiter for check status")
			// set monitor status
			s.monitor.SetEPStatus(s.master, true)
			return
		}
	} else {
		s.monitor.Stop()
		// to slave, do nothing
	}
}

// Elect just do elect from healthy endpoints
func (s *Sentinel) Elect() error {
	// do elect and change remote status
	peers := s.monitor.GetHealthy()

	// test select first one as master
	did := false
	for _, peer := range peers {
		err := s.changeEPRole(peer, true)
		if err == nil {
			did = true
			s.master = peer.PeerId
			break
		} else {
			log.Errorf("elect master error: %v", err)
		}
	}
	if !did {
		s.master = ""
	}
	return nil
}

func (s *Sentinel) changeEPRole(peer *model.PeerInfo, master bool) error {
	var u string
	if master {
		u = config.ProxyConfig.ToMaster
	} else {
		u = config.ProxyConfig.ToSlave
	}

	client := http.Client{
		Timeout: 10*time.Second,
	}
	url := fmt.Sprintf("http://%s%s", peer.PeerAddr, u)

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("change ep role failed: %d", resp.StatusCode)
	}
	return nil
}
