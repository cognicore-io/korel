package maintenance

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cognicore/korel/pkg/korel/autotune/rules"
)

type fakeWriter struct {
	content string
	err     error
}

func (f *fakeWriter) WriteRules(ctx context.Context, content string) error {
	if f.err != nil {
		return f.err
	}
	f.content = content
	return nil
}

func TestRuleExporterWritesFacts(t *testing.T) {
	writer := &fakeWriter{}
	exporter := RuleExporter{Writer: writer}

	suggs := []rules.Suggestion{
		{Relation: "related_to", Subject: "machine-learning", Object: "neural-network", Confidence: 0.9, Support: 20},
	}

	if err := exporter.Export(context.Background(), suggs); err != nil {
		t.Fatalf("Export: %v", err)
	}

	if !strings.Contains(writer.content, "related_to(machine_learning, neural_network).") {
		t.Fatalf("unexpected export: %s", writer.content)
	}
}

func TestRuleExporterWriterError(t *testing.T) {
	exporter := RuleExporter{Writer: &fakeWriter{err: errors.New("fail")}}
	err := exporter.Export(context.Background(), []rules.Suggestion{})
	if err == nil {
		t.Fatal("expected error")
	}
}
