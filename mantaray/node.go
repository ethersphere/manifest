// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mantaray

import (
	"errors"
	"fmt"
)

// Error used when lookup path does not match
var (
	ErrNotFound = errors.New("not found")
)

// Node represents a mantaray Node
type Node struct {
	ref   []byte // reference to uninstantiated Node persisted serialised
	entry []byte
	forks map[byte]*fork
}

type fork struct {
	prefix []byte // the non-branching part of the subpath
	*Node         // in memory structure that represents the Node
}

// NewNodeRef is the exported Node constructor used to represent manifests by reference
func NewNodeRef(ref []byte) *Node {
	return &Node{ref: ref}
}

// New is the constructor for in-memory Node structure
func New() *Node {
	return &Node{forks: make(map[byte]*fork)}
}

func notFound(path []byte) error {
	return fmt.Errorf("entry on '%s' ('%x'): %w", path, path, ErrNotFound)
}

// Lookup finds the entry for a path or returns error if not found
func (n *Node) Lookup(path []byte, l Loader) ([]byte, error) {
	if n.forks == nil {
		if err := n.load(l); err != nil {
			return nil, err
		}
	}
	if len(path) == 0 {
		return n.entry, nil
	}
	f := n.forks[path[0]]
	if f == nil {
		return nil, notFound(path)
	}
	c := common(f.prefix, path)
	if len(c) == len(f.prefix) {
		return f.Node.Lookup(path[len(c):], l)
	}
	return nil, notFound(path)
}

// Add adds an entry to the path
func (n *Node) Add(path []byte, entry []byte, ls LoadSaver) error {
	if len(path) == 0 {
		n.entry = entry
		n.ref = nil
		return nil
	}
	if n.forks == nil {
		if err := n.load(ls); err != nil {
			return err
		}
		n.ref = nil
	}
	f := n.forks[path[0]]
	if f == nil {
		nn := New()
		nn.entry = entry
		n.forks[path[0]] = &fork{path, nn}
		return nil
	}
	c := common(f.prefix, path)
	rest := f.prefix[len(c):]
	nn := f.Node
	if len(rest) > 0 {
		nn = New()
		nn.forks[rest[0]] = &fork{rest, f.Node}
	}
	err := nn.Add(path[len(c):], entry, ls)
	if err != nil {
		return err
	}
	n.forks[path[0]] = &fork{c, nn}
	return nil
}

func common(a, b []byte) (c []byte) {
	for i := 0; i < len(a) && i < len(b) && a[i] == b[i]; i++ {
		c = append(c, a[i])
	}
	return c
}
