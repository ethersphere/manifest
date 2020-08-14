// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simple

// Entry is a JSON representation of a single manifest entry.
type Entry struct {
	Ref string `json:"reference"`
}

// NewEntry creates a new Entry struct and returns it.
func NewEntry(reference string) *Entry {
	return &Entry{
		Ref: reference,
	}
}

// Reference returns the address of the file in the entry.
func (me *Entry) Reference() string {
	return me.Ref
}
