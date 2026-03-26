package encoder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type SettingsData struct {
	DecayRate      int    `json:"decay_rate"`
	MoveToTrash    bool   `json:"move_to_trash"`
	LowPriority    bool   `json:"low_priority"`
	OutputDir      string `json:"output_dir"`
	ShutdownOnDone bool   `json:"shutdown_on_done"`
	AppendH265     bool   `json:"append_h265"`
	AppendRate     bool   `json:"append_rate"`
	Language       string `json:"language"` // "auto", "ja", "en"
}

func DefaultSettings() SettingsData {
	return SettingsData{
		DecayRate:      50,
		MoveToTrash:    false,
		LowPriority:    false,
		OutputDir:      "",
		ShutdownOnDone: false,
		AppendH265:     true,
		AppendRate:     true,
		Language:       "auto",
	}
}

type Settings struct {
	mu   sync.RWMutex
	data SettingsData
	path string
}

func NewSettings() *Settings {
	s := &Settings{
		data: DefaultSettings(),
	}

	appData := os.Getenv("APPDATA")
	if appData != "" {
		dir := filepath.Join(appData, "h265conv")
		os.MkdirAll(dir, 0755)
		s.path = filepath.Join(dir, "settings.json")
	}

	s.loadFromFile()
	return s
}

func (s *Settings) Load() SettingsData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

func (s *Settings) Update(fn func(*SettingsData)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.data)
}

func (s *Settings) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.path == "" {
		return nil
	}

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *Settings) loadFromFile() {
	if s.path == "" {
		return
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	json.Unmarshal(data, &s.data)

	// Clamp decay rate
	if s.data.DecayRate < 10 {
		s.data.DecayRate = 10
	}
	if s.data.DecayRate > 90 {
		s.data.DecayRate = 90
	}
}
