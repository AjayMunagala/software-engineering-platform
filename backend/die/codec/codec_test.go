package codec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/AjayMunagala/software-engineering-platform/backend/die"
	"io"
	"os"
	"reflect"
	"testing"
)

type vector struct {
	Name    string `json:"name"`
	Payload string `json:"payload_utf8"`
	Size    uint64 `json:"payload_size"`
	SHA     string `json:"payload_sha256"`
}

func vectors(t testing.TB) []vector {
	t.Helper()
	b, err := os.ReadFile("../../../experiments/dependency-platform-vectors/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct{ Vectors []vector }
	if err = json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	return data.Vectors
}
func inventory(t testing.TB) die.DependencyInventory {
	t.Helper()
	c, _ := die.NewConfig(die.ConfigParams{})
	core, _ := die.New(c)
	v, err := core.Normalize(context.Background(), die.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestLiteralInventories(t *testing.T) {
	e, _ := New(0)
	base := inventory(t)
	ac, _ := die.NewAnalysisConfig(die.AnalysisConfigParams{})
	a, _ := die.NewAnalyzer(ac)
	analyzed, err := a.Analyze(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors(t) {
		if v.Name != "empty" && v.Name != "analyzed_empty" {
			continue
		}
		t.Run(v.Name, func(t *testing.T) {
			input := base
			if v.Name == "analyzed_empty" {
				input = analyzed
			}
			before, _ := json.Marshal(input.View())
			var b bytes.Buffer
			r, err := e.Encode(context.Background(), input, &b)
			if err != nil {
				t.Fatal(err)
			}
			if b.String() != v.Payload || r.Size() != v.Size || hex.EncodeToString(r.digest[:]) != v.SHA {
				t.Fatalf("literal mismatch: size %d", r.Size())
			}
			after, _ := json.Marshal(input.View())
			if !bytes.Equal(before, after) {
				t.Fatal("mutated input")
			}
		})
	}
}

// Synthetic wire cases are tested against the same record writer, without
// manufacturing a private immutable inventory or accepting a public decoder.
func TestLiteralWireRecords(t *testing.T) {
	for _, v := range vectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			var view die.DependencyInventoryView
			if err := json.Unmarshal([]byte(v.Payload), &view); err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			s := stream{ctx: context.Background(), w: &b, h: sha256.New(), maximum: MaximumBytes}
			if err := s.value(view); err != nil {
				t.Fatal(err)
			}
			if err := s.write([]byte("\n")); err != nil {
				t.Fatal(err)
			}
			if b.String() != v.Payload {
				t.Fatal("literal wire mismatch")
			}
		})
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(b []byte) (int, error) { return f(b) }
func TestErrorsAndLimits(t *testing.T) {
	base := inventory(t)
	e, _ := New(0)
	if _, err := New(MaximumBytes + 1); err != Invalid {
		t.Fatal(err)
	}
	var nilBuffer *bytes.Buffer
	for _, w := range []io.Writer{nil, nilBuffer} {
		if _, err := e.Encode(context.Background(), base, w); err != Invalid {
			t.Fatal(err)
		}
	}
	if _, err := e.Encode(nil, base, io.Discard); err != Invalid {
		t.Fatal(err)
	}
	if _, err := (*Encoder)(nil).Encode(context.Background(), base, io.Discard); err != Invalid {
		t.Fatal(err)
	}
	if _, err := e.Encode(context.Background(), die.DependencyInventory{}, io.Discard); err != Unsupported {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.Encode(ctx, base, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, limit := range []uint64{1, 613, 614, 615} {
		enc, _ := New(limit)
		r, err := enc.Encode(context.Background(), base, io.Discard)
		if limit < 614 && err != Limit {
			t.Fatal(limit, err)
		}
		if limit >= 614 && (err != nil || r.Size() != 614) {
			t.Fatal(limit, err)
		}
	}
	for _, w := range []writerFunc{func(b []byte) (int, error) { return 0, nil }, func(b []byte) (int, error) { return -1, nil }, func(b []byte) (int, error) { return len(b) + 1, nil }, func(b []byte) (int, error) { return 0, errors.New("secret") }} {
		if _, err := e.Encode(context.Background(), base, w); err != Unavailable {
			t.Fatal(err)
		}
	}
	for _, v := range []any{"\xff", []string{"\xff"}, map[string]string{"\xff": "x"}, map[string]string{"x": "\xff"}, struct{ X string }{"\xff"}} {
		if err := checkText(context.Background(), reflect.ValueOf(v)); err != Invalid {
			t.Fatal(err)
		}
	}
	var b bytes.Buffer
	s := stream{ctx: context.Background(), w: &b, h: sha256.New(), maximum: 100}
	if err := s.value(make(chan int)); err != Invalid {
		t.Fatal(err)
	}
}
func TestRecordArrayFailuresAndCancellation(t *testing.T) {
	for budget := uint64(1); budget < 20; budget++ {
		s := stream{ctx: context.Background(), w: io.Discard, h: sha256.New(), maximum: budget}
		_ = array(&s, []string{"abc", "def"})
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := writerFunc(func(b []byte) (int, error) { cancel(); return len(b), nil })
	e, _ := New(0)
	if _, err := e.Encode(ctx, inventory(t), w); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func FuzzRecordEncoding(f *testing.F) {
	f.Add("π<&", uint16(32))
	f.Add("\xff", uint16(1))
	f.Fuzz(func(t *testing.T, value string, limit uint16) {
		var b bytes.Buffer
		s := stream{ctx: context.Background(), w: &b, h: sha256.New(), maximum: uint64(limit) + 1}
		err := s.value(value)
		if err == nil && !json.Valid(b.Bytes()) {
			t.Fatal("invalid JSON")
		}
		if uint64(b.Len()) > uint64(limit)+1 {
			t.Fatal("limit bypass")
		}
	})
}
func BenchmarkEmpty(b *testing.B) {
	v := inventory(b)
	e, _ := New(0)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := e.Encode(context.Background(), v, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}
