package mantaray

import (
	"errors"
	"sync"
)

var (
	// ErrNoSaver  saver interface not given
	ErrNoSaver = errors.New("Node is not persisted but no saver")
	// ErrNoLoader saver interface not given
	ErrNoLoader = errors.New("Node is reference but no loader")
)

// Loader  defines a generic interface to retrieve nodes
// from a persistent storage
// for read only  operations only
type Loader interface {
	Load([]byte) ([]byte, error)
}

// Saver  defines a generic interface to persist  nodes
// for write operations
type Saver interface {
	Save([]byte) ([]byte, error)
}

// LoadSaver is a composite interface of Loader and Saver
// it is meant to be implemented as  thin wrappers around persistent storage like Swarm
type LoadSaver interface {
	Loader
	Save([]byte) ([]byte, error)
}

func (n *Node) load(l Loader) error {
	if n == nil || n.ref == nil {
		return nil
	}
	if l == nil {
		return ErrNoLoader
	}
	b, err := l.Load(n.ref)
	if err != nil {
		return err
	}
	if err := n.UnmarshalBinary(b); err != nil {
		return err
	}
	return nil
}

// Save persists a trie recursively  traversing the nodes
func (n *Node) Save(s Saver) error {
	if s == nil {
		return ErrNoSaver
	}
	errc := make(chan error, 1)
	closed := make(chan struct{})
	n.save(s, errc, closed)
	select {
	case err := <-errc:
		return err
	default:
	}
	return nil

}

func (n *Node) save(s Saver, errc chan error, closed chan struct{}) {
	if n != nil && n.ref != nil {
		return
	}
	var wg sync.WaitGroup
	for _, f := range n.forks {
		wg.Add(1)
		go func(f *fork) {
			defer wg.Done()
			f.Node.save(s, errc, closed)
		}(f)
	}
	wg.Wait()
	select {
	case <-closed:
		return
	default:
	}
	bytes, err := n.MarshalBinary()
	if err == nil {
		n.ref, err = s.Save(bytes)
	}
	if err != nil {
		select {
		case errc <- err:
			close(closed)
		default:
		}
	}
	n.forks = nil
}
