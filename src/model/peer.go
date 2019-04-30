package model

import (
	"strings"
	"fmt"
)

var Self *PeerInfo

type PeerInfo struct {
	PeerId   string
	// for endpoint this is the control address, not the proxy address
	PeerAddr string
	// proxied address for endpoint, used when do transmit
	ProxiedAddress string
	// continuous fail or success, will reset to 1 if 'success' changed
	Count int
	// record the status of last time
	Success     bool
	//
	Alive bool
}

func NewPeer(peerAddr string, proxiedPort int) *PeerInfo {
	var proxiedAddress string
	if proxiedPort != 0 {
		ipPort := strings.Split(peerAddr, ":")
		proxiedAddress = fmt.Sprintf("http://%s:%d", ipPort[0], proxiedPort)
	}
	return &PeerInfo{
		PeerId: peerAddr,
		PeerAddr: peerAddr,
		ProxiedAddress: proxiedAddress,
		Count: 0,
		Success: true,
		Alive: true,
	}
}

func (pi *PeerInfo) Tick(success bool, count int) {
	if success == pi.Success {
		if count == pi.Count + 1 { // should change when reach threshold
			pi.Alive = success
			pi.Count++
		} else if count > pi.Count + 1 {
			pi.Count++
		}
	} else { // should change when peer status change
		pi.Count = 0
		pi.Success = success
		if count == pi.Count + 1 { // should change when reach threshold
			pi.Alive = success
			pi.Count++
		} else if count > pi.Count + 1 {
			pi.Count++
		}
	}
}
