package golang

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/die"
)

// Test-only composition; the released adapter does not invoke analysis.
func TestOptInAnalysisWithReleasedWorkerFixtures(t *testing.T) {
	cfg, e := die.NewAnalysisConfig(die.AnalysisConfigParams{})
	if e != nil {
		t.Fatal(e)
	}
	analyzer, e := die.NewAnalyzer(cfg)
	if e != nil {
		t.Fatal(e)
	}
	var expected []byte
	for _, workers := range []int{1, 8} {
		p := fixture(t, workspace(), workers)
		in := inputs(t, p)
		inv, e := configured(t, ConfigParams{}).Analyze(context.Background(), in)
		if e != nil {
			t.Fatal(e)
		}
		before := marshal(t, inv)
		if inv.Analysis() != nil || len(inv.StrongComponents()) != 0 {
			t.Fatal("adapter ran algorithms")
		}
		out, e := analyzer.Analyze(context.Background(), inv)
		if e != nil {
			t.Fatal(e)
		}
		if string(before) != string(marshal(t, inv)) {
			t.Fatal("input changed")
		}
		b := marshal(t, out)
		if expected == nil {
			expected = b
		} else if string(expected) != string(b) {
			t.Fatal("worker fixture changed analysis")
		}
		for _, c := range out.Cycles() {
			if c.Classification != "structural" {
				t.Fatal("language classification")
			}
		}
	}
	t.Logf("adapter_workers=1,8 analysis_sha256=%x", sha256.Sum256(expected))
}
