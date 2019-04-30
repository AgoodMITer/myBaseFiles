package handler

import (
	"github.com/mmpei/janus/src/model"
	"sync"
)

type PeerManager struct {
	sync.Mutex
	peerMap   map[int]*model.PeerInfo
}

func NewPeerManager() *PeerManager {
	return &PeerManager{
		peerMap: make(map[int]*model.PeerInfo),
	}
}
