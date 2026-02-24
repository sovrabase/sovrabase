package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	cliStateDirName  = "sovrabase"
	cliStateFileName = "cli-state.json"
)

type cliState struct {
	ActorUserID string `json:"actor_user_id"`
}

type cliStateStore struct {
	path string
}

func newCLIStateStore() (cliStateStore, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return cliStateStore{}, fmt.Errorf("resolve user config dir: %w", err)
	}
	path := filepath.Join(root, cliStateDirName, cliStateFileName)
	return cliStateStore{path: path}, nil
}

func (s cliStateStore) Load() (cliState, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return cliState{}, err
	}
	var state cliState
	if err := json.Unmarshal(data, &state); err != nil {
		return cliState{}, fmt.Errorf("decode state: %w", err)
	}
	if state.ActorUserID == "" {
		return cliState{}, fmt.Errorf("invalid state: actor_user_id is empty")
	}
	return state, nil
}

func (s cliStateStore) Save(state cliState) error {
	if state.ActorUserID == "" {
		return fmt.Errorf("invalid state: actor_user_id is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

func (s cliStateStore) Path() (string, error) {
	return filepath.Dir(s.path), nil
}
