package maintenance

import (
	"context"
	"fmt"
	"strings"

	"github.com/cognicore/korel/pkg/korel/autotune/rules"
)

// RuleWriter persists approved rules to a destination (file, DB, etc.).
type RuleWriter interface {
	WriteRules(ctx context.Context, content string) error
}

// RuleExporter renders rule suggestions as Prolog-style facts.
type RuleExporter struct {
	Writer RuleWriter
}

func (e *RuleExporter) Export(ctx context.Context, suggs []rules.Suggestion) error {
	if e.Writer == nil {
		return fmt.Errorf("rule exporter: nil writer")
	}
	var b strings.Builder
	for _, sugg := range suggs {
		b.WriteString(fmt.Sprintf("%s(%s, %s). %% confidence %.2f support %d\n",
			sanitize(sugg.Relation), sanitize(sugg.Subject), sanitize(sugg.Object),
			sugg.Confidence, sugg.Support))
	}
	return e.Writer.WriteRules(ctx, b.String())
}

func sanitize(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}
