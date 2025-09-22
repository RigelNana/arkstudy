from __future__ import annotations

import os
from dataclasses import dataclass

try:
    # Load .env if present (development convenience)
    from dotenv import load_dotenv  # type: ignore

    load_dotenv()
except Exception:
    # dotenv is optional; environment may be provided by the runtime
    pass


@dataclass
class Settings:
    # gRPC listening address components
    grpc_host: str = os.getenv("LLM_GRPC_HOST", "[::]")
    # Backwards compatibility: support LLM_GRPC_PORT and legacy LLM_GRPC_ADDR (port only)
    grpc_port: int = int(os.getenv("LLM_GRPC_PORT", os.getenv("LLM_GRPC_ADDR", "50054")))

    # HTTP (uvicorn) host/port are typically passed via CLI; keep here for reference
    http_host: str = os.getenv("LLM_HTTP_HOST", "0.0.0.0")
    http_port: int = int(os.getenv("LLM_HTTP_PORT", "8000"))

    # OpenAI-compatible API settings (optional)
    # Example: OPENAI_BASE_URL="https://api.openai.com/v1" or a compatible endpoint (vLLM/Ollama/DeepSeek etc.)
    openai_base_url: str | None = os.getenv("OPENAI_BASE_URL") or os.getenv("LLM_OPENAI_BASE_URL")
    openai_api_key: str | None = os.getenv("OPENAI_API_KEY") or os.getenv("LLM_OPENAI_API_KEY")
    # Chat/Completion model (e.g., gpt-4o-mini, gpt-4o, deepseek-chat, qwen2.5, llama3.1, etc.)
    openai_model: str | None = os.getenv("OPENAI_MODEL") or os.getenv("LLM_OPENAI_MODEL")
    # Embedding model (e.g., text-embedding-3-small/large, bge-m3, jina-embeddings, etc.)
    openai_embedding_model: str | None = os.getenv("OPENAI_EMBEDDING_MODEL") or os.getenv("LLM_OPENAI_EMBEDDING_MODEL")

    # Vector dimension for pgvector (must match the embedding model). Default 128 for built-in embedder.
    vector_dim: int = int(os.getenv("LLM_VECTOR_DIM", "128"))

    # Database settings
    db_user: str = os.getenv("DB_USER", "postgres")
    db_password: str = os.getenv("DB_PASSWORD", "password")
    db_host: str = os.getenv("DB_HOST", "localhost")
    db_port: str = os.getenv("DB_PORT", "5432")
    db_name: str = os.getenv("DB_NAME", "arkdb")

    # Kafka settings
    kafka_bootstrap_servers: str = os.getenv("KAFKA_BROKERS", "localhost:9092")
    kafka_group_id: str = os.getenv("KAFKA_GROUP_ID", "llm-worker")
    kafka_topic_file_processing: str = os.getenv("KAFKA_TOPIC_FILE_PROCESSING", "file.processing")

    # MinIO settings
    minio_endpoint: str = os.getenv("MINIO_ENDPOINT", "localhost:9000")
    minio_access_key: str = os.getenv("MINIO_ACCESS_KEY", "minioadmin")
    minio_secret_key: str = os.getenv("MINIO_SECRET_KEY", "minioadmin")
    minio_bucket_name: str = os.getenv("MINIO_BUCKET_NAME", "arkstudy")

    @property
    def database_url(self) -> str:
        """构建数据库连接URL"""
        return f"postgresql+asyncpg://{self.db_user}:{self.db_password}@{self.db_host}:{self.db_port}/{self.db_name}"


def get_settings() -> Settings:
    return Settings()
