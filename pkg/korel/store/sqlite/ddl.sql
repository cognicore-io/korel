-- Korel SQLite Schema
-- Knowledge Organization & Retrieval Engine

-- Documents table
CREATE TABLE IF NOT EXISTS docs (
  doc_id INTEGER PRIMARY KEY AUTOINCREMENT,
  url TEXT UNIQUE NOT NULL,
  title TEXT NOT NULL,
  outlet TEXT,
  published_at DATETIME NOT NULL,
  cats TEXT, -- JSON array of categories
  ents TEXT, -- JSON array of entities
  links_out INTEGER DEFAULT 0,
  text_len INTEGER,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_docs_published ON docs(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_docs_outlet ON docs(outlet);

-- Tokens table
CREATE TABLE IF NOT EXISTS tokens (
  token TEXT PRIMARY KEY,
  vars TEXT, -- JSON array of variants/synonyms
  cats TEXT, -- JSON array of categories
  df INTEGER DEFAULT 0, -- document frequency
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tokens_df ON tokens(df DESC);

-- Document-Token mapping (for inverted index)
CREATE TABLE IF NOT EXISTS doc_tokens (
  doc_id INTEGER NOT NULL,
  token TEXT NOT NULL,
  PRIMARY KEY (doc_id, token),
  FOREIGN KEY (doc_id) REFERENCES docs(doc_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_doc_tokens_token ON doc_tokens(token);

-- Token co-occurrence counts
CREATE TABLE IF NOT EXISTS pair_counts (
  t1 TEXT NOT NULL,
  t2 TEXT NOT NULL,
  n_xy INTEGER DEFAULT 0, -- co-occurrence count
  pmi REAL, -- pointwise mutual information
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (t1, t2),
  CHECK (t1 < t2) -- ensure canonical ordering (t1 < t2)
);

CREATE INDEX IF NOT EXISTS idx_pair_pmi ON pair_counts(pmi DESC);
CREATE INDEX IF NOT EXISTS idx_pair_t1 ON pair_counts(t1);
CREATE INDEX IF NOT EXISTS idx_pair_t2 ON pair_counts(t2);

-- Result cards
CREATE TABLE IF NOT EXISTS cards (
  card_id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  bullets TEXT, -- JSON array
  sources TEXT, -- JSON array of {url, time}
  score_json TEXT, -- JSON object with score breakdown
  period TEXT, -- e.g., "2025-W45"
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cards_period ON cards(period);

-- Stopword list (versioned)
CREATE TABLE IF NOT EXISTS stoplist (
  token TEXT PRIMARY KEY,
  reason TEXT, -- why it's a stopword (high DF, low PMI, etc.)
  added_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- System statistics (for self-tuning)
CREATE TABLE IF NOT EXISTS stats (
  key TEXT PRIMARY KEY,
  value TEXT, -- JSON value
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Insert initial stats
INSERT OR IGNORE INTO stats (key, value) VALUES ('N_docs', '0');
INSERT OR IGNORE INTO stats (key, value) VALUES ('N_tokens', '0');
INSERT OR IGNORE INTO stats (key, value) VALUES ('N_pairs', '0');
INSERT OR IGNORE INTO stats (key, value) VALUES ('last_index', '1970-01-01T00:00:00Z');
