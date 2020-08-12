// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mantaray

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

// Version constants.
const (
	versionNameString   = "mantaray"
	versionCode01String = "0.1"

	versionSeparatorString = ":"

	version01String     = versionNameString + versionSeparatorString + versionCode01String   // "mantaray:0.1"
	version01HashString = "025184789d63635766d78c41900196b57d7400875ebe4d9b5d1e76bd9652a9b7" // pre-calculated version string, Keccak-256
)

// Node header fields constants.
const (
	nodeObfuscationKeySize = 32
	versionHashSize        = 31
	nodeRefBytesSize       = 1

	// nodeHeaderSize defines the total size of the header part
	nodeHeaderSize = nodeObfuscationKeySize + versionHashSize + nodeRefBytesSize
)

// Node fork constats.
const (
	nodeForkTypeBytesSize    = 1
	nodeForkPrefixBytesSize  = 1
	nodeForkHeaderSize       = nodeForkTypeBytesSize + nodeForkPrefixBytesSize // 2
	nodeForkPreReferenceSize = 32
	nodePrefixMaxSize        = nodeForkPreReferenceSize - nodeForkHeaderSize // 30
)

var (
	version01HashBytes []byte
)

func init() {
	b, err := hex.DecodeString(version01HashString)
	if err != nil {
		panic(err)
	}

	version01HashBytes = make([]byte, versionHashSize)
	copy(version01HashBytes, b)
}

var (
	// ErrTooShort too short input
	ErrTooShort = errors.New("serialised input too short")
	// ErrInvalid input to seralise invalid
	ErrInvalid = errors.New("input invalid")
	// ErrForkIvalid shows embedded node on a fork has no reference
	ErrForkIvalid = errors.New("fork node without reference")
)

var obfuscationKeyFn = func(p []byte) (n int, err error) {
	return rand.Read(p)
}

// MarshalBinary serialises the node
func (n *Node) MarshalBinary() (bytes []byte, err error) {
	if n.forks == nil {
		return nil, ErrInvalid
	}

	// header

	headerBytes := make([]byte, nodeHeaderSize)

	if len(n.obfuscationKey) == 0 {
		// generate obfuscation key
		obfuscationKey := make([]byte, nodeObfuscationKeySize)
		for i := 0; i < nodeObfuscationKeySize; {
			read, _ := obfuscationKeyFn(obfuscationKey[i:])
			i += read
		}
		n.obfuscationKey = obfuscationKey
	}
	copy(headerBytes[0:nodeObfuscationKeySize], n.obfuscationKey)

	copy(headerBytes[nodeObfuscationKeySize:nodeObfuscationKeySize+versionHashSize], version01HashBytes)

	headerBytes[nodeObfuscationKeySize+versionHashSize] = uint8(n.refBytesSize)

	bytes = append(bytes, headerBytes...)

	// entry

	entryBytes := make([]byte, n.refBytesSize)
	copy(entryBytes, n.entry)
	bytes = append(bytes, entryBytes...)

	// index

	indexBytes := make([]byte, 32)

	var index = &bitsForBytes{}
	for k := range n.forks {
		index.set(k)
	}
	copy(indexBytes, index.bytes())

	bytes = append(bytes, indexBytes...)

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
func (n *Node) UnmarshalBinary(ba []byte) error {
	if len(ba) < nodeHeaderSize {
		return ErrTooShort
	}

	n.obfuscationKey = append([]byte{}, ba[0:nodeObfuscationKeySize]...)

	// Verify version hash.
	if versionHash := append([]byte{}, ba[nodeObfuscationKeySize:nodeObfuscationKeySize+versionHashSize]...); !bytes.Equal(versionHash, version01HashBytes) {
		return fmt.Errorf("invalid version hash %x", versionHash)
	}

	refBytesSize := int(ba[nodeHeaderSize-1])

	n.entry = append([]byte{}, ba[nodeHeaderSize:nodeHeaderSize+refBytesSize]...)
	offset := nodeHeaderSize + refBytesSize // skip entry
	n.forks = make(map[byte]*fork)
	bb := &bitsForBytes{}
	bb.fromBytes(ba[offset:])
	offset += 32 // skip forks
	return bb.iter(func(b byte) error {
		f := &fork{}
		f.fromBytes(ba[offset : offset+nodeForkPreReferenceSize+refBytesSize])
		n.forks[b] = f
		offset += nodeForkPreReferenceSize + refBytesSize
		return nil
	})
}

func (f *fork) fromBytes(b []byte) {
	nodeType := uint8(b[0])
	prefixLen := int(uint8(b[1]))

	f.prefix = b[nodeForkHeaderSize : nodeForkHeaderSize+prefixLen]
	f.Node = NewNodeRef(b[nodeForkPreReferenceSize:])
	f.Node.nodeType = nodeType
}

func (f *fork) bytes() (b []byte, err error) {
	r := refBytes(f)
	// using 1 byte ('f.Node.refBytesSize') for size
	if len(r) > 256 {
		err = fmt.Errorf("node reference size > 256: %d", len(r))
		return
	}
	b = append(b, f.Node.nodeType)
	b = append(b, uint8(len(f.prefix)))

	prefixBytes := make([]byte, nodePrefixMaxSize)
	copy(prefixBytes, f.prefix)
	b = append(b, prefixBytes...)

	refBytes := make([]byte, len(r))
	copy(refBytes, r)
	b = append(b, refBytes...)

	return b, nil
}

var refBytes = nodeRefBytes

func nodeRefBytes(f *fork) []byte {
	return f.Node.ref
}
