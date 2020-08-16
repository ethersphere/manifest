// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mantaray

import (
	"bytes"
	"encoding/hex"
	mrand "math/rand"
	"testing"

	"golang.org/x/crypto/sha3"
)

const testMarshalOutput = "52fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c64950ac787fbce1061870e8d34e0a638bc7e812c7ca4ebd31d626a572ba47b06f6952fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c64952fdfc072182654f163f5f0fa0621d729566c74d10037c4d7bbb0407d1e2c64950f89d6640e3044f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c64952fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c64950ff9f642182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c64952fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c64850fc98072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c64952fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c64b50ff99622182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c64952fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c64a"

var testPrefixes = [][]byte{
	[]byte("aaaaa"),
	[]byte("cc"),
	[]byte("d"),
	[]byte("ee"),
}

func init() {
	obfuscationKeyFn = func(p []byte) (n int, err error) {
		return mrand.Read(p)
	}
}

func TestVersion01(t *testing.T) {
	hasher := sha3.NewLegacyKeccak256()

	_, err := hasher.Write([]byte(version01String))
	if err != nil {
		t.Fatal(err)
	}
	sum := hasher.Sum(nil)

	sumHex := hex.EncodeToString(sum)

	if version01HashString != sumHex {
		t.Fatalf("expecting version hash '%s', got '%s'", version01String, sumHex)
	}
}

func TestUnmarshal(t *testing.T) {
	input, _ := hex.DecodeString(testMarshalOutput)
	n := &Node{}
	err := n.UnmarshalBinary(input)
	if err != nil {
		t.Fatalf("expected no error marshaling, got %v", err)
	}

	expEncrypted := testMarshalOutput[128:192]
	// perform XOR decryption
	expEncryptedBytes, _ := hex.DecodeString(expEncrypted)
	expBytes := encryptDecrypt(expEncryptedBytes, n.obfuscationKey)
	exp := hex.EncodeToString(expBytes)

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
		e := append(make([]byte, 32-len(c)), c...)
		err := n.Add(c, e, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
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
