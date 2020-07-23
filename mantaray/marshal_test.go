package mantaray

import (
	"bytes"
	"encoding/hex"
	"testing"
)

const testMarshalOutput = "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000003a0000000000000000000000000000000000000061616161612f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f000000000000000000000000000000000000000000000000000000000000000063632f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f0000000000000000000000000000000000000000000000000000000000000001642f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f000000000000000000000000000000000000000000000000000000000000000265652f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f0000000000000000000000000000000000000000000000000000000000000003"

var testPrefixes = [][]byte{
	[]byte("aaaaa"),
	[]byte("cc"),
	[]byte("d"),
	[]byte("ee"),
}

func TestUnmarshal(t *testing.T) {
	input, _ := hex.DecodeString(testMarshalOutput)
	n := &Node{}
	err := n.UnmarshalBinary(input)
	if err != nil {
		t.Fatalf("expected no error marshaling, got %v", err)
	}
	exp := testMarshalOutput[:64]
	if hex.EncodeToString(n.entry) != exp {
		t.Fatalf("expected %x, got %x", exp, n.entry)
	}
	if len(testPrefixes) != len(n.forks) {
		t.Fatalf("expected %d forks, got %d", len(testPrefixes), len(n.forks))
	}
	for _, prefix := range testPrefixes {
		f := n.forks[prefix[0]]
		if f == nil {
			t.Fatalf("expected to have  fork on byte %x", prefix[:1])
		}
		if !bytes.Equal(f.prefix, prefix) {
			t.Fatalf("expected prefix for byte %x to match %s, got %s", prefix[:1], prefix, f.prefix)
		}
	}
}
func TestMarshal(t *testing.T) {
	n := New()
	defer func(r func(*fork) []byte) { refBytes = r }(refBytes)
	i := uint8(0)
	refBytes = func(*fork) []byte {
		b := make([]byte, 32)
		b[31] = byte(i)
		i++
		return b
	}
	for i := 0; i < len(testPrefixes); i++ {
		c := testPrefixes[i]
		n.Add(c, c, nil)
	}
	b, err := n.MarshalBinary()
	if err != nil {
		t.Fatalf("expected no error marshaling, got %v", err)
	}
	exp, _ := hex.DecodeString(testMarshalOutput)
	if !bytes.Equal(b, exp) {
		t.Fatalf("expected marshalled output to match %x, got %x", exp, b)
	}
	// n = &Node{}
	// err = n.UnmarshalBinary(b)
	// if err != nil {
	// 	t.Fatalf("expected no error unmarshaling, got %v", err)
	// }

	// for j := 0; j < len(testCases); j++ {
	// 	d := testCases[j]
	// 	m, err := n.Lookup(d, nil)
	// 	if err != nil {
	// 		t.Fatalf("expected no error, got %v", err)
	// 	}
	// 	if !bytes.Equal(m, d) {
	// 		t.Fatalf("expected value %x, got %x", d, m)
	// 	}
	// }
}
