// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simple

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// EachEntryFunc is the type of the function called for each entry visited
// by EachEntryAsync.
type EachEntryFunc func(path string, entry Entry) error

func (m *manifest) EachEntryAsync(ctx context.Context, root string, walkFn EachEntryFunc) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	eg, _ := errgroup.WithContext(ctx)

	for k, v := range m.Entries {
		k := k
		v := v

		entry := newEntry(v.Ref, v.Meta)

		eg.Go(func() error {
			return walkFn(k, entry)
		})
	}

	return eg.Wait()
}
