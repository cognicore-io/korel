package korel

import "testing"

func TestDetectMode(t *testing.T) {
	tests := []struct {
		query string
		want  SearchMode
	}{
		{"what is kubernetes", ModeFact},
		{"who created golang", ModeFact},
		{"how does TLS work", ModeFact},
		{"define neural network", ModeFact},
		{"kubernetes vs docker", ModeCompare},
		{"compare rust and go", ModeCompare},
		{"difference between TCP and UDP", ModeCompare},
		{"AI trend 2024", ModeTrend},
		{"cloud adoption over time", ModeTrend},
		{"change in interest rates", ModeTrend},
		{"kubernetes security", ModeExplore},
		{"machine learning", ModeExplore},
		{"", ModeExplore},
	}

	for _, tt := range tests {
		got := DetectMode(tt.query)
		if got != tt.want {
			t.Errorf("DetectMode(%q) = %q, want %q", tt.query, got, tt.want)
		}
	}
}

func TestModeWeightsAllPositive(t *testing.T) {
	for _, mode := range []SearchMode{ModeAuto, ModeFact, ModeTrend, ModeCompare, ModeExplore} {
		mw := ModeWeights(mode)
		if mw.PMI <= 0 || mw.Cats <= 0 || mw.Recency <= 0 || mw.Authority <= 0 || mw.Len <= 0 {
			t.Errorf("ModeWeights(%q) has non-positive multiplier: %+v", mode, mw)
		}
		if mw.PMI > 2 || mw.Cats > 2 || mw.Recency > 2 || mw.Authority > 2 || mw.Len > 2 {
			t.Errorf("ModeWeights(%q) has unreasonably large multiplier: %+v", mode, mw)
		}
	}
}

func TestModeWeightsAutoIsNeutral(t *testing.T) {
	mw := ModeWeights(ModeAuto)
	if mw.PMI != 1.0 || mw.Cats != 1.0 || mw.Recency != 1.0 || mw.Authority != 1.0 || mw.Len != 1.0 {
		t.Errorf("ModeWeights(auto) should be all 1.0, got %+v", mw)
	}
}

func TestDetectModeCaseInsensitive(t *testing.T) {
	if got := DetectMode("What Is Kubernetes"); got != ModeFact {
		t.Errorf("DetectMode should be case-insensitive, got %q", got)
	}
	if got := DetectMode("KUBERNETES VS DOCKER"); got != ModeCompare {
		t.Errorf("DetectMode should be case-insensitive, got %q", got)
	}
}
