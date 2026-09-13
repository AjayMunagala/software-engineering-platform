// Package codec encodes accepted immutable dependency inventories without analysis.
package codec

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"hash"
	"io"
	"reflect"
	"unicode/utf8"

	"github.com/AjayMunagala/software-engineering-platform/backend/die"
)

const Name = "dependency-canonical-json"
const Version = "0.1.0"
const MaximumBytes uint64 = 4 << 30

type Error string

func (e Error) Error() string { return string(e) }

const (
	Invalid     Error = "invalid_input"
	Unsupported Error = "unsupported_contract"
	Limit       Error = "limit_exceeded"
	Unavailable Error = "unavailable"
)

type Receipt struct {
	size   uint64
	digest [32]byte
}

func (r Receipt) Size() uint64     { return r.size }
func (r Receipt) Digest() [32]byte { return r.digest }

type Encoder struct{ maximum uint64 }

func New(maximum uint64) (*Encoder, error) {
	if maximum == 0 {
		maximum = MaximumBytes
	}
	if maximum > MaximumBytes {
		return nil, Invalid
	}
	return &Encoder{maximum}, nil
}
func nilValue(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return r.IsNil()
	}
	return false
}

type stream struct {
	ctx           context.Context
	w             io.Writer
	h             hash.Hash
	size, maximum uint64
}

func (s *stream) write(b []byte) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if uint64(len(b)) > s.maximum-s.size {
		return Limit
	}
	n, err := s.w.Write(b)
	if n < 0 || n > len(b) {
		return Unavailable
	}
	s.size += uint64(n)
	s.h.Write(b[:n])
	if err != nil || n != len(b) {
		return Unavailable
	}
	return s.ctx.Err()
}

// checkText prevents encoding/json's lossy replacement of invalid UTF-8.
func checkText(ctx context.Context, v reflect.Value) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.String:
		if !utf8.ValidString(v.String()) {
			return Invalid
		}
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			return checkText(ctx, v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := checkText(ctx, v.Field(i)); err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := checkText(ctx, v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		it := v.MapRange()
		for it.Next() {
			if err := checkText(ctx, it.Key()); err != nil {
				return err
			}
			if err := checkText(ctx, it.Value()); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *stream) value(v any) error {
	if err := checkText(s.ctx, reflect.ValueOf(v)); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return Invalid
	}
	return s.write(b)
}
func array[T any](s *stream, values []T) error {
	if err := s.write([]byte("[")); err != nil {
		return err
	}
	for i, v := range values {
		if i > 0 {
			if err := s.write([]byte(",")); err != nil {
				return err
			}
		}
		if err := s.value(v); err != nil {
			return err
		}
	}
	return s.write([]byte("]"))
}
func (e *Encoder) Encode(ctx context.Context, inventory die.DependencyInventory, w io.Writer) (Receipt, error) {
	if ctx == nil || e == nil || e.maximum == 0 || nilValue(w) {
		return Receipt{}, Invalid
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	m := inventory.Metadata()
	if m.Name != die.ArtifactName || m.Version != die.ArtifactVersion || m.EngineName != die.EngineName || m.EngineVersion != die.EngineVersion || m.NodeIDSchemeVersion != "dependency-node-id/v1" || m.EdgeIDSchemeVersion != "dependency-edge-id/v1" || m.ContainmentIDSchemeVersion != "dependency-containment-id/v1" {
		return Receipt{}, Unsupported
	}
	a := inventory.Analysis()
	if a != nil && (a.EngineName != "dependency-graph-analysis" || a.EngineVersion != Version || a.DigestScheme != die.AnalysisDigestScheme || a.SCCIDScheme != "dependency-scc-id/v1" || a.CycleIDScheme != "dependency-cycle-id/v1" || a.ProjectionPolicy != "dependency-local-projection/v1") {
		return Receipt{}, Unsupported
	}
	s := &stream{ctx: ctx, w: w, h: sha256.New(), maximum: e.maximum}
	sections := []struct {
		name string
		emit func() error
	}{
		{"artifact", func() error { return s.value(m) }},
		{"source_artifacts", func() error { return array(s, inventory.SourceArtifacts()) }},
		{"nodes", func() error { return array(s, inventory.Nodes()) }},
		{"containment", func() error { return array(s, inventory.Containment()) }},
		{"dependencies", func() error { return array(s, inventory.Dependencies()) }},
		{"strong_components", func() error { return array(s, inventory.StrongComponents()) }},
		{"cycles", func() error { return array(s, inventory.Cycles()) }},
		{"diagnostics", func() error { return array(s, inventory.Diagnostics()) }},
		{"statistics", func() error { return s.value(inventory.Statistics()) }},
	}
	if a != nil {
		sections = append(sections, struct {
			name string
			emit func() error
		}{"analysis", func() error { return s.value(a) }})
	}
	for i, section := range sections {
		prefix := ","
		if i == 0 {
			prefix = "{"
		}
		if err := s.write([]byte(prefix + "\"" + section.name + "\":")); err != nil {
			return Receipt{}, err
		}
		if err := ctx.Err(); err != nil {
			return Receipt{}, err
		}
		if err := section.emit(); err != nil {
			return Receipt{}, err
		}
	}
	if err := s.write([]byte("}\n")); err != nil {
		return Receipt{}, err
	}
	r := Receipt{size: s.size}
	copy(r.digest[:], s.h.Sum(nil))
	return r, nil
}
