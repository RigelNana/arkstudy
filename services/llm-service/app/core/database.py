from __future__ import annotations

import os
from typing import Optional

from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)
from sqlalchemy.orm import DeclarativeBase


class Base(DeclarativeBase):
    pass


_engine: Optional[AsyncEngine] = None
_session_factory: Optional[async_sessionmaker[AsyncSession]] = None


def _build_db_url() -> Optional[str]:
    # Prefer fully specified URL
    url = os.getenv("LLM_DATABASE_URL") or os.getenv("DATABASE_URL")
    if url:
        return url

    # Compose from parts if available
    user = os.getenv("DB_USER")
    password = os.getenv("DB_PASSWORD")
    name = os.getenv("DB_NAME")
    host = os.getenv("DB_HOST")
    port = os.getenv("DB_PORT", "5432")
    if all([user, password, name, host]):
        return f"postgresql+asyncpg://{user}:{password}@{host}:{port}/{name}"
    return None


def is_db_enabled() -> bool:
    return _engine is not None and _session_factory is not None


async def init_db(echo: bool = False) -> None:
    global _engine, _session_factory
    url = _build_db_url()
    if not url:
        # DB not configured; service will run without persistence
        return

    try:
        _engine = create_async_engine(url, echo=echo, future=True)
        _session_factory = async_sessionmaker(_engine, expire_on_commit=False)

        # Import models to ensure metadata is populated
        from app.models import models  # noqa: F401

        async with _engine.begin() as conn:
            # Enable pgvector extension and ensure schema
            try:
                await conn.exec_driver_sql("CREATE EXTENSION IF NOT EXISTS vector")
            except Exception:
                # ignore if not Postgres/pgvector
                pass
            await conn.run_sync(Base.metadata.create_all)
            # Ensure pgvector column exists (for deployments that created table before adding the column)
            try:
                vec_dim = int(os.getenv("LLM_VECTOR_DIM", "128"))
            except Exception:
                vec_dim = 128
            try:
                # add column only if table/column conditions are satisfied
                await conn.exec_driver_sql(
                    f"""
                    DO $$
                    BEGIN
                        IF to_regclass('public.material_chunks') IS NOT NULL THEN
                            IF NOT EXISTS (
                                SELECT 1 FROM information_schema.columns
                                WHERE table_name = 'material_chunks' AND column_name = 'vector'
                            ) THEN
                                ALTER TABLE material_chunks ADD COLUMN vector vector({vec_dim});
                            END IF;
                        END IF;
                    END$$;
                    """
                )
            except Exception:
                # ignore if not supported or insufficient permissions
                pass
            try:
                # create ivfflat index only if vector column exists (use inner product opclass to match <#>)
                lists = 100
                try:
                    lists = int(os.getenv("LLM_IVFFLAT_LISTS", "100"))
                except Exception:
                    lists = 100
                await conn.exec_driver_sql(
                    f"""
                    DO $$
                    BEGIN
                        IF EXISTS (
                            SELECT 1 FROM information_schema.columns
                            WHERE table_name = 'material_chunks' AND column_name = 'vector'
                        ) THEN
                            IF NOT EXISTS (
                                SELECT 1 FROM pg_class c
                                JOIN pg_namespace n ON n.oid = c.relnamespace
                                WHERE c.relname = 'ix_material_chunks_vector'
                            ) THEN
                                CREATE INDEX ix_material_chunks_vector
                                ON material_chunks USING ivfflat (vector vector_ip_ops)
                                WITH (lists = {lists});
                            END IF;
                        END IF;
                    END$$;
                    """
                )
            except Exception:
                # ignore if not supported
                pass
        print("[database] initialized and tables ensured")
    except Exception as e:
        # Disable DB usage if initialization fails (e.g., DNS/connection refused)
        _engine = None
        _session_factory = None
        print(f"[database] init failed, running without persistence: {e}")


def get_session_factory() -> async_sessionmaker[AsyncSession] | None:
    return _session_factory


async def get_db_session() -> AsyncSession:
    """Get a database session.
    
    Raises:
        RuntimeError: If database is not enabled/configured.
    """
    if _session_factory is None:
        raise RuntimeError("Database is not enabled or configured")
    return _session_factory()
