// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mantaray

import (
	"crypto/rand"
	"errors"
	"fmt"
)

const (
	PathSeparator = '/' // path separator
)

const (
	preambleSize = 64
	forkSize     = 64
)

var (
	// ErrTooShort too short input
	ErrTooShort = errors.New("serialised input too short")
	// ErrInvalid input to seralise invalid
	ErrInvalid = errors.New("input invalid")
	// ErrForkIvalid shows embedded node on a fork has no reference
	ErrForkIvalid = errors.New("fork node without reference")
)

var nonceFn = func(p []byte) (n int, err error) {
	return rand.Read(p)
}

// MarshalBinary serialises the node
func (n *Node) MarshalBinary() (bytes []byte, err error) {
	if n.forks == nil {
		return nil, ErrInvalid
	}
	bytes = make([]byte, 128)

	if len(n.nonce) == 0 {
		// generate nonce
		nonce := make([]byte, 32)
		for i := 0; i < 32; {
			read, _ := nonceFn(nonce[i:])
			i += read
		}
		n.nonce = nonce
	}
	copy(bytes[32:64], n.nonce)

	copy(bytes[64:96], n.entry)

	var index = &bitsForBytes{}
	for k := range n.forks {
		index.set(k)
	}
	copy(bytes[96:128], index.bytes())

	err = index.iter(func(b byte) error {
		f := n.forks[b]
		ref, err := f.bytes()
		if err != nil {
			return fmt.Errorf("%w on byte '%x'", err, []byte{b})
		}
		bytes = append(bytes, ref...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// add info about fork node type
	// NOTE: variable size
	nodeTypesBytes := []byte{}
	err = index.iter(func(b byte) error {
		nt := n.forks[b].nodeType
		nodeTypesBytes = append(nodeTypesBytes, byte(nt))
		return nil
	})
	if err != nil {
		return nil, err
	}
	bytes = append(bytes, nodeTypesBytes...)

	return bytes, nil
}

// bitsForBytes is a set of bytes represented as a 256-length bitvector
type bitsForBytes struct {
	bits [32]byte
}

func (bb *bitsForBytes) bytes() (b []byte) {
	b = append(b, bb.bits[:]...)
	return b
}

func (bb *bitsForBytes) fromBytes(b []byte) {
	copy(bb.bits[:], b)
}

func (bb *bitsForBytes) set(b byte) {
	bb.bits[uint8(b)/8] |= 1 << (uint8(b) % 8)
}

//nolint,unused
func (bb *bitsForBytes) get(b byte) bool {
	return bb.getUint8(uint8(b))
}

func (bb *bitsForBytes) getUint8(i uint8) bool {
	return (bb.bits[i/8]>>(i%8))&1 > 0
}

func (bb *bitsForBytes) iter(f func(byte) error) error {
	for i := uint8(0); ; i++ {
		if bb.getUint8(i) {
			if err := f(byte(i)); err != nil {
				return err
			}
		}
		if i == 255 {
			return nil
		}
	}
}

// UnmarshalBinary deserialises a node
func (n *Node) UnmarshalBinary(bytes []byte) error {
	if len(bytes) < preambleSize {
		return ErrTooShort
	}
	offset := 0
	offset += 32
	n.nonce = append([]byte{}, bytes[offset:offset+32]...)
	offset += 32
	n.entry = append([]byte{}, bytes[offset:offset+32]...)
	offset += 32
	n.forks = make(map[byte]*fork)
	bb := &bitsForBytes{}
	bb.fromBytes(bytes[offset:])
	offset += 32
	err := bb.iter(func(b byte) error {
		f := &fork{}
		f.fromBytes(bytes[offset : offset+forkSize])
		n.forks[b] = f
		offset += forkSize
		return nil
	})
	if err != nil {
		return err
	}

	// read info about fork node type
	// process fork node types sequentally
	// NOTE: this MUST come after reading forks
	return bb.iter(func(b byte) error {
		n.forks[b].nodeType = uint8(bytes[offset])
		offset++
		return nil
	})
}

func (f *fork) fromBytes(b []byte) {
	f.prefix = bytesToPrefix(b[:32])
	f.Node = NewNodeRef(b[32:64])
}

func (f *fork) bytes() (b []byte, err error) {
	b = append(b, prefixToBytes(f.prefix)...)
	r := refBytes(f)
	b = append(b, r...)
	return b, nil
}

var refBytes = nodeRefBytes

func nodeRefBytes(f *fork) []byte {
	return f.Node.ref
}

func prefixToBytes(prefix []byte) (bytes []byte) {
	bytes = append(bytes, prefix...)
	for i := len(prefix); i < 32; i++ {
		bytes = append(bytes, PathSeparator)
	}
	return bytes
}

func bytesToPrefix(bytes []byte) (prefix []byte) {
	for _, b := range bytes {
		if b != PathSeparator {
			prefix = append(prefix, b)
		}
	}
	return prefix
}
