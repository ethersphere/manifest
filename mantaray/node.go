package mantaray

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// Error used when lookup path does not match
var (
	ErrNotFound  = errors.New("not found")
	ErrEmptyPath = errors.New("empty path")
)

// Node represents a mantaray Node
type Node struct {
	ref   []byte // reference to uninstantiated Node persisted serialised
	entry []byte
	forks map[byte]*fork
}

type fork struct {
	prefix []byte // the non-branching part of the subpath
	*Node         // in memory structure that represents the Node
}

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

// Lookup finds the entry for a path or returns error if not found
func (n *Node) Lookup(path []byte, l Loader) ([]byte, error) {
	if n.forks == nil {
		if err := n.load(l); err != nil {
			return nil, err
		}
	}
	if len(path) == 0 {
		return n.entry, nil
	}
	f := n.forks[path[0]]
	if f == nil {
		return nil, notFound(path)
	}
	c := common(f.prefix, path)
	if len(c) == len(f.prefix) {
		return f.Node.Lookup(path[len(c):], l)
	}
	return nil, notFound(path)
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
			n.forks[path[0]] = &fork{prefix, nn}
			return nil
		}
		nn.entry = entry
		n.forks[path[0]] = &fork{path, nn}
		return nil
	}
	c := common(f.prefix, path)
	rest := f.prefix[len(c):]
	nn := f.Node
	if len(rest) > 0 {
		nn = New()
		nn.forks[rest[0]] = &fork{rest, f.Node}
	}
	err := nn.Add(path[len(c):], entry, ls)
	if err != nil {
		return err
	}
	n.forks[path[0]] = &fork{c, nn}
	return nil
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
