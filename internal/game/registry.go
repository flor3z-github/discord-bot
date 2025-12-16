package game

import (
	"fmt"
	"sync"
)

// Registry manages all registered game trackers
type Registry struct {
	mu       sync.RWMutex
	trackers map[GameType]Tracker
}

// NewRegistry creates a new game registry
func NewRegistry() *Registry {
	return &Registry{
		trackers: make(map[GameType]Tracker),
	}
}

// Register adds a game tracker to the registry
func (r *Registry) Register(tracker Tracker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trackers[tracker.Type()] = tracker
}

// Get retrieves a tracker by game type
func (r *Registry) Get(gameType GameType) (Tracker, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tracker, ok := r.trackers[gameType]
	if !ok {
		return nil, fmt.Errorf("unknown game type: %s", gameType)
	}
	return tracker, nil
}

// GetAll returns all registered trackers
func (r *Registry) GetAll() []Tracker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	trackers := make([]Tracker, 0, len(r.trackers))
	for _, tracker := range r.trackers {
		trackers = append(trackers, tracker)
	}
	return trackers
}

// List returns information about all registered games
func (r *Registry) List() []GameInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	games := make([]GameInfo, 0, len(r.trackers))
	for _, tracker := range r.trackers {
		games = append(games, GameInfo{
			Type:        tracker.Type(),
			Name:        tracker.Name(),
			Description: tracker.Description(),
		})
	}
	return games
}

// GameInfo contains display information about a game
type GameInfo struct {
	Type        GameType
	Name        string
	Description string
}
