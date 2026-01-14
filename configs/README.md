# Configuration Files

This directory contains default configuration files used by Korel's bootstrap process.

## Stopword Files (Language-Specific)

Base stopword lists are used during bootstrap to filter out common words when generating taxonomy keywords.

### Available Languages

- **stopwords-en.yaml** - English stopwords (default)

### Usage

```bash
# Use default English stopwords
go run ./cmd/bootstrap --input=corpus.jsonl --domain=mydomain --output=configs/mydomain/

# Use custom or different language stopwords
go run ./cmd/bootstrap \
  --input=corpus.jsonl \
  --domain=mydomain \
  --output=configs/mydomain/ \
  --base-stoplist=configs/stopwords-de.yaml
```

### Creating Custom Stopword Lists

To create a stopword list for another language:

1. Copy `stopwords-en.yaml` as a template
2. Replace terms with common words in your target language
3. Save as `stopwords-{lang}.yaml` (e.g., `stopwords-de.yaml`, `stopwords-fr.yaml`)

Example for German:

```yaml
# configs/stopwords-de.yaml
terms:
  - der
  - die
  - das
  - den
  - dem
  - des
  - ein
  - eine
  - einen
  - einem
  # ... more German stopwords
```

### Minimal Stoplist for Testing

If you want minimal filtering during bootstrap (useful for domain-specific corpora), create a minimal stoplist:

```yaml
# configs/stopwords-minimal.yaml
terms:
  - the
  - a
  - and
  - of
```

Then use: `--base-stoplist=configs/stopwords-minimal.yaml`

## Synonym Files (Lexicon Configuration)

Synonym files define vocabulary normalization rules for your corpus. They map variant forms to canonical tokens and track contextual relationships (c-tokens).

### Available Templates

- **synonyms.example.yaml** - Example lexicon with best practices and common patterns

### What Synonyms Do

**Before normalization:**
```
tokens: ["machine", "learning", "ML", "game", "games", "gaming"]
```

**After normalization:**
```
tokens: ["ml", "ml", "ml", "game", "game", "game"]
```

Benefits:
- Reduces vocabulary size (fewer unique tokens)
- Improves statistics (higher token frequencies)
- Enables query expansion (search variants automatically)

### Bootstrap Auto-Generation

The bootstrap tool can suggest synonym candidates based on corpus analysis:

```bash
go run ./cmd/bootstrap \
  --input=corpus.jsonl \
  --domain=mydomain \
  --output=configs/mydomain/ \
  --synonym-limit=20 \
  --synonym-min-support=3 \
  --synonym-min-pmi=1.5
```

This generates `configs/mydomain/synonyms.yaml` with suggested synonym groups.

**⚠️ Important**: Auto-generated synonyms are **candidates** that require manual review:
- ✅ Keep true synonyms: `game/games/gaming`
- ❌ Remove collocations: `machine/learning` (appear together, not synonyms)
- ❌ Remove false positives from statistical clustering

### Creating Custom Synonym Files

Use `synonyms.example.yaml` as a template:

```yaml
synonyms:
  # Your domain-specific terms
  - canonical: ml
    variants: [ml, ML, machine learning, machine-learning]

  - canonical: game
    variants: [game, games, gaming, gamer, gamers]

  - canonical: api
    variants: [api, API, apis, APIs]
```

**Format Rules:**
- `canonical` - The normalized form (lowercase recommended)
- `variants` - List of alternative forms (includes canonical)
- Case-insensitive matching automatically applied

### Using Synonym Files

**In bootstrap**: Synonyms are auto-generated during bootstrap

**In code**: Load and apply to tokenizer
```go
import "github.com/cognicore/korel/pkg/korel/lexicon"

// Load lexicon from YAML
lex, err := lexicon.LoadFromYAML("configs/mydomain/synonyms.yaml")
if err != nil {
    log.Fatal(err)
}

// Apply to tokenizer
tokenizer := ingest.NewTokenizer(stopwords)
tokenizer.SetLexicon(lex)

// Now tokenization includes normalization
tokens := tokenizer.Tokenize("Machine Learning and gaming")
// Result: ["ml", "game"] (normalized + stopwords filtered)
```

### Bootstrap Configuration Flags

Control synonym generation with these flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--synonym-limit` | 20 | Maximum synonym candidates to suggest |
| `--synonym-min-support` | 3 | Minimum co-occurrence count |
| `--synonym-min-pmi` | 1.5 | Minimum PMI threshold for relatedness |

**Higher thresholds** = fewer but higher-quality suggestions
**Lower thresholds** = more candidates but more false positives

### Best Practices

1. **Start with bootstrap**: Let the tool suggest candidates from your corpus
2. **Review carefully**: Not all high-PMI pairs are synonyms
3. **Domain-specific**: Include field-specific abbreviations and jargon
4. **Morphology**: Add singular/plural, verb forms (analyze/analysis/analytical)
5. **Test iteratively**: Measure search improvement after adding synonyms
6. **Version control**: Track changes to understand impact

### Common Patterns

**Abbreviations and expansions:**
```yaml
- canonical: ml
  variants: [ml, ML, machine learning, machine-learning]
```

**Morphological variants:**
```yaml
- canonical: analyze
  variants: [analyze, analysis, analytical, analyzer, analysing, analysed]
```

**Technical terms:**
```yaml
- canonical: api
  variants: [api, API, apis, APIs]
```

**Multi-token phrases:**
```yaml
- canonical: anova
  variants: [anova, ANOVA, analysis of variance]
```

### Documentation

- **[docs/LEXICON.md](../docs/LEXICON.md)** - Complete lexicon system guide
- **[examples/lexicon-usage/](../examples/lexicon-usage/)** - Working code example
- **[synonyms.example.yaml](synonyms.example.yaml)** - Template with examples
