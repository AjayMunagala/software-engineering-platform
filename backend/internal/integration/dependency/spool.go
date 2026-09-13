package dependency

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type SpoolFactory struct {
	directory       string
	memory, maximum uint64
}

func NewSpoolFactory(directory string, memory, maximum uint64) (*SpoolFactory, error) {
	if memory == 0 {
		memory = 64 << 20
	}
	if maximum == 0 {
		maximum = 4 << 30
	}
	if !filepath.IsAbs(directory) || memory == 0 || memory > 64<<20 || maximum == 0 || maximum > 4<<30 || memory > maximum {
		return nil, Invalid
	}
	return &SpoolFactory{filepath.Clean(directory), memory, maximum}, nil
}
func (f *SpoolFactory) String() string   { return "dependency spool factory (redacted)" }
func (f *SpoolFactory) GoString() string { return f.String() }

type spool struct {
	mu                     sync.Mutex
	factory                *SpoolFactory
	ctx                    context.Context
	memory                 bytes.Buffer
	root                   *os.Root
	file                   *os.File
	name                   string
	size                   uint64
	h                      hash.Hash
	digest                 [32]byte
	sealed, closed, poison bool
	owned                  bool
	readers                int
}

func (f *SpoolFactory) create(ctx context.Context) (*spool, error) {
	if f == nil || ctx == nil {
		return nil, Invalid
	}
	if err := ctx.Err(); err != nil {
		return nil, safe(err)
	}
	return &spool{factory: f, ctx: ctx, h: sha256.New()}, nil
}
func (s *spool) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.sealed || s.poison {
		return 0, Conflict
	}
	if err := s.ctx.Err(); err != nil {
		return 0, safe(err)
	}
	if uint64(len(p)) > s.factory.maximum-s.size {
		s.poison = true
		return 0, Limit
	}
	if s.file == nil && s.size+uint64(len(p)) > s.factory.memory {
		root, err := os.OpenRoot(s.factory.directory)
		if err != nil {
			s.poison = true
			return 0, Unavailable
		}
		s.root = root
		var random [16]byte
		if _, err = rand.Read(random[:]); err != nil {
			s.poison = true
			return 0, Unavailable
		}
		s.name = "dep-" + hex.EncodeToString(random[:])
		s.file, err = root.OpenFile(s.name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err != nil {
			s.poison = true
			return 0, Unavailable
		}
		s.owned = true
		n, err := s.file.Write(s.memory.Bytes())
		if err != nil || n != s.memory.Len() {
			s.poison = true
			return 0, Unavailable
		}
		s.memory = bytes.Buffer{}
	}
	var n int
	var err error
	if s.file != nil {
		n, err = s.file.Write(p)
	} else {
		n, err = s.memory.Write(p)
	}
	if n > 0 {
		s.h.Write(p[:n])
		s.size += uint64(n)
	}
	if err != nil || n != len(p) {
		s.poison = true
		return n, Unavailable
	}
	return n, nil
}
func (s *spool) seal() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poison {
		return Conflict
	}
	if s.sealed {
		return nil
	}
	if err := s.ctx.Err(); err != nil {
		return safe(err)
	}
	if s.file != nil {
		if err := s.file.Sync(); err != nil {
			s.poison = true
			return Unavailable
		}
		if err := s.file.Close(); err != nil {
			s.poison = true
			return Unavailable
		}
		s.file = nil
	}
	copy(s.digest[:], s.h.Sum(nil))
	s.sealed = true
	return nil
}

type spoolReader struct {
	io.Reader
	close func() error
	once  sync.Once
	err   error
}

func (r *spoolReader) Close() error { r.once.Do(func() { r.err = r.close() }); return r.err }
func (s *spool) open(ctx context.Context) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx == nil {
		return nil, Invalid
	}
	if err := ctx.Err(); err != nil {
		return nil, safe(err)
	}
	if !s.sealed || s.closed {
		return nil, Conflict
	}
	var r io.Reader = bytes.NewReader(s.memory.Bytes())
	var file *os.File
	if s.root != nil {
		var err error
		file, err = s.root.Open(s.name)
		if err != nil {
			return nil, Unavailable
		}
		r = file
	}
	s.readers++
	return &spoolReader{Reader: r, close: func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.readers--
		if file != nil {
			if file.Close() != nil {
				return Unavailable
			}
		}
		return nil
	}}, nil
}
func (s *spool) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if s.readers > 0 {
		return Busy
	}
	if s.file != nil {
		if s.file.Close() != nil {
			return Unavailable
		}
		s.file = nil
	}
	if s.root != nil {
		if s.owned {
			if err := s.root.Remove(s.name); err != nil && !os.IsNotExist(err) {
				return Unavailable
			}
		}
		if s.root.Close() != nil {
			return Unavailable
		}
		s.root = nil
	}
	s.memory = bytes.Buffer{}
	s.closed = true
	return nil
}
