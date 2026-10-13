package domainlist

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"sync"
)

// Lookuper looks up the lists which a Merger merges, e.g. DomainListSet, FileLookuper which reads the lists
// lazily, and CachedLookuper which caches the lists of another Lookuper. The looked up lists must not be
// modified. A Lookuper is not required to be safe for concurrent use, wrap it by LockedLookuper if needed.
type Lookuper interface {
	// Lookup returns the list of the name, or an error wrapping ErrListNotFound if there is not.
	Lookup(name string) (DomainList, error)
	// All returns all lists, it yields an error and stops if it fails.
	All() iter.Seq2[NamedDomainList, error]
}

var (
	_ Lookuper = (*DomainListSet)(nil)
	_ Lookuper = (*FileLookuper)(nil)
	_ Lookuper = (*CachedLookuper)(nil)
	_ Lookuper = (*LockedLookuper)(nil)
)

// FileLookuper looks up the lists from the files in FS named by the list names, e.g. os.DirFS of the data
// directory of v2fly/domain-list-community. A file is read when its list is looked up. It is safe for
// concurrent use if FS is.
type FileLookuper struct {
	FS fs.FS
}

func (l *FileLookuper) Lookup(name string) (DomainList, error) {
	// A list name is a valid file name, which could not be a path.
	if !isListName(name) {
		return DomainList{}, fmt.Errorf("%w: %q", ErrListNotFound, name)
	}
	file, err := l.FS.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return DomainList{}, fmt.Errorf("%w: %q", ErrListNotFound, name)
	}
	if err != nil {
		return DomainList{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return DomainList{}, err
	}
	if info.IsDir() {
		return DomainList{}, fmt.Errorf("%w: %q is a directory", ErrListNotFound, name)
	}
	list, err := Read(file)
	if err != nil {
		return DomainList{}, fmt.Errorf("read %s: %w", name, err)
	}
	return *list, nil
}

// All reads the files in the root of FS in the directory order, the directory is read in batches.
// The files whose names are not list names are skipped, e.g. ".DS_Store" or "README.md".
func (l *FileLookuper) All() iter.Seq2[NamedDomainList, error] {
	return func(yield func(NamedDomainList, error) bool) {
		dir, err := l.FS.Open(".")
		if err != nil {
			yield(NamedDomainList{}, err)
			return
		}
		defer dir.Close()
		readDir, ok := dir.(fs.ReadDirFile)
		if !ok {
			yield(NamedDomainList{}, &fs.PathError{Op: "readdir", Path: ".", Err: errors.New("not implemented")})
			return
		}

		for {
			entries, err := readDir.ReadDir(64)
			for _, entry := range entries {
				if entry.IsDir() || !isListName(entry.Name()) {
					continue
				}
				list, err := l.Lookup(entry.Name())
				if err != nil {
					yield(NamedDomainList{}, err)
					return
				}
				if !yield(NamedDomainList{Name: entry.Name(), DomainList: list}, nil) {
					return
				}
			}
			if err == io.EOF {
				return
			}
			if err != nil {
				yield(NamedDomainList{}, err)
				return
			}
		}
	}
}

// CachedLookuper caches the lists looked up from an upstream Lookuper, so that each list is looked up
// from the upstream once unless it fails. After All iterates all lists once, they are iterated from the
// cache instead of the upstream. It is not safe for concurrent use, see LockedLookuper.
type CachedLookuper struct {
	upstream Lookuper
	lists    *DomainListSet
	complete bool // all lists are cached
}

func NewCachedLookuper(upstream Lookuper) *CachedLookuper {
	return &CachedLookuper{upstream: upstream, lists: NewDomainListSet()}
}

func (c *CachedLookuper) Lookup(name string) (DomainList, error) {
	if i, ok := c.lists.index[name]; ok {
		return c.lists.lists[i].DomainList, nil
	}
	list, err := c.upstream.Lookup(name)
	if err != nil {
		return DomainList{}, err
	}
	c.lists.put(NamedDomainList{Name: name, DomainList: list})
	return list, nil
}

// All iterates the upstream until all lists are cached, then it iterates the cache in the order of being cached.
func (c *CachedLookuper) All() iter.Seq2[NamedDomainList, error] {
	if c.complete {
		return c.lists.All()
	}
	return func(yield func(NamedDomainList, error) bool) {
		for list, err := range c.upstream.All() {
			if err != nil {
				yield(NamedDomainList{}, err)
				return
			}
			if i, ok := c.lists.index[list.Name]; ok {
				list = c.lists.lists[i]
			} else {
				c.lists.put(list)
			}
			if !yield(list, nil) {
				return
			}
		}
		c.complete = true
	}
}

// LockedLookuper makes a Lookuper safe for concurrent use by a mutex. All holds the mutex only while
// getting each list, so that the lists could be looked up in the loop.
type LockedLookuper struct {
	mu       sync.Mutex
	lookuper Lookuper
}

func NewLockedLookuper(lookuper Lookuper) *LockedLookuper {
	return &LockedLookuper{lookuper: lookuper}
}

func (l *LockedLookuper) Lookup(name string) (DomainList, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lookuper.Lookup(name)
}

func (l *LockedLookuper) All() iter.Seq2[NamedDomainList, error] {
	return func(yield func(NamedDomainList, error) bool) {
		// All might read the state of the Lookuper before iterating, e.g. CachedLookuper.
		l.mu.Lock()
		all := l.lookuper.All()
		l.mu.Unlock()
		next, stop := iter.Pull2(all)
		defer func() {
			// stop might run the rest of the iterator.
			l.mu.Lock()
			stop()
			l.mu.Unlock()
		}()
		for {
			l.mu.Lock()
			list, err, ok := next()
			l.mu.Unlock()
			if !ok || !yield(list, err) {
				return
			}
		}
	}
}
