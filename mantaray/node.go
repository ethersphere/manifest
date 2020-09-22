// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mantaray

import (
	"bytes"
	"errors"
	"fmt"
)

const (
	PathSeparator = '/' // path separator
)

// Error used when lookup path does not match
var (
	ErrNotFound         = errors.New("not found")
	ErrEmptyPath        = errors.New("empty path")
	ErrMetadataTooLarge = errors.New("metadata too large")
)

// Node represents a mantaray Node
type Node struct {
	nodeType       uint8
	refBytesSize   int
	obfuscationKey []byte
	ref            []byte // reference to uninstantiated Node persisted serialised
	entry          []byte
	metadata       map[string]string
	forks          map[byte]*fork
}

type fork struct {
	prefix []byte // the non-branching part of the subpath
	*Node         // in memory structure that represents the Node
}

const (
	nodeTypeValue             = uint8(2)
	nodeTypeEdge              = uint8(4)
	nodeTypeWithPathSeparator = uint8(8)
	nodeTypeWithMetadata      = uint8(16)

	nodeTypeMask = uint8(255)
)

func nodeTypeIsWithMetadataType(nodeType uint8) bool {
	return nodeType&nodeTypeWithMetadata == nodeTypeWithMetadata
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

func (n *Node) isValueType() bool {
	return n.nodeType&nodeTypeValue == nodeTypeValue
}

func (n *Node) isEdgeType() bool {
	return n.nodeType&nodeTypeEdge == nodeTypeEdge
}

func (n *Node) isWithPathSeparatorType() bool {
	return n.nodeType&nodeTypeWithPathSeparator == nodeTypeWithPathSeparator
}

func (n *Node) isWithMetadataType() bool {
	return n.nodeType&nodeTypeWithMetadata == nodeTypeWithMetadata
}

func (n *Node) makeValue() {
	n.nodeType = n.nodeType | nodeTypeValue
}

func (n *Node) makeEdge() {
	n.nodeType = n.nodeType | nodeTypeEdge
}

func (n *Node) makeWithPathSeparator() {
	n.nodeType = n.nodeType | nodeTypeWithPathSeparator
}

func (n *Node) makeWithMetadata() {
	n.nodeType = n.nodeType | nodeTypeWithMetadata
}

//nolint,unused
func (n *Node) makeNotValue() {
	n.nodeType = (nodeTypeMask ^ nodeTypeValue) & n.nodeType
}

//nolint,unused
func (n *Node) makeNotEdge() {
	n.nodeType = (nodeTypeMask ^ nodeTypeEdge) & n.nodeType
}

func (n *Node) makeNotWithPathSeparator() {
	n.nodeType = (nodeTypeMask ^ nodeTypeWithPathSeparator) & n.nodeType
}

//nolint,unused
func (n *Node) makeNotWithMetadata() {
	n.nodeType = (nodeTypeMask ^ nodeTypeWithMetadata) & n.nodeType
}

func (n *Node) SetObfuscationKey(obfuscationKey []byte) {
	bytes := make([]byte, 32)
	copy(bytes, obfuscationKey)
	n.obfuscationKey = bytes
}

// Reference returns the address of the mantaray node if saved.
func (n *Node) Reference() []byte {
	return n.ref
}

// Entry returns the value stored on the specific path.
func (n *Node) Entry() []byte {
	return n.entry
}

// Metadata returns the metadata stored on the specific path.
func (n *Node) Metadata() map[string]string {
	return n.metadata
}

// LookupNode finds the node for a path or returns error if not found
func (n *Node) LookupNode(path []byte, l Loader) (*Node, error) {
	if n.forks == nil {
		if err := n.load(l); err != nil {
			return nil, err
		}
	}
	if len(path) == 0 {
		return n, nil
	}
	f := n.forks[path[0]]
	if f == nil {
		return nil, notFound(path)
	}
	c := common(f.prefix, path)
	if len(c) == len(f.prefix) {
		return f.Node.LookupNode(path[len(c):], l)
	}
	return nil, notFound(path)
}

// Lookup finds the entry for a path or returns error if not found
func (n *Node) Lookup(path []byte, l Loader) ([]byte, error) {
	node, err := n.LookupNode(path, l)
	if err != nil {
		return nil, err
	}
	return node.entry, nil
}

// Add adds an entry to the path
func (n *Node) Add(path []byte, entry []byte, metadata map[string]string, ls LoadSaver) error {
	if n.refBytesSize == 0 {
		if len(entry) > 256 {
			return fmt.Errorf("node entry size > 256: %d", len(entry))
		}
		// empty entry for directories
		if len(entry) > 0 {
			n.refBytesSize = len(entry)
		}
	} else {
		if len(entry) > 0 && n.refBytesSize != len(entry) {
			return fmt.Errorf("invalid entry size: %d, expected: %d", len(entry), n.refBytesSize)
		}
	}

	if len(path) == 0 {
		n.entry = entry
		if len(metadata) > 0 {
			n.metadata = metadata
			n.makeWithMetadata()
		}
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
		nn.refBytesSize = n.refBytesSize
		// check for prefix size limit
		if len(path) > nodePrefixMaxSize {
			prefix := path[:nodePrefixMaxSize]
			rest := path[nodePrefixMaxSize:]
			err := nn.Add(rest, entry, metadata, ls)
			if err != nil {
				return err
			}
			nn.updateIsWithPathSeparator(prefix)
			n.forks[path[0]] = &fork{prefix, nn}
			n.makeEdge()
			return nil
		}
		nn.entry = entry
		if len(metadata) > 0 {
			nn.metadata = metadata
			nn.makeWithMetadata()
		}
		nn.makeValue()
		nn.updateIsWithPathSeparator(path)
		n.forks[path[0]] = &fork{path, nn}
		n.makeEdge()
		return nil
	}
	c := common(f.prefix, path)
	rest := f.prefix[len(c):]
	nn := f.Node
	if len(rest) > 0 {
		// move current common prefix node
		nn = New()
		nn.refBytesSize = n.refBytesSize
		f.Node.updateIsWithPathSeparator(rest)
		nn.forks[rest[0]] = &fork{rest, f.Node}
		nn.makeEdge()
	}
	// NOTE: special case on edge split
	nn.updateIsWithPathSeparator(path)
	// add new for shared prefix
	err := nn.Add(path[len(c):], entry, metadata, ls)
	if err != nil {
		return err
	}
	n.forks[path[0]] = &fork{c, nn}
	n.makeEdge()
	return nil
}

func (n *Node) updateIsWithPathSeparator(path []byte) {
	if bytes.IndexRune(path, PathSeparator) > 0 {
		n.makeWithPathSeparator()
	} else {
		n.makeNotWithPathSeparator()
	}
}

// Remove removes a path from the node
func (n *Node) Remove(path []byte, ls LoadSaver) error {
	if len(path) == 0 {
		return ErrEmptyPath
	}
	if n.forks == nil {
		if err := n.load(ls); err != nil {
			return err
		}
	}
	f := n.forks[path[0]]
	if f == nil {
		return ErrNotFound
	}
	prefixIndex := bytes.Index(path, f.prefix)
	if prefixIndex != 0 {
		return ErrNotFound
	}
	rest := path[len(f.prefix):]
	if len(rest) == 0 {
		// full path matched
		delete(n.forks, path[0])
		return nil
	}
	return f.Node.Remove(rest, ls)
}

func common(a, b []byte) (c []byte) {
	for i := 0; i < len(a) && i < len(b) && a[i] == b[i]; i++ {
		c = append(c, a[i])
	}
	return c
}

// HasPrefix tests whether the node contains prefix path.
func (n *Node) HasPrefix(path []byte, l Loader) (bool, error) {
	if n.forks == nil {
		if err := n.load(l); err != nil {
			return false, err
		}
	}
	if len(path) == 0 {
		return true, nil
	}
	f := n.forks[path[0]]
	if f == nil {
		return false, nil
	}
	c := common(f.prefix, path)
	if len(c) == len(f.prefix) {
		return f.Node.HasPrefix(path[len(c):], l)
	}
	if bytes.HasPrefix(f.prefix, path) {
		return true, nil
	}
	return false, nil
}
