package mantaray

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	PathSeparator = '/' // path separator
)

// Node header fields constants.
const (
	// magicNode is 4 bytes at the head of a node.
	magicNode = 0x2D833CCB
	// magicNodeSize is the size in bytes of magicNode.
	magicNodeSize         = 4
	nodeFormatV1          = 1
	nodeFormatVersionSize = 1
	nodeForkRefBytesSize  = 1
	nodeHeaderPaddingSize = 26 // configured to make header size 64
	nodeNonceSize         = 32
	// nodeHeaderSize defines the total size of the header part.
	nodeHeaderSize = magicNodeSize + nodeFormatVersionSize + nodeForkRefBytesSize + nodeHeaderPaddingSize + nodeNonceSize
)

const (
	preambleSize = 64
	forkSize     = 64 + nodeForkRefBytesSize
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

	// header

	headerBytes := make([]byte, nodeHeaderSize)

	binary.BigEndian.PutUint32(headerBytes[:magicNodeSize], magicNode)
	headerBytes[4] = nodeFormatV1
	headerBytes[5] = nodeForkRefBytesSize

	if len(n.nonce) == 0 {
		// generate nonce
		nonce := make([]byte, nodeNonceSize)
		for i := 0; i < nodeNonceSize; {
			read, _ := nonceFn(nonce[i:])
			i += read
		}
		n.nonce = nonce
	}
	copy(headerBytes[nodeHeaderSize-nodeNonceSize:nodeHeaderSize], n.nonce)

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

	n.nonce = append([]byte{}, bytes[nodeHeaderSize-nodeNonceSize:nodeHeaderSize]...)
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

	// read info about fork node type
	// process fork node types sequentally
	// NOTE: this MUST come after reading forks
	err := bb.iter(func(b byte) error {
		n.forks[b].nodeType = uint8(bytes[offset])
		offset++
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (f *fork) fromBytes(b []byte) {
	f.prefix = bytesToPrefix(b[:32])
	refLen := int(uint8(b[32]))
	f.Node = NewNodeRef(b[33 : 33+refLen])
}

func (f *fork) bytes() (b []byte, err error) {
	r := refBytes(f)
	// using 1 byte ('nodeForkRefBytesSize') for size
	if len(r) > 256 {
		err = fmt.Errorf("node reference size > 256: %d", len(r))
		return
	}
	b = append(b, prefixToBytes(f.prefix)...)
	b = append(b, uint8(len(r)))
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
