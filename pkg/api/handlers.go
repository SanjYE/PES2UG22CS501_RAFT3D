package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/raft3d/internal/fsm"
	"github.com/raft3d/pkg/models"
	"github.com/raft3d/pkg/raft"
)

// Handler manages HTTP requests for the Raft service
type Handler struct {
	raftServer *raft.Server
	fsm        *fsm.FSM
}

func NewHandler(raftServer *raft.Server, fsm *fsm.FSM) *Handler {
	return &Handler{
		raftServer: raftServer,
		fsm:        fsm,
	}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/v1/printers", h.CreatePrinter).Methods("POST")
	router.HandleFunc("/api/v1/printers", h.ListPrinters).Methods("GET")
	router.HandleFunc("/api/v1/printers/{id}", h.GetPrinter).Methods("GET")

	router.HandleFunc("/api/v1/filaments", h.CreateFilament).Methods("POST")
	router.HandleFunc("/api/v1/filaments", h.ListFilaments).Methods("GET")
	router.HandleFunc("/api/v1/filaments/{id}", h.GetFilament).Methods("GET")

	router.HandleFunc("/api/v1/print_jobs", h.CreatePrintJob).Methods("POST")
	router.HandleFunc("/api/v1/print_jobs", h.ListPrintJobs).Methods("GET")
	router.HandleFunc("/api/v1/print_jobs/{id}", h.GetPrintJob).Methods("GET")
	router.HandleFunc("/api/v1/print_jobs/{id}/status", h.UpdatePrintJobStatus).Methods("POST")

	router.HandleFunc("/api/v1/status", h.GetNodeStatus).Methods("GET")
}

func (h *Handler) isLeader(w http.ResponseWriter) bool {
	if !h.raftServer.IsLeader() {
		leaderAddr := h.raftServer.LeaderAddr()
		http.Error(w, fmt.Sprintf("not the leader, please redirect to: %s", leaderAddr), http.StatusTemporaryRedirect)
		return false
	}
	return true
}

func (h *Handler) applyCommand(cmd *fsm.Command) (interface{}, error) {
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %v", err)
	}

	return h.raftServer.Apply(data, 5*time.Second)
}

func (h *Handler) CreatePrinter(w http.ResponseWriter, r *http.Request) {
	if !h.isLeader(w) {
		return
	}

	var printer models.Printer
	if err := json.NewDecoder(r.Body).Decode(&printer); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if err := printer.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	printerData, err := json.Marshal(printer)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to marshal printer data: %v", err), http.StatusInternalServerError)
		return
	}

	cmd := fsm.Command{
		Op:         fsm.OpCreate,
		EntityType: fsm.EntityPrinter,
		Payload:    printerData,
	}

	_, err = h.applyCommand(&cmd)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create printer: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(printer)
}

func (h *Handler) ListPrinters(w http.ResponseWriter, r *http.Request) {
	printers := h.fsm.Store().GetPrinters()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(printers)
}

func (h *Handler) GetPrinter(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	printer, found := h.fsm.Store().GetPrinter(id)
	if !found {
		http.Error(w, "printer not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(printer)
}

func (h *Handler) CreateFilament(w http.ResponseWriter, r *http.Request) {
	if !h.isLeader(w) {
		return
	}

	var filament models.Filament
	if err := json.NewDecoder(r.Body).Decode(&filament); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if err := filament.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filamentData, err := json.Marshal(filament)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to marshal filament data: %v", err), http.StatusInternalServerError)
		return
	}

	cmd := fsm.Command{
		Op:         fsm.OpCreate,
		EntityType: fsm.EntityFilament,
		Payload:    filamentData,
	}

	_, err = h.applyCommand(&cmd)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create filament: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(filament)
}

func (h *Handler) ListFilaments(w http.ResponseWriter, r *http.Request) {
	filaments := h.fsm.Store().GetFilaments()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filaments)
}

func (h *Handler) GetFilament(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	filament, found := h.fsm.Store().GetFilament(id)
	if !found {
		http.Error(w, "filament not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filament)
}

func (h *Handler) CreatePrintJob(w http.ResponseWriter, r *http.Request) {
	if !h.isLeader(w) {
		return
	}

	var printJob models.PrintJob
	if err := json.NewDecoder(r.Body).Decode(&printJob); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	printJob.Status = models.StatusQueued

	printJobData, err := json.Marshal(printJob)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to marshal print job data: %v", err), http.StatusInternalServerError)
		return
	}

	cmd := fsm.Command{
		Op:         fsm.OpCreate,
		EntityType: fsm.EntityPrintJob,
		Payload:    printJobData,
	}

	result, err := h.applyCommand(&cmd)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create print job: %v", err), http.StatusInternalServerError)
		return
	}

	if result != nil {
		if errResult, ok := result.(error); ok {
			http.Error(w, errResult.Error(), http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(printJob)
}

func (h *Handler) ListPrintJobs(w http.ResponseWriter, r *http.Request) {
	printJobs := h.fsm.Store().GetPrintJobs()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(printJobs)
}

func (h *Handler) GetPrintJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	printJob, found := h.fsm.Store().GetPrintJob(id)
	if !found {
		http.Error(w, "print job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(printJob)
}

func (h *Handler) UpdatePrintJobStatus(w http.ResponseWriter, r *http.Request) {
	if !h.isLeader(w) {
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var statusUpdate struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&statusUpdate); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	_, found := h.fsm.Store().GetPrintJob(id)
	if !found {
		http.Error(w, "print job not found", http.StatusNotFound)
		return
	}

	statusChange := fsm.PrintJobStatusChange{
		ID:     id,
		Status: statusUpdate.Status,
	}

	statusData, err := json.Marshal(statusChange)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to marshal status data: %v", err), http.StatusInternalServerError)
		return
	}

	cmd := fsm.Command{
		Op:         fsm.OpUpdate,
		EntityType: fsm.EntityPrintJob,
		Payload:    statusData,
	}

	result, err := h.applyCommand(&cmd)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to update print job status: %v", err), http.StatusInternalServerError)
		return
	}

	if result != nil {
		if errResult, ok := result.(error); ok {
			http.Error(w, errResult.Error(), http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"id": id, "status": statusUpdate.Status})
}

func (h *Handler) GetNodeStatus(w http.ResponseWriter, r *http.Request) {
	isLeader := h.raftServer.IsLeader()
	leaderAddr := h.raftServer.LeaderAddr()
	state := h.raftServer.GetState()
	nodeID := h.raftServer.GetNodeID()

	status := struct {
		NodeID     string `json:"node_id"`
		State      string `json:"state"`
		IsLeader   bool   `json:"is_leader"`
		LeaderAddr string `json:"leader_addr"`
	}{
		NodeID:     nodeID,
		State:      state.String(),
		IsLeader:   isLeader,
		LeaderAddr: leaderAddr,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func StartServer(handler *Handler, addr string) error {
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	return server.ListenAndServe()
}
