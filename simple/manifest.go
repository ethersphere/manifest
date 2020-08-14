// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simple

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// Error used when lookup path does not match
var (
	ErrNotFound  = errors.New("not found")
	ErrEmptyPath = errors.New("empty path")
)

// Manifest is a JSON representation of a manifest.
// It stores manifest entries in a map based on string keys.
type Manifest struct {
	Entries map[string]*Entry `json:"entries,omitempty"`

	mu sync.RWMutex // mutex for accessing the entries map
}

// NewManifest creates a new Manifest struct and returns a pointer to it.
func NewManifest() *Manifest {
	return &Manifest{
		Entries: make(map[string]*Entry),
	}
}

func notFound(path string) error {
	return fmt.Errorf("entry on '%s': %w", path, ErrNotFound)
}

// Add adds a manifest entry to the specified path.
func (m *Manifest) Add(path string, entry string) error {
	if len(path) == 0 {
		return ErrEmptyPath
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.Entries[path] = NewEntry(entry)

	return nil
}

// Remove removes a manifest entry on the specified path.
func (m *Manifest) Remove(path string) error {
	if len(path) == 0 {
		return ErrEmptyPath
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.Entries, path)

	return nil
}

// Lookup returns a manifest node entry if one is found in the specified path.
func (m *Manifest) Lookup(path string) (*Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.Entries[path]
	if !ok {
		return nil, notFound(path)
	}

	// return a copy to prevent external modification
	return NewEntry(entry.Reference()), nil
}

// Length returns an implementation-specific count of elements in the manifest.
// For Manifest, this means the number of all the existing entries.
func (m *Manifest) Length() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.Entries)
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (m *Manifest) MarshalBinary() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return json.Marshal(m)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (m *Manifest) UnmarshalBinary(b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return json.Unmarshal(b, m)
}
