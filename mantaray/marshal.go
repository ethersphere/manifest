package mantaray

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
)

// Node header fields constants.
const (
	// magicNode is 4 bytes at the head of a node.
	magicNode = 0x2D833CCB
	// magicNodeSize is the size in bytes of magicNode.
	magicNodeSize          = 4
	nodeFormatV1           = 1
	nodeFormatVersionSize  = 1
	nodeForkRefBytesSize   = 1
	nodeHeaderPaddingSize  = 26 // configured to make header size 64
	nodeObfuscationKeySize = 32
	// nodeHeaderSize defines the total size of the header part.
	nodeHeaderSize = magicNodeSize + nodeFormatVersionSize + nodeForkRefBytesSize + nodeHeaderPaddingSize + nodeObfuscationKeySize
)

const (
	nodeForkTypeBytesSize    = 1
	nodeForkPrefixBytesSize  = 1
	nodeForkHeaderSize       = nodeForkTypeBytesSize + nodeForkPrefixBytesSize // 2
	nodeForkPreReferenceSize = 32
	nodePrefixMaxSize        = nodeForkPreReferenceSize - nodeForkHeaderSize // 30
	nodeReferenceMaxSize     = 32
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

	binary.BigEndian.PutUint32(headerBytes[:magicNodeSize], magicNode)
	headerBytes[4] = nodeFormatV1
	headerBytes[5] = nodeForkRefBytesSize

	if len(n.obfuscationKey) == 0 {
		// generate obfuscation key
		obfuscationKey := make([]byte, nodeObfuscationKeySize)
		for i := 0; i < nodeObfuscationKeySize; {
			read, _ := obfuscationKeyFn(obfuscationKey[i:])
			i += read
		}
		n.obfuscationKey = obfuscationKey
	}
	copy(headerBytes[nodeHeaderSize-nodeObfuscationKeySize:nodeHeaderSize], n.obfuscationKey)

	bytes = append(bytes, headerBytes...)

	// entry

	entryBytes := make([]byte, 32)
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
	if len(bytes) < nodeHeaderSize {
		return ErrTooShort
	}
	// Verify magic number.
	if m := binary.BigEndian.Uint32(bytes[0:magicNodeSize]); m != magicNode {
		return fmt.Errorf("invalid magic number %x", m)
	}
	// Verify chunk format version.
	if v := int(bytes[magicNodeSize : magicNodeSize+nodeFormatVersionSize][0]); v != nodeFormatV1 {
		return fmt.Errorf("invalid chunk format version %d", v)
	}

	// refLen := int(bytes[magicNodeSize+nodeFormatVersionSize+1])

	n.obfuscationKey = append([]byte{}, bytes[nodeHeaderSize-nodeObfuscationKeySize:nodeHeaderSize]...)
	n.entry = append([]byte{}, bytes[nodeHeaderSize:nodeHeaderSize+32]...)
	offset := nodeHeaderSize + 32
	n.forks = make(map[byte]*fork)
	bb := &bitsForBytes{}
	bb.fromBytes(bytes[offset:])
	offset += 32
	bb.iter(func(b byte) error {
		f := &fork{}
		f.fromBytes(bytes[offset : offset+forkSize])
		n.forks[b] = f
		offset += forkSize
		return nil
	})

	return nil
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
	// using 1 byte ('nodeForkRefBytesSize') for size
	if len(r) > 256 {
		err = fmt.Errorf("node reference size > 256: %d", len(r))
		return
	}
	b = append(b, f.Node.nodeType)
	b = append(b, uint8(len(f.prefix)))

	prefixBytes := make([]byte, nodePrefixMaxSize)
	copy(prefixBytes, f.prefix)
	b = append(b, prefixBytes...)

	refBytes := make([]byte, nodeReferenceMaxSize)
	copy(refBytes, r)
	b = append(b, refBytes...)

	return b, nil
}

var refBytes = nodeRefBytes

func nodeRefBytes(f *fork) []byte {
	return f.Node.ref
}
