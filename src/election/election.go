package election

import "github.com/mmpei/janus/src/sync"

type Election struct {
}

func NewElection(m *sync.Monitor) *Election {
	e := &Election{
	}
	return e
}

func (e *Election) Elect() {

}

func (e *Election) DoHook(peerId string) {

}
