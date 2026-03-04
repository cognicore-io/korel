// Package bootstrap provides shared engine setup for CLI binaries.
//
// Two entry points:
//   - Run(): full engine bootstrap (config → pipeline → inference → store → korel.New → WarmInference)
//   - LoadPipeline(): lightweight config + pipeline only (for analysis tools)
package bootstrap

import (
	"context"
	"fmt"
	"os"

	"github.com/cognicore/korel/pkg/korel"
	"github.com/cognicore/korel/pkg/korel/config"
	"github.com/cognicore/korel/pkg/korel/inference"
	prologinf "github.com/cognicore/korel/pkg/korel/inference/prolog"
	"github.com/cognicore/korel/pkg/korel/inference/simple"
	"github.com/cognicore/korel/pkg/korel/ingest"
	"github.com/cognicore/korel/pkg/korel/pmi"
	"github.com/cognicore/korel/pkg/korel/store"
	"github.com/cognicore/korel/pkg/korel/store/sqlite"
)

// Options configures the bootstrap sequence.
// Zero-value fields fall back to sensible defaults.
type Options struct {
	DBPath          string
	StoplistPath    string
	DictPath        string
	BaseDictPath    string
	TaxonomyPath    string
	RulesPath       string
	SimpleInference bool
	Weights         korel.ScoreWeights // zero → korel.DefaultScoreWeights()
	RecencyHalfLife float64            // zero → korel.DefaultRecencyHalfLife
	Graph           korel.GraphConfig  // zero → korel.DefaultGraphConfig()
	PMI             pmi.Config         // zero → pmi.DefaultConfig()
	SkipWarm        bool               // skip WarmInference (for indexers that call it later)
}

// Result holds the fully initialized engine and its dependencies.
type Result struct {
	Engine    *korel.Korel
	Store     store.Store
	Pipeline  *ingest.Pipeline
	Inference inference.Engine
}

// Close cleanly shuts down all resources.
func (r *Result) Close() {
	r.Engine.Close()
}

// PipelineResult holds config + pipeline without engine/store.
// Use for analysis tools that don't need the full engine.
type PipelineResult struct {
	Pipeline  *ingest.Pipeline
	Stopwords []string
}

// LoadPipeline loads config files and creates a processing pipeline.
// This is the lightweight alternative to Run() for tools that only need
// tokenization and parsing (e.g., analytics, corpus inspection).
func LoadPipeline(opts Options) (*PipelineResult, error) {
	components, err := loadConfig(opts)
	if err != nil {
		return nil, err
	}
	pipeline := ingest.NewPipeline(components.Tokenizer, components.Parser, components.Taxonomy)
	return &PipelineResult{
		Pipeline:  pipeline,
		Stopwords: components.Stopwords,
	}, nil
}

// Run performs the full engine bootstrap sequence:
//  1. Load config files (stoplist, dict, taxonomy, rules)
//  2. Build processing pipeline
//  3. Initialize inference engine (Prolog or simple)
//  4. Open SQLite store
//  5. Persist stopwords to store
//  6. Create Korel engine
//  7. Load persisted edges into inference (WarmInference), unless SkipWarm
func Run(ctx context.Context, opts Options) (*Result, error) {
	// 1-2. Load config + build pipeline
	components, err := loadConfig(opts)
	if err != nil {
		return nil, err
	}
	pipeline := ingest.NewPipeline(components.Tokenizer, components.Parser, components.Taxonomy)

	// 3. Initialize inference engine
	var inf inference.Engine
	if opts.SimpleInference {
		inf = simple.New()
	} else {
		inf, err = prologinf.New()
		if err != nil {
			return nil, fmt.Errorf("prolog engine: %w", err)
		}
	}
	if components.Rules != "" {
		data, err := os.ReadFile(components.Rules)
		if err != nil {
			return nil, fmt.Errorf("read rules: %w", err)
		}
		if err := inf.LoadRules(string(data)); err != nil {
			return nil, fmt.Errorf("load rules: %w", err)
		}
	}

	// 4. Open store
	st, err := sqlite.OpenSQLite(ctx, opts.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	// 5. Persist stopwords from config to the store so BuildGraph can use them
	if len(components.Stopwords) > 0 {
		if err := st.UpsertStoplist(ctx, components.Stopwords); err != nil {
			st.Close()
			return nil, fmt.Errorf("persist stoplist: %w", err)
		}
	}

	// 6. Create Korel instance (zero-value Options fields get defaults)
	engine := korel.New(korel.Options{
		Store:           st,
		Pipeline:        pipeline,
		Inference:       inf,
		Weights:         opts.Weights,
		RecencyHalfLife: opts.RecencyHalfLife,
		PMI:             opts.PMI,
		Graph:           opts.Graph,
	})

	// 7. Load persisted edges into the inference engine
	if !opts.SkipWarm {
		if err := engine.WarmInference(ctx); err != nil {
			st.Close()
			return nil, fmt.Errorf("warm inference: %w", err)
		}
	}

	return &Result{
		Engine:    engine,
		Store:     st,
		Pipeline:  pipeline,
		Inference: inf,
	}, nil
}

// loadConfig creates a config.Loader from Options and loads all components.
func loadConfig(opts Options) (*config.Components, error) {
	loader := config.Loader{
		StoplistPath: opts.StoplistPath,
		DictPath:     opts.DictPath,
		BaseDictPath: opts.BaseDictPath,
		TaxonomyPath: opts.TaxonomyPath,
		RulesPath:    opts.RulesPath,
	}
	components, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return components, nil
}
