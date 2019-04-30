package handler

import (
	"net/http"
	"encoding/json"
	"fmt"
	"github.com/mmpei/janus/src/sync"
	log "github.com/sirupsen/logrus"
)

type Handler struct {
	syncManager *sync.SyncManager
}

func NewHandler(sm *sync.SyncManager) *Handler {
	return &Handler{
		syncManager: sm,
	}
}

func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	req := sync.ElectPeer{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("json decode request: %s \n", err)
		w.WriteHeader(500)
		return
	}

	h.syncManager.Handle(&req)
	res := h.syncManager.Get()
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Errorf("json encode response: %s", err)
	}
}

type Peer struct {
	ElectPeer sync.ElectPeer
	IsMaster  bool
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	res := Peer{
		ElectPeer: *h.syncManager.Get(),
		IsMaster: h.syncManager.IsMaster(),
	}

	res.ElectPeer.EPMasterId = h.syncManager.GetEPMaster()

	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Errorf("json encode response: %s", err)
	}
}
