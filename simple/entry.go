// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simple

// Entry is a representation of a single manifest entry.
type Entry interface {
	// Reference returns the address of the file in the entry.
	Reference() string
}

// entry is a JSON representation of a single manifest entry.
type entry struct {
	Ref string `json:"reference"`
}

// newEntry creates a new Entry struct and returns it.
func newEntry(reference string) *entry {
	return &entry{
		Ref: reference,
	}
}

func (me *entry) Reference() string {
	return me.Ref
}
