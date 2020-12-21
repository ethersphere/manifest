// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mantaray

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// EachNodeFunc is the type of the function called for each node visited
// by EachNodeAsync.
type EachNodeFunc func(path []byte, node *Node) error

func eachNodeFnCopyBytes(ctx context.Context, path []byte, node *Node, eachNodeFn EachNodeFunc) error {
	return eachNodeFn(append(path[:0:0], path...), node)
}

// eachNodeAsync recursively descends path, calling eachNodeFn.
func eachNodeAsync(ctx context.Context, path []byte, l Loader, n *Node, eachNodeFn EachNodeFunc) error {
	if n.forks == nil {
		if err := n.load(ctx, l); err != nil {
			return err
		}
	}

	err := eachNodeFnCopyBytes(ctx, path, n, eachNodeFn)
	if err != nil {
		return err
	}

	eg, ectx := errgroup.WithContext(ctx)

	for _, f := range n.forks {
		f := f

		nextPath := append(path[:0:0], path...)
		nextPath = append(nextPath, f.prefix...)

		eg.Go(func() error {
			return eachNodeAsync(ectx, nextPath, l, f.Node, eachNodeFn)
		})
	}

	return eg.Wait()
}

// EachNodeAsync walks the node tree structure rooted at root, calling
// eachNodeFn for each node in the tree, including root.
func (n *Node) EachNodeAsync(ctx context.Context, root []byte, l Loader, eachNodeFn EachNodeFunc) error {
	node, err := n.LookupNode(ctx, root, l)
	if err != nil {
		return err
	}

	return eachNodeAsync(ctx, root, l, node, eachNodeFn)
}

// EachPathFunc is the type of the function called for each file or directory
// visited by EachPathAsync.
type EachPathFunc func(path []byte, isDir bool) error

func eachPathFnCopyBytes(path []byte, isDir bool, eachPathFn EachPathFunc) error {
	return eachPathFn(append(path[:0:0], path...), isDir)
}

// eachPathAsync recursively descends path, calling eachPathFn.
func eachPathAsync(ctx context.Context, path, prefix []byte, l Loader, n *Node, eachPathFn EachPathFunc) error {
	if n.forks == nil {
		if err := n.load(ctx, l); err != nil {
			return err
		}
	}

	nextPath := append(path[:0:0], path...)

	for i := 0; i < len(prefix); i++ {
		if prefix[i] == PathSeparator {
			// path ends with separator
			err := eachPathFnCopyBytes(nextPath, true, eachPathFn)
			if err != nil {
				return err
			}
		}
		nextPath = append(nextPath, prefix[i])
	}

	if n.IsValueType() {
		if nextPath[len(nextPath)-1] == PathSeparator {
			// path ends with separator; already reported
		} else {
			err := eachPathFnCopyBytes(nextPath, false, eachPathFn)
			if err != nil {
				return err
			}
		}
	}

	eg, ectx := errgroup.WithContext(ctx)

	if n.IsEdgeType() {
		for _, f := range n.forks {
			f := f

			eg.Go(func() error {
				return eachPathAsync(ectx, nextPath, f.prefix, l, f.Node, eachPathFn)
			})
		}
	}

	return eg.Wait()
}

// EachPathAsync walks the node tree structure rooted at root, calling eachPathFn
// for each file or directory in the tree, including root.
func (n *Node) EachPathAsync(ctx context.Context, root []byte, l Loader, eachPathFn EachPathFunc) error {
	node, err := n.LookupNode(ctx, root, l)
	if err != nil {
		return err
	}

	return eachPathAsync(ctx, root, []byte{}, l, node, eachPathFn)
}
