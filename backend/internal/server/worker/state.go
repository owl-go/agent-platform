package worker

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

type fatalError struct{ cause error }

func (err fatalError) Error() string { return err.cause.Error() }
func (err fatalError) Unwrap() error { return err.cause }

func Fatal(err error) error {
	if err == nil {
		return nil
	}
	return fatalError{cause: err}
}

func IsFatal(err error) bool {
	var target fatalError
	return errors.As(err, &target)
}

type State struct {
	mu         sync.RWMutex
	registered map[string]struct{}
	started    map[string]bool
	fatal      map[string]error
}

func NewState() *State {
	return &State{
		registered: make(map[string]struct{}),
		started:    make(map[string]bool),
		fatal:      make(map[string]error),
	}
}

func (state *State) register(name string) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if _, exists := state.registered[name]; exists {
		return fmt.Errorf("Worker loop %s is registered more than once", name)
	}
	state.registered[name] = struct{}{}
	return nil
}

func (state *State) markStarted(name string) {
	state.mu.Lock()
	state.started[name] = true
	state.mu.Unlock()
}

func (state *State) markStopped(name string) {
	state.mu.Lock()
	state.started[name] = false
	state.mu.Unlock()
}

func (state *State) markFatal(name string, err error) {
	state.mu.Lock()
	state.fatal[name] = err
	state.started[name] = false
	state.mu.Unlock()
}

func (state *State) Ready() bool {
	if state == nil {
		return false
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	if len(state.registered) == 0 || len(state.fatal) != 0 {
		return false
	}
	for name := range state.registered {
		if !state.started[name] {
			return false
		}
	}
	return true
}

type LoopStatus struct {
	Name    string
	Started bool
	Fatal   bool
}

func (state *State) Statuses() []LoopStatus {
	if state == nil {
		return nil
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	statuses := make([]LoopStatus, 0, len(state.registered))
	for name := range state.registered {
		_, fatal := state.fatal[name]
		statuses = append(statuses, LoopStatus{Name: name, Started: state.started[name], Fatal: fatal})
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })
	return statuses
}
