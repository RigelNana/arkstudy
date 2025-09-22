# llm-service (MVP skeleton)

This service provides a minimal FastAPI app and an async gRPC server for:
- AskQuestion
- SemanticSearch
- GenerateEmbeddings
- UpsertChunks (batch chunk ingest)

## Run locally (with uv)

```bash
cd services/llm-service
uv sync
# ensure generated proto imports resolve
PYTHONPATH=app uv run uvicorn app.main:app --host 0.0.0.0 --port 8000
```

The gRPC server starts automatically on port 50054 when the FastAPI app starts.

Health check:
```bash
curl http://localhost:8000/health
```

## Configuration (.env)

The service reads configuration from environment variables (optionally via .env in development):

- LLM_GRPC_HOST (default: [::])
- LLM_GRPC_PORT (default: 50054)
- LLM_HTTP_HOST (default: 0.0.0.0)
- LLM_HTTP_PORT (default: 8000)

Legacy compatibility: LLM_GRPC_ADDR is treated as the port number if LLM_GRPC_PORT is not set.

### Optional database (persistence)

If you set any of the following environment variables, the service will initialize a PostgreSQL connection and automatically create a table `material_chunks` to persist embeddings:

- LLM_DATABASE_URL or DATABASE_URL (e.g., postgresql+asyncpg://user:pass@host:5432/db)
	- or compose from parts: DB_USER, DB_PASSWORD, DB_NAME, DB_HOST, DB_PORT

When DB is enabled, generated embeddings are written asynchronously to the database.

#### pgvector mode

If your Postgres has the pgvector extension, llm-service will:
- Enable `CREATE EXTENSION IF NOT EXISTS vector` on startup
- Create table column `material_chunks.vector` (pgvector) in addition to legacy JSON `vector_text`
- Create `ivfflat` index: `CREATE INDEX ix_material_chunks_vector ON material_chunks USING ivfflat (vector)`
- Prefer DB-side vector search using `<#>` distance operator when external embeddings are enabled

Config:
- `LLM_DATABASE_URL` or parts (DB_USER, DB_PASSWORD, DB_HOST, DB_PORT, DB_NAME)
- `LLM_VECTOR_DIM` to match your embedding dim (default 128)

### Batch chunk ingest (gRPC UpsertChunks)

For higher throughput from parsers (ASR/OCR/PDF), use the gRPC method `UpsertChunks` to batch-embed and persist many chunks in one call.

Request shape:
- user_id: string (for observability)
- material_id: string (all chunks belong to the same material)
- chunks: list of items { content: string, timecode?: string, page?: int32, metadata?: map<string,string> }

Behavior:
- If OpenAI-compatible embedding is configured, uses that to compute vectors; otherwise uses the built-in embedding.
- Writes to process-local in-memory store immediately for retrieval.
- If DB is configured, persists all chunks in a single transaction.

Response:
- inserted: number of chunks accepted.

### Optional OpenAI-compatible API

To use external models (OpenAI or compatible providers like vLLM/Ollama/DeepSeek), set:

- OPENAI_BASE_URL (e.g., https://api.openai.com/v1 or your gateway)
- OPENAI_API_KEY
- OPENAI_MODEL (chat model, e.g., gpt-4o-mini)
- OPENAI_EMBEDDING_MODEL (embedding model, e.g., text-embedding-3-small)

When these are present, llm-service will:
- Use the external embedding model for GenerateEmbeddings and SemanticSearch
- Use the chat model in AskQuestion to synthesize answers grounded on retrieved snippets

### Session memory (multi-turn context)

MVP includes an in-memory chat history store to provide conversational context across turns.

- Pass session parameters via the `context` map in gRPC requests (the gateway already populates these from query/form fields):
	- `session_id`: a stable identifier for the conversation thread
	- `max_history_tokens`: preferred budget for past messages (token-based trimming)
	- `max_history_turns`: fallback number of past user+assistant turns (service clamps to [0, 20])

Behavior:
- On AskQuestion / AskQuestionStream, the service:
	1) Retrieves past history for `session_id` (generously), then trims by `max_history_tokens` if present; otherwise by `max_history_turns` (or default=3 if `session_id` is provided)
	2) Prepends the trimmed history to the LLM messages along with retrieved context passages
	3) Appends the new user question and the final assistant answer back into the session memory

Notes:
- The current backend is process-local memory only (not shared across replicas, resets on restart) and applies a soft cap of 200 messages per session.
- Response metadata includes `used_history_turns` and `used_history_tokens` for observability.
- For production, swap in a Redis/DB-backed implementation by implementing the same `MemoryStore` interface.
