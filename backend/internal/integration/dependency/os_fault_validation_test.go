package dependency

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func TestSpoolOSFaults(t *testing.T) {
	t.Run("close", func(t *testing.T) {
		f, _ := NewSpoolFactory(t.TempDir(), 1, 1<<20)
		s, _ := f.create(context.Background())
		if _, e := s.Write([]byte("spill")); e != nil {
			t.Fatal(e)
		}
		if e := s.file.Close(); e != nil {
			t.Fatal(e)
		}
		if e := s.close(); e != Unavailable {
			t.Fatal("closed descriptor was not reported", e)
		}
		// Test-only recovery of deliberately invalidated descriptor for cleanup.
		s.file = nil
		if e := s.close(); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("remove", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Unix permission injection; Windows ACL test not claimed")
		}
		if os.Geteuid() == 0 {
			t.Skip("permission denial requires unprivileged OS user")
		}
		dir := t.TempDir()
		f, _ := NewSpoolFactory(dir, 1, 1<<20)
		s, _ := f.create(context.Background())
		if _, e := s.Write([]byte("spill")); e != nil {
			t.Fatal(e)
		}
		if e := s.seal(); e != nil {
			t.Fatal(e)
		}
		if e := os.Chmod(dir, 0500); e != nil {
			t.Fatal(e)
		}
		defer os.Chmod(dir, 0700)
		if e := s.close(); e != Unavailable {
			t.Fatal("remove denial not surfaced; run as non-root", e)
		}
		if e := os.Chmod(dir, 0700); e != nil {
			t.Fatal(e)
		}
		if e := s.close(); e != nil {
			t.Fatal(e)
		}
	})
}

func TestSpoolFilesystemFull(t *testing.T) {
	dir := os.Getenv("AEGIS_FAULT_SPOOL_ROOT")
	if dir == "" {
		t.Skip("requires isolated capacity-limited filesystem")
	}
	if !strings.HasPrefix(filepath.Clean(dir), "/tmp/aegis-die505-full-") {
		t.Fatal("capacity test requires harness-owned isolated filesystem")
	}
	f, e := NewSpoolFactory(dir, 1, 64<<20)
	if e != nil {
		t.Fatal(e)
	}
	s, e := f.create(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := s.close(); e != nil {
			t.Error(e)
		}
	}()
	block := bytes.Repeat([]byte{0x5a}, 1<<20)
	for i := 0; i < 64; i++ {
		_, e = s.Write(block)
		if e != nil {
			if e != Unavailable {
				t.Fatal("expected real OS capacity failure, not configured quota", e)
			}
			proof := filepath.Join(dir, "enospc-proof")
			proofErr := os.WriteFile(proof, block, 0600)
			defer os.Remove(proof)
			if !errors.Is(proofErr, syscall.ENOSPC) {
				t.Fatal("OS ENOSPC proof absent", proofErr)
			}
			if e = s.seal(); e == nil {
				t.Fatal("failed spool sealed")
			}
			t.Logf("filesystem_full after_successful_bytes=%d", s.size)
			return
		}
	}
	t.Fatal("isolated filesystem did not exhaust")
}
