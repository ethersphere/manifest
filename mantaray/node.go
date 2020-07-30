package mantaray

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// Error used when lookup path does not match
var (
	ErrNotFound  = errors.New("not found")
	ErrEmptyPath = errors.New("empty path")
)

// Node represents a mantaray Node
type Node struct {
	nodeType uint8
	nonce    []byte
	ref      []byte // reference to uninstantiated Node persisted serialised
	entry    []byte
	forks    map[byte]*fork
}

type fork struct {
	prefix []byte // the non-branching part of the subpath
	*Node         // in memory structure that represents the Node
}

const (
	nodeTypeValue             = uint8(2)
	nodeTypeEdge              = uint8(4)
	nodeTypeWithPathSeparator = uint8(8)

	nodeTypeMask = uint8(255)
)

// NewNodeRef is the exported Node constructor used to represent manifests by reference
func NewNodeRef(ref []byte) *Node {
	return &Node{ref: ref}
}

// New is the constructor for in-memory Node structure
func New() *Node {
	return &Node{forks: make(map[byte]*fork)}
}

func notFound(path []byte) error {
	return fmt.Errorf("entry on '%s' ('%x'): %w", path, path, ErrNotFound)
}

func (n *Node) isValueType() bool {
	return n.nodeType&nodeTypeValue == nodeTypeValue
}

func (n *Node) isEdgeType() bool {
	return n.nodeType&nodeTypeEdge == nodeTypeEdge
}

func (n *Node) isWithPathSeparatorType() bool {
	return n.nodeType&nodeTypeWithPathSeparator == nodeTypeWithPathSeparator
}

func (n *Node) makeValue() {
	n.nodeType = n.nodeType | nodeTypeValue
}

func (n *Node) makeEdge() {
	n.nodeType = n.nodeType | nodeTypeEdge
}

func (n *Node) makeWithPathSeparator() {
	n.nodeType = n.nodeType | nodeTypeWithPathSeparator
}

func (n *Node) makeNotValue() {
	n.nodeType = (nodeTypeMask ^ nodeTypeValue) & n.nodeType
}

func (n *Node) makeNotEdge() {
	n.nodeType = (nodeTypeMask ^ nodeTypeEdge) & n.nodeType
}

func (n *Node) makeNotWithPathSeparator() {
	n.nodeType = (nodeTypeMask ^ nodeTypeWithPathSeparator) & n.nodeType
}

func (n *Node) SetNonce(nonce []byte) {
	bytes := make([]byte, 32)
	copy(bytes, nonce)
	n.nonce = bytes
}

// LookupNode finds the node for a path or returns error if not found
func (n *Node) LookupNode(path []byte, l Loader) (*Node, error) {
	if n.forks == nil {
		if err := n.load(l); err != nil {
			return nil, err
		}
	}
	if len(path) == 0 {
		return n, nil
	}
	f := n.forks[path[0]]
	if f == nil {
		return nil, notFound(path)
	}
	c := common(f.prefix, path)
	if len(c) == len(f.prefix) {
		return f.Node.LookupNode(path[len(c):], l)
	}
	return nil, notFound(path)
}

// Lookup finds the entry for a path or returns error if not found
func (n *Node) Lookup(path []byte, l Loader) ([]byte, error) {
	node, err := n.LookupNode(path, l)
	if err != nil {
		return nil, err
	}
	return node.entry, nil
}

// Add adds an entry to the path
func (n *Node) Add(path []byte, entry []byte, ls LoadSaver) error {
	if len(path) == 0 {
		n.entry = entry
		n.ref = nil
		return nil
	}
	if n.forks == nil {
		if err := n.load(ls); err != nil {
			return err
		}
		n.ref = nil
	}
	f := n.forks[path[0]]
	if f == nil {
		nn := New()
		// check for prefix size limit
		if len(path) > 32 {
			prefix := path[:32]
			rest := path[32:]
			err := nn.Add(rest, entry, ls)
			if err != nil {
				return err
			}
			nn.updateIsWithPathSeparator(prefix)
			n.forks[path[0]] = &fork{prefix, nn}
			n.makeEdge()
			return nil
		}
		nn.entry = entry
		nn.makeValue()
		nn.updateIsWithPathSeparator(path)
		n.forks[path[0]] = &fork{path, nn}
		n.makeEdge()
		return nil
	}
	c := common(f.prefix, path)
	rest := f.prefix[len(c):]
	nn := f.Node
	if len(rest) > 0 {
		// move current common prefix node
		nn = New()
		f.Node.updateIsWithPathSeparator(rest)
		nn.forks[rest[0]] = &fork{rest, f.Node}
		nn.makeEdge()
	}
	// NOTE: special case on edge split
	nn.updateIsWithPathSeparator(path)
	// add new for shared prefix
	err := nn.Add(path[len(c):], entry, ls)
	if err != nil {
		return err
	}
	n.forks[path[0]] = &fork{c, nn}
	n.makeEdge()
	return nil
}

func (n *Node) updateIsWithPathSeparator(path []byte) {
	if bytes.IndexRune(path, PathSeparator) > 0 {
		n.makeWithPathSeparator()
	} else {
		n.makeNotWithPathSeparator()
	}
}

// Remove removes a path from the node
func (n *Node) Remove(path []byte, ls LoadSaver) error {
	if len(path) == 0 {
		return ErrEmptyPath
	}
	if n.forks == nil {
		if err := n.load(ls); err != nil {
			return err
		}
	}
	f := n.forks[path[0]]
	if f == nil {
		return ErrNotFound
	}
	prefixIndex := bytes.Index(path, f.prefix)
	if prefixIndex != 0 {
		return ErrNotFound
	}
	rest := path[len(f.prefix):]
	if len(rest) == 0 {
		// full path matched
		delete(n.forks, path[0])
		return nil
	}
	return f.Node.Remove(rest, ls)
}

func common(a, b []byte) (c []byte) {
	for i := 0; i < len(a) && i < len(b) && a[i] == b[i]; i++ {
		c = append(c, a[i])
	}
	return c
}

// WalkNodeFunc is the type of the function called for each node visited
// by WalkNode.
type WalkNodeFunc func(path []byte, node *Node, err error) error

func walkNodeFnCopyBytes(path []byte, node *Node, err error, walkFn WalkNodeFunc) error {
	return walkFn(append(path[:0:0], path...), node, nil)
}

// walkNode recursively descends path, calling walkFn.
func walkNode(path []byte, l Loader, n *Node, walkFn WalkNodeFunc) error {
	if n.forks == nil {
		if err := n.load(l); err != nil {
			return err
		}
	}

	if n.isValueType() {
		err := walkNodeFnCopyBytes(path, n, nil, walkFn)
		if err != nil {
			return err
		}
	}

	if n.isEdgeType() {
		for _, v := range n.forks {
			nextPath := append(path[:0:0], path...)
			nextPath = append(nextPath, v.prefix...)

			err := walkNode(nextPath, l, v.Node, walkFn)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// WalkNode walks the node tree structure rooted at root, calling walkFn for
// each node in the tree, including root. All errors that arise visiting nodes
// are filtered by walkFn.
func (n *Node) WalkNode(root []byte, l Loader, walkFn WalkNodeFunc) error {
	node, err := n.LookupNode(root, l)
	if err != nil {
		err = walkFn(root, nil, err)
	} else {
		err = walkNode(root, l, node, walkFn)
	}
	return err
}

// WalkFunc is the type of the function called for each file or directory
// visited by Walk.
type WalkFunc func(path []byte, isDir bool, err error) error

func walkFnCopyBytes(path []byte, isDir bool, err error, walkFn WalkFunc) error {
	return walkFn(append(path[:0:0], path...), isDir, nil)
}

// walk recursively descends path, calling walkFn.
func walk(path, prefix []byte, l Loader, n *Node, walkFn WalkFunc) error {
	if n.forks == nil {
		if err := n.load(l); err != nil {
			return err
		}
	}

	nextPath := append(path[:0:0], path...)

	for i := 0; i < len(prefix); i++ {
		if prefix[i] == PathSeparator {
			// path ends with separator
			err := walkFnCopyBytes(nextPath, true, nil, walkFn)
			if err != nil {
				return err
			}
		}
		nextPath = append(nextPath, prefix[i])
	}

	if n.isValueType() {
		if nextPath[len(nextPath)-1] == PathSeparator {
			// path ends with separator; already reported
		} else {
			err := walkFnCopyBytes(nextPath, false, nil, walkFn)
			if err != nil {
				return err
			}
		}
	}

	if n.isEdgeType() {
		for _, v := range n.forks {
			err := walk(nextPath, v.prefix, l, v.Node, walkFn)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Walk walks the node tree structure rooted at root, calling walkFn for
// each file or directory in the tree, including root. All errors that arise
// visiting files and directories are filtered by walkFn.
func (n *Node) Walk(root []byte, l Loader, walkFn WalkFunc) error {
	node, err := n.LookupNode(root, l)
	if err != nil {
		err = walkFn(root, false, err)
	} else {
		err = walk(root, []byte{}, l, node, walkFn)
	}
	return err
}

func (n *Node) String() string {
	buf := bytes.NewBuffer(nil)
	io.WriteString(buf, tableCharsMap["bottom-left"])
	io.WriteString(buf, tableCharsMap["bottom"])
	io.WriteString(buf, tableCharsMap["top-right"])
	io.WriteString(buf, "\n")
	nodeStringWithPrefix(n, "  ", buf)
	return buf.String()
}

func nodeStringWithPrefix(n *Node, prefix string, writer io.Writer) {
	io.WriteString(writer, prefix)
	io.WriteString(writer, tableCharsMap["left-mid"])
	io.WriteString(writer, fmt.Sprintf("r: '%x'\n", n.ref))
	io.WriteString(writer, prefix)
	io.WriteString(writer, tableCharsMap["left-mid"])
	io.WriteString(writer, fmt.Sprintf("t: '%s'", strconv.FormatInt(int64(n.nodeType), 2)))
	io.WriteString(writer, fmt.Sprint(" ["))
	if n.isValueType() {
		io.WriteString(writer, fmt.Sprint(" Value"))
	}
	if n.isEdgeType() {
		io.WriteString(writer, fmt.Sprint(" Edge"))
	}
	if n.isWithPathSeparatorType() {
		io.WriteString(writer, fmt.Sprint(" PathSeparator"))
	}
	io.WriteString(writer, fmt.Sprint(" ]"))
	io.WriteString(writer, fmt.Sprint("\n"))
	io.WriteString(writer, prefix)
	if len(n.forks) == 0 {
		io.WriteString(writer, tableCharsMap["bottom-left"])
	} else {
		io.WriteString(writer, tableCharsMap["left-mid"])
	}
	io.WriteString(writer, fmt.Sprintf("e: '%s'\n", string(n.entry)))
	counter := 0
	for k, f := range n.forks {
		isLast := counter != len(n.forks)-1
		io.WriteString(writer, prefix)
		if isLast {
			io.WriteString(writer, tableCharsMap["left-mid"])
		} else {
			io.WriteString(writer, tableCharsMap["bottom-left"])
		}
		io.WriteString(writer, tableCharsMap["mid"])
		io.WriteString(writer, fmt.Sprintf("[%s]", string(k)))
		io.WriteString(writer, tableCharsMap["mid"])
		io.WriteString(writer, tableCharsMap["top-mid"])
		io.WriteString(writer, tableCharsMap["mid"])
		io.WriteString(writer, fmt.Sprintf("`%s`\n", string(f.prefix)))
		newPrefix := prefix
		if isLast {
			newPrefix += tableCharsMap["middle"]
		} else {
			newPrefix += " "
		}
		newPrefix += "     "
		nodeStringWithPrefix(f.Node, newPrefix, writer)
		counter++
	}
}

var tableCharsMap = map[string]string{
	"top":          "─",
	"top-mid":      "┬",
	"top-left":     "┌",
	"top-right":    "┐",
	"bottom":       "─",
	"bottom-mid":   "┴",
	"bottom-left":  "└",
	"bottom-right": "┘",
	"left":         "│",
	"left-mid":     "├",
	"mid":          "─",
	"mid-mid":      "┼",
	"right":        "│",
	"right-mid":    "┤",
	"middle":       "│",
}
