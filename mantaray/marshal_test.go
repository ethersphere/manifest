package mantaray

import (
	"bytes"
	"encoding/hex"
	mrand "math/rand"
	"testing"
)

const testMarshalOutput = "2d833ccb0101000000000000000000000000000000000000000000000000000052fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c64900000000000000000000000000000000000000000000000000000000000000000000000000000000000000003a0000000000000000000000000000000000000061616161612f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f20000000000000000000000000000000000000000000000000000000000000000063632f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f200000000000000000000000000000000000000000000000000000000000000001642f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f20000000000000000000000000000000000000000000000000000000000000000265652f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f2f20000000000000000000000000000000000000000000000000000000000000000302020202"

var testPrefixes = [][]byte{
	[]byte("aaaaa"),
	[]byte("cc"),
	[]byte("d"),
	[]byte("ee"),
}

func init() {
	nonceFn = func(p []byte) (n int, err error) {
		return mrand.Read(p)
	}
}

func TestUnmarshal(t *testing.T) {
	input, _ := hex.DecodeString(testMarshalOutput)
	n := &Node{}
	err := n.UnmarshalBinary(input)
	if err != nil {
		t.Fatalf("expected no error marshaling, got %v", err)
	}
	exp := testMarshalOutput[128:192]
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
