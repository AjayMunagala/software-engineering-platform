package lie

import (
	"fmt"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// EngineMetadata identifies one registered language engine.
type EngineMetadata struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Language     string `json:"language"`
	ArtifactName string `json:"artifact_name"`
	Description  string `json:"description"`
}

// Severity indicates the classification of a diagnostic.
type Severity uint8

const (
	SeverityWarning Severity = 1
	SeverityError   Severity = 2
)

func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

// Position represents a zero-based byte offset and one-based line and byte column.
type Position struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

func (p Position) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Column) }

// SourceRange is inclusive at Start and exclusive at End.
type SourceRange struct {
	File  string   `json:"file"`
	Start Position `json:"start"`
	End   Position `json:"end"`
}

func (r SourceRange) String() string {
	if r.File == "" {
		return fmt.Sprintf("%s-%s", r.Start, r.End)
	}
	return fmt.Sprintf("%s:%s-%s", r.File, r.Start, r.End)
}

// Diagnostic represents a language-independent analysis diagnostic.
type Diagnostic struct {
	Engine   string       `json:"engine"`
	Severity Severity     `json:"severity"`
	Code     string       `json:"code"`
	Message  string       `json:"message"`
	Location *SourceRange `json:"location,omitempty"`
}

func (d Diagnostic) String() string {
	if d.Location != nil {
		return fmt.Sprintf("[%s][%s] %s (%s): %s", d.Engine, d.Severity, d.Code, d.Location, d.Message)
	}
	return fmt.Sprintf("[%s][%s] %s: %s", d.Engine, d.Severity, d.Code, d.Message)
}

// RunReport records orchestration timing, executed engines, and published artifacts.
type RunReport struct {
	startedAt        time.Time
	finishedAt       time.Time
	engines          []EngineMetadata
	published        []rie.ArtifactReference
	fatalDiagnostics []Diagnostic
}

func newRunReport(startedAt time.Time) RunReport {
	return RunReport{
		startedAt:        startedAt.UTC(),
		engines:          []EngineMetadata{},
		published:        []rie.ArtifactReference{},
		fatalDiagnostics: []Diagnostic{},
	}
}

func (r RunReport) StartedAt() time.Time { return r.startedAt }

func (r RunReport) FinishedAt() time.Time { return r.finishedAt }

func (r RunReport) DurationMilliseconds() float64 {
	if r.finishedAt.IsZero() {
		return 0
	}
	return float64(r.finishedAt.Sub(r.startedAt)) / float64(time.Millisecond)
}

func (r RunReport) Engines() []EngineMetadata {
	return append([]EngineMetadata(nil), r.engines...)
}

func (r RunReport) Published() []rie.ArtifactReference {
	return append([]rie.ArtifactReference(nil), r.published...)
}

func (r RunReport) FatalDiagnostics() []Diagnostic {
	return cloneDiagnostics(r.fatalDiagnostics)
}

func cloneDiagnostics(source []Diagnostic) []Diagnostic {
	result := make([]Diagnostic, len(source))
	for i, diagnostic := range source {
		result[i] = diagnostic
		if diagnostic.Location != nil {
			location := *diagnostic.Location
			result[i].Location = &location
		}
	}
	return result
}
