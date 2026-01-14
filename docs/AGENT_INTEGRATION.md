# AI Agent & RAG Integration

Korel is built as an **explainable retrieval engine**.  This document outlines how to
use it inside larger agent/RAG systems.

## 1. Retrieval + LLM Summaries

1. Ingest your corpus (news, reports, etc.).
2. Query Korel via its Go API or the `chat-cli` binary.
3. Forward the retrieved cards (bullets + scores + sources) to any OpenAI-compatible
   LLM endpoint for natural-language summaries or downstream reasoning.

### CLI Example (`cmd/ai-chat`)

```bash
go run ./cmd/ai-chat \
  -config configs/ai-chat.yaml \
  -query "What are the latest solar policy changes in Europe?"
```

Config file (`configs/ai-chat.yaml`):

```yaml
db_path: ./data/korel.db
stoplist: configs/stoplist.yaml
dict: configs/tokens.dict
taxonomy: configs/taxonomies.yaml
rules: configs/rules/ai.rules
top_k: 3
llm:
  base_url: https://api.openai.com/v1/chat/completions
  model: gpt-4o-mini
  api_key: ${OPENAI_API_KEY}
```

The tool prints Korel's cards as JSON and then an LLM summary grounded entirely in
those facts.

#### Other Endpoints

- **Azure OpenAI** – set `llm.base_url` to the full deployment URL, e.g.
  `https://<resource>.openai.azure.com/openai/deployments/<deployment>/chat/completions?api-version=2024-02-15-preview`,
  and `llm.model` to the deployment name.  The `api_key` value is your Azure key.
- **Ollama** – use `http://localhost:11434/v1/chat/completions` with `model: qwen3:8b`
  (see `configs/ai-chat.ollama.yaml` for a full template).  Leave `api_key` empty.

## 2. Embedding Into RAG Services

1. **Retriever** – Use `korel.New(...)` in your backend and expose `Search` via gRPC/REST.
2. **Context Builder** – Serialize the top cards (bullets, sources, score breakdowns).
3. **Generator** – Call your preferred LLM with a prompt such as:

```
SYSTEM: You are a grounded analyst. Answer only using the provided facts.
USER: Question: {{query}}
Facts:
1. {{card1.Title}} – bullets and sources…
```

This keeps generations auditable and prevents hallucinations.

## 3. Agent Tools (MCP / LangChain / LangGraph)

Expose Korel as a tool:

- **Input**: natural language question (and optional filters).
- **Tool action**: call `Search`, relay cards back to the agent.
- **Tool description**: “Explainable corpus retrieval; returns verified sources.”

Because Korel outputs structured explanations, agents can reason about confidence,
fall back to alternative tools, or ask follow-up questions anchored in the same facts.

## 4. Automation Loop

When your corpus evolves:

1. Run autotuners (stopwords/taxonomy/rules/entities) to keep configs fresh.
2. Maintenance jobs clean affected docs (partial reindex) and export new rules.
3. Reload the configs before the next agent session.

With this loop, agents always see up-to-date, deterministic facts.
