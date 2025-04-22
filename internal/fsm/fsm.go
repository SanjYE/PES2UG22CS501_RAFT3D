package fsm

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/raft"
	"github.com/raft3d/pkg/models"
)

type Command struct {
	Op         string          `json:"op"`
	EntityType string          `json:"entity_type"`
	Payload    json.RawMessage `json:"payload"`
}

const (
	EntityPrinter  = "printer"
	EntityFilament = "filament"
	EntityPrintJob = "print_job"
)

const (
	OpCreate = "create"
	OpUpdate = "update"
	OpDelete = "delete"
)

type PrintJobStatusChange struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type Store struct {
	mu        sync.RWMutex
	printers  map[string]*models.Printer
	filaments map[string]*models.Filament
	printJobs map[string]*models.PrintJob
}

func NewStore() *Store {
	return &Store{
		printers:  make(map[string]*models.Printer),
		filaments: make(map[string]*models.Filament),
		printJobs: make(map[string]*models.PrintJob),
	}
}

type FSM struct {
	store *Store
}

func NewFSM() *FSM {
	return &FSM{
		store: NewStore(),
	}
}

func (f *FSM) Apply(log *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %v", err)
	}

	f.store.mu.Lock()
	defer f.store.mu.Unlock()

	switch cmd.EntityType {
	case EntityPrinter:
		return f.applyPrinterCommand(&cmd)
	case EntityFilament:
		return f.applyFilamentCommand(&cmd)
	case EntityPrintJob:
		return f.applyPrintJobCommand(&cmd)
	default:
		return fmt.Errorf("unknown entity type: %s", cmd.EntityType)
	}
}

func (f *FSM) applyPrinterCommand(cmd *Command) interface{} {
	switch cmd.Op {
	case OpCreate:
		var printer models.Printer
		if err := json.Unmarshal(cmd.Payload, &printer); err != nil {
			return fmt.Errorf("failed to unmarshal printer: %v", err)
		}

		if err := printer.Validate(); err != nil {
			return err
		}

		f.store.printers[printer.ID] = &printer
		return nil

	case OpUpdate:
		var printer models.Printer
		if err := json.Unmarshal(cmd.Payload, &printer); err != nil {
			return fmt.Errorf("failed to unmarshal printer: %v", err)
		}

		if err := printer.Validate(); err != nil {
			return err
		}

		if _, exists := f.store.printers[printer.ID]; !exists {
			return fmt.Errorf("printer not found: %s", printer.ID)
		}

		f.store.printers[printer.ID] = &printer
		return nil

	case OpDelete:
		var id string
		if err := json.Unmarshal(cmd.Payload, &id); err != nil {
			return fmt.Errorf("failed to unmarshal ID: %v", err)
		}

		if _, exists := f.store.printers[id]; !exists {
			return fmt.Errorf("printer not found: %s", id)
		}

		delete(f.store.printers, id)
		return nil

	default:
		return fmt.Errorf("unknown printer operation: %s", cmd.Op)
	}
}

func (f *FSM) applyFilamentCommand(cmd *Command) interface{} {
	switch cmd.Op {
	case OpCreate:
		var filament models.Filament
		if err := json.Unmarshal(cmd.Payload, &filament); err != nil {
			return fmt.Errorf("failed to unmarshal filament: %v", err)
		}

		if err := filament.Validate(); err != nil {
			return err
		}

		f.store.filaments[filament.ID] = &filament
		return nil

	case OpUpdate:
		var filament models.Filament
		if err := json.Unmarshal(cmd.Payload, &filament); err != nil {
			return fmt.Errorf("failed to unmarshal filament: %v", err)
		}

		if err := filament.Validate(); err != nil {
			return err
		}

		if _, exists := f.store.filaments[filament.ID]; !exists {
			return fmt.Errorf("filament not found: %s", filament.ID)
		}

		f.store.filaments[filament.ID] = &filament
		return nil

	case OpDelete:
		var id string
		if err := json.Unmarshal(cmd.Payload, &id); err != nil {
			return fmt.Errorf("failed to unmarshal ID: %v", err)
		}

		if _, exists := f.store.filaments[id]; !exists {
			return fmt.Errorf("filament not found: %s", id)
		}

		delete(f.store.filaments, id)
		return nil

	default:
		return fmt.Errorf("unknown filament operation: %s", cmd.Op)
	}
}

func (f *FSM) applyPrintJobCommand(cmd *Command) interface{} {
	switch cmd.Op {
	case OpCreate:
		var printJob models.PrintJob
		if err := json.Unmarshal(cmd.Payload, &printJob); err != nil {
			return fmt.Errorf("failed to unmarshal print job: %v", err)
		}

		printJob.Status = models.StatusQueued

		if err := printJob.Validate(); err != nil {
			return err
		}

		if _, exists := f.store.printers[printJob.PrinterID]; !exists {
			return fmt.Errorf("printer not found: %s", printJob.PrinterID)
		}

		filament, exists := f.store.filaments[printJob.FilamentID]
		if !exists {
			return fmt.Errorf("filament not found: %s", printJob.FilamentID)
		}

		if printJob.PrintWeightInGrams > filament.RemainingWeightInGrams {
			return fmt.Errorf("insufficient filament: required %d g, available %d g",
				printJob.PrintWeightInGrams, filament.RemainingWeightInGrams)
		}

		f.store.printJobs[printJob.ID] = &printJob
		return nil

	case OpUpdate:
		var statusChange PrintJobStatusChange
		if err := json.Unmarshal(cmd.Payload, &statusChange); err != nil {
			return fmt.Errorf("failed to unmarshal status change: %v", err)
		}

		printJob, exists := f.store.printJobs[statusChange.ID]
		if !exists {
			return fmt.Errorf("print job not found: %s", statusChange.ID)
		}

		if err := printJob.ValidateTransition(statusChange.Status); err != nil {
			return err
		}

		if statusChange.Status == models.StatusDone {
			filament, exists := f.store.filaments[printJob.FilamentID]
			if !exists {
				return fmt.Errorf("filament not found: %s", printJob.FilamentID)
			}

			filament.RemainingWeightInGrams -= printJob.PrintWeightInGrams
			if filament.RemainingWeightInGrams < 0 {
				filament.RemainingWeightInGrams = 0
			}
		}

		printJob.Status = statusChange.Status
		return nil

	case OpDelete:
		var id string
		if err := json.Unmarshal(cmd.Payload, &id); err != nil {
			return fmt.Errorf("failed to unmarshal ID: %v", err)
		}

		if _, exists := f.store.printJobs[id]; !exists {
			return fmt.Errorf("print job not found: %s", id)
		}

		delete(f.store.printJobs, id)
		return nil

	default:
		return fmt.Errorf("unknown print job operation: %s", cmd.Op)
	}
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.store.mu.RLock()
	defer f.store.mu.RUnlock()

	printers := make(map[string]*models.Printer)
	for k, v := range f.store.printers {
		printer := *v
		printers[k] = &printer
	}

	filaments := make(map[string]*models.Filament)
	for k, v := range f.store.filaments {
		filament := *v
		filaments[k] = &filament
	}

	printJobs := make(map[string]*models.PrintJob)
	for k, v := range f.store.printJobs {
		printJob := *v
		printJobs[k] = &printJob
	}

	return &Snapshot{
		Printers:  printers,
		Filaments: filaments,
		PrintJobs: printJobs,
	}, nil
}

func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	decoder := json.NewDecoder(rc)
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return fmt.Errorf("failed to decode snapshot: %v", err)
	}

	f.store.mu.Lock()
	defer f.store.mu.Unlock()

	f.store.printers = snapshot.Printers
	f.store.filaments = snapshot.Filaments
	f.store.printJobs = snapshot.PrintJobs

	return nil
}

type Snapshot struct {
	Printers  map[string]*models.Printer
	Filaments map[string]*models.Filament
	PrintJobs map[string]*models.PrintJob
}

func (s *Snapshot) Persist(sink raft.SnapshotSink) error {
	data, err := json.Marshal(s)
	if err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to marshal snapshot: %v", err)
	}

	if _, err := sink.Write(data); err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to write snapshot: %v", err)
	}

	return sink.Close()
}

func (s *Snapshot) Release() {}

func (s *Store) GetPrinters() []*models.Printer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	printers := make([]*models.Printer, 0, len(s.printers))
	for _, p := range s.printers {
		printers = append(printers, p)
	}

	return printers
}

func (s *Store) GetPrinter(id string) (*models.Printer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	printer, found := s.printers[id]
	return printer, found
}

func (s *Store) GetFilaments() []*models.Filament {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filaments := make([]*models.Filament, 0, len(s.filaments))
	for _, f := range s.filaments {
		filaments = append(filaments, f)
	}

	return filaments
}

func (s *Store) GetFilament(id string) (*models.Filament, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filament, found := s.filaments[id]
	return filament, found
}

func (s *Store) GetPrintJobs() []*models.PrintJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	printJobs := make([]*models.PrintJob, 0, len(s.printJobs))
	for _, j := range s.printJobs {
		printJobs = append(printJobs, j)
	}

	return printJobs
}

func (s *Store) GetPrintJob(id string) (*models.PrintJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	printJob, found := s.printJobs[id]
	return printJob, found
}

func (f *FSM) Store() *Store {
	return f.store
}
