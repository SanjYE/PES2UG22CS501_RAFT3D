package models

import (
	"encoding/json"
	"fmt"
)

type Printer struct {
	ID      string `json:"id"`
	Company string `json:"company"`
	Model   string `json:"model"`
}

type Filament struct {
	ID                     string `json:"id"`
	Type                   string `json:"type"`
	Color                  string `json:"color"`
	TotalWeightInGrams     int    `json:"total_weight_in_grams"`
	RemainingWeightInGrams int    `json:"remaining_weight_in_grams"`
}

type PrintJob struct {
	ID                 string `json:"id"`
	PrinterID          string `json:"printer_id"`
	FilamentID         string `json:"filament_id"`
	Filepath           string `json:"filepath"`
	PrintWeightInGrams int    `json:"print_weight_in_grams"`
	Status             string `json:"status"`
}

const (
	StatusQueued    = "Queued"
	StatusRunning   = "Running"
	StatusDone      = "Done"
	StatusCancelled = "Cancelled"
)

func (p *Printer) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("printer ID cannot be empty")
	}
	if p.Company == "" {
		return fmt.Errorf("printer company cannot be empty")
	}
	if p.Model == "" {
		return fmt.Errorf("printer model cannot be empty")
	}
	return nil
}

func (f *Filament) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("filament ID cannot be empty")
	}

	validTypes := map[string]bool{
		"PLA":  true,
		"PETG": true,
		"ABS":  true,
		"TPU":  true,
	}

	if !validTypes[f.Type] {
		return fmt.Errorf("filament type must be one of: PLA, PETG, ABS, TPU")
	}

	if f.Color == "" {
		return fmt.Errorf("filament color cannot be empty")
	}

	if f.TotalWeightInGrams <= 0 {
		return fmt.Errorf("total weight must be positive")
	}

	if f.RemainingWeightInGrams < 0 || f.RemainingWeightInGrams > f.TotalWeightInGrams {
		return fmt.Errorf("remaining weight must be between 0 and total weight")
	}

	return nil
}

func (j *PrintJob) Validate() error {
	if j.ID == "" {
		return fmt.Errorf("print job ID cannot be empty")
	}
	if j.PrinterID == "" {
		return fmt.Errorf("printer ID cannot be empty")
	}
	if j.FilamentID == "" {
		return fmt.Errorf("filament ID cannot be empty")
	}
	if j.Filepath == "" {
		return fmt.Errorf("filepath cannot be empty")
	}
	if j.PrintWeightInGrams <= 0 {
		return fmt.Errorf("print weight must be positive")
	}

	validStatuses := map[string]bool{
		StatusQueued:    true,
		StatusRunning:   true,
		StatusDone:      true,
		StatusCancelled: true,
	}

	if !validStatuses[j.Status] {
		return fmt.Errorf("status must be one of: Queued, Running, Done, Cancelled")
	}

	return nil
}

func (j *PrintJob) ValidateTransition(newStatus string) error {

	validTransitions := map[string]map[string]bool{
		StatusQueued: {
			StatusRunning:   true,
			StatusCancelled: true,
		},
		StatusRunning: {
			StatusDone:      true,
			StatusCancelled: true,
		},
	}

	if transitions, ok := validTransitions[j.Status]; ok {
		if transitions[newStatus] {
			return nil
		}
	}

	return fmt.Errorf("invalid status transition from %s to %s", j.Status, newStatus)
}

func (p *Printer) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

func (f *Filament) ToJSON() ([]byte, error) {
	return json.Marshal(f)
}

func (j *PrintJob) ToJSON() ([]byte, error) {
	return json.Marshal(j)
}

func (p *Printer) FromJSON(data []byte) error {
	return json.Unmarshal(data, p)
}

func (f *Filament) FromJSON(data []byte) error {
	return json.Unmarshal(data, f)
}

func (j *PrintJob) FromJSON(data []byte) error {
	return json.Unmarshal(data, j)
}
