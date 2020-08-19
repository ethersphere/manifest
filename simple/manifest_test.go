// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simple_test

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"

	"github.com/ethersphere/manifest/simple"
)

// randomAddress generates a random address.
func randomAddress() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func TestNilPath(t *testing.T) {
	m := simple.NewManifest()
	n, err := m.Lookup("")
	if err == nil {
		t.Fatalf("expected error, got reference %s", n.Reference())
	}
}

// struct for manifest entries for test cases
type e struct {
	path      string
	reference string
}

var testCases = []struct {
	name    string
	entries []e // entries to add to manifest
}{
	{
		name:    "empty-manifest",
		entries: nil,
	},
	{
		name: "one-entry",
		entries: []e{
			{
				path:      "entry-1",
				reference: randomAddress(),
			},
		},
	},
	{
		name: "two-entries",
		entries: []e{
			{
				path:      "entry-1.txt",
				reference: randomAddress(),
			},
			{
				path:      "entry-2.png",
				reference: randomAddress(),
			},
		},
	},
	{
		name: "nested-entries",
		entries: []e{
			{
				path:      "text/robots.txt",
				reference: randomAddress(),
			},
			{
				path:      "img/1.png",
				reference: randomAddress(),
			},
			{
				path:      "img/2.jpg",
				reference: randomAddress(),
			},
			{
				path:      "readme.md",
				reference: randomAddress(),
			},
		},
	},
}

func TestEntries(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := simple.NewManifest()

			checkLength(t, m, 0)

			// add entries
			for i, e := range tc.entries {
				err := m.Add(e.path, e.reference)
				if err != nil {
					t.Fatal(err)
				}

				checkLength(t, m, i+1)
				checkEntry(t, m, e.reference, e.path)
			}

			manifestLen := m.Length()

			if len(tc.entries) != manifestLen {
				t.Fatalf("expected %d entries, found %d", len(tc.entries), manifestLen)
			}

			if manifestLen == 0 {
				// special case for empty manifest
				return
			}

			// replace entry
			lastEntry := tc.entries[len(tc.entries)-1]

			newReference := randomAddress()

			err := m.Add(lastEntry.path, newReference)
			if err != nil {
				t.Fatal(err)
			}

			checkLength(t, m, manifestLen) // length should not have changed
			checkEntry(t, m, newReference, lastEntry.path)

			// remove entries
			err = m.Remove("invalid/path.ext") // try removing inexistent entry
			if err != nil {
				t.Fatal(err)
			}

			checkLength(t, m, manifestLen) // length should not have changed

			for i, e := range tc.entries {
				err = m.Remove(e.path)
				if err != nil {
					t.Fatal(err)
				}

				entry, err := m.Lookup(e.path)
				if entry != nil || !errors.Is(err, simple.ErrNotFound) {
					t.Fatalf("expected path %v not to be present in the manifest, but it was found", e.path)
				}

				checkLength(t, m, manifestLen-i-1)
			}

		})
	}
}

// checkLength verifies that the given manifest length and integer match.
func checkLength(t *testing.T, m simple.Manifest, length int) {
	if m.Length() != length {
		t.Fatalf("expected length to be %d, but is %d instead", length, m.Length())
	}
}

// checkEntry verifies that an entry is equal to the one retrieved from the given manifest and path.
func checkEntry(t *testing.T, m simple.Manifest, reference string, path string) {
	n, err := m.Lookup(path)
	if err != nil {
		t.Fatal(err)
	}
	if n.Reference() != reference {
		t.Fatalf("expected reference %s, got: %s", reference, n.Reference())
	}
}

// TestMarshal verifies that created manifests are successfully marshalled and unmarshalled.
// This function wil add all test case entries to a manifest and marshal it.
// After, it will unmarshal the result, and verify that it is equal to the original manifest.
func TestMarshal(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := simple.NewManifest()

			for _, e := range tc.entries {
				err := m.Add(e.path, e.reference)
				if err != nil {
					t.Fatal(err)
				}
			}

			b, err := m.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			um := simple.NewManifest()
			if err := um.UnmarshalBinary(b); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(m, um) {
				t.Fatalf("marshalled and unmarshalled manifests are not equal: %v, %v", m, um)
			}
		})
	}
}
