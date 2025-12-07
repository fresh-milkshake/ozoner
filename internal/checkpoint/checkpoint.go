package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ozoner/internal/models"

	"github.com/charmbracelet/log"
)

// CheckpointConfig holds configuration for checkpoints
type CheckpointConfig struct {
	Enabled  bool
	Dir      string
	AutoSave bool
	OnSave   func(checkpoint *Checkpoint) error
}

// DefaultCheckpointConfig returns default checkpoint configuration
func DefaultCheckpointConfig() CheckpointConfig {
	return CheckpointConfig{
		Enabled:  true,
		Dir:      ".ozoner/checkpoints",
		AutoSave: true,
	}
}

// Checkpoint represents a saved state of processing
type Checkpoint struct {
	ID           string                 `json:"id"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Query        string                 `json:"query"`
	Filters      *models.SearchFilters  `json:"filters,omitempty"`
	ProcessedIDs []string               `json:"processed_ids"`
	CurrentPage  int                    `json:"current_page"`
	TotalTarget  int                    `json:"total_target"`
	State        map[string]interface{} `json:"state,omitempty"`
}

// Manager manages checkpoints
type Manager struct {
	config CheckpointConfig
	mu     sync.RWMutex
}

// NewManager creates a new checkpoint manager
func NewManager(config CheckpointConfig) (*Manager, error) {
	if !config.Enabled {
		return nil, nil
	}

	manager := &Manager{
		config: config,
	}

	// Create checkpoint directory
	if err := os.MkdirAll(config.Dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	return manager, nil
}

// Save saves a checkpoint to disk
func (m *Manager) Save(checkpoint *Checkpoint) error {
	if m == nil || !m.config.Enabled {
		return fmt.Errorf("checkpoint manager not enabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ID if not set (timestamp-based)
	if checkpoint.ID == "" {
		checkpoint.ID = fmt.Sprintf("checkpoint-%d", time.Now().UnixNano())
	}

	// Set timestamps
	now := time.Now()
	if checkpoint.CreatedAt.IsZero() {
		checkpoint.CreatedAt = now
	}
	checkpoint.UpdatedAt = now

	// Serialize checkpoint
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	// Save to file
	checkpointPath := filepath.Join(m.config.Dir, checkpoint.ID+".json")
	if err := os.WriteFile(checkpointPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint file: %w", err)
	}

	// Call callback if set
	if m.config.OnSave != nil {
		if err := m.config.OnSave(checkpoint); err != nil {
			log.Warn("Checkpoint OnSave callback failed", "err", err)
		}
	}

	return nil
}

// Load loads a checkpoint by ID
func (m *Manager) Load(id string) (*Checkpoint, error) {
	if m == nil || !m.config.Enabled {
		return nil, fmt.Errorf("checkpoint manager not enabled")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	checkpointPath := filepath.Join(m.config.Dir, id+".json")
	data, err := os.ReadFile(checkpointPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("checkpoint not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read checkpoint file: %w", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// List returns all checkpoints
func (m *Manager) List() ([]Checkpoint, error) {
	if m == nil || !m.config.Enabled {
		return nil, fmt.Errorf("checkpoint manager not enabled")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.config.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint directory: %w", err)
	}

	var checkpoints []Checkpoint
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		checkpointPath := filepath.Join(m.config.Dir, entry.Name())
		data, err := os.ReadFile(checkpointPath)
		if err != nil {
			log.Warn("Failed to read checkpoint file", "file", entry.Name(), "err", err)
			continue
		}

		var checkpoint Checkpoint
		if err := json.Unmarshal(data, &checkpoint); err != nil {
			log.Warn("Failed to unmarshal checkpoint", "file", entry.Name(), "err", err)
			continue
		}

		checkpoints = append(checkpoints, checkpoint)
	}

	// Sort by CreatedAt descending (newest first)
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].CreatedAt.After(checkpoints[j].CreatedAt)
	})

	return checkpoints, nil
}

// Delete deletes a checkpoint by ID
func (m *Manager) Delete(id string) error {
	if m == nil || !m.config.Enabled {
		return fmt.Errorf("checkpoint manager not enabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	checkpointPath := filepath.Join(m.config.Dir, id+".json")
	if err := os.Remove(checkpointPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("checkpoint not found: %s", id)
		}
		return fmt.Errorf("failed to delete checkpoint file: %w", err)
	}

	return nil
}

// GetLatest returns the most recently created checkpoint
func (m *Manager) GetLatest() (*Checkpoint, error) {
	if m == nil || !m.config.Enabled {
		return nil, fmt.Errorf("checkpoint manager not enabled")
	}

	checkpoints, err := m.List()
	if err != nil {
		return nil, err
	}

	if len(checkpoints) == 0 {
		return nil, fmt.Errorf("no checkpoints found")
	}

	// List already sorts by CreatedAt descending, so first is latest
	return &checkpoints[0], nil
}

// UpdateProcessedIDs updates the list of processed IDs in a checkpoint
func (m *Manager) UpdateProcessedIDs(checkpointID string, productID string) error {
	if m == nil || !m.config.Enabled {
		return fmt.Errorf("checkpoint manager not enabled")
	}

	checkpoint, err := m.Load(checkpointID)
	if err != nil {
		return err
	}

	// Check if already processed
	for _, id := range checkpoint.ProcessedIDs {
		if id == productID {
			return nil // Already processed
		}
	}

	// Add to processed list
	checkpoint.ProcessedIDs = append(checkpoint.ProcessedIDs, productID)
	return m.Save(checkpoint)
}

// IsProcessed checks if a product ID is already processed in a checkpoint
func (m *Manager) IsProcessed(checkpointID string, productID string) (bool, error) {
	if m == nil || !m.config.Enabled {
		return false, fmt.Errorf("checkpoint manager not enabled")
	}

	checkpoint, err := m.Load(checkpointID)
	if err != nil {
		return false, err
	}

	for _, id := range checkpoint.ProcessedIDs {
		if id == productID {
			return true, nil
		}
	}

	return false, nil
}
