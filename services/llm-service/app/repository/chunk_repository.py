from __future__ import annotations

from typing import List, Optional
import uuid
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.database import get_session_factory
from app.models.models import KnowledgeChunk


class ChunkRepository:
    def __init__(self, session: Optional[AsyncSession] = None):
        self._external_session = session

    async def _get_session(self) -> AsyncSession:
        if self._external_session is not None:
            return self._external_session
        factory = get_session_factory()
        if factory is None:
            raise RuntimeError("Database not initialized: session factory is None")
        return factory()

    async def create(self, *, material_id: str, content: str, vector: List[float], metadata: dict | None = None) -> KnowledgeChunk:
        sess = await self._get_session()
        close_needed = sess is not self._external_session
        try:
            obj = KnowledgeChunk(
                material_id=material_id, 
                content=content, 
                chunk_id=f"{material_id}:{uuid.uuid4().hex[:8]}",
                user_id=metadata.get("user_id", "unknown") if metadata else "unknown",
                content_type=metadata.get("content_type", "text") if metadata else "text",
                char_count=len(content),
                extra_metadata=metadata or {}
            )
            obj.set_vector(vector)
            sess.add(obj)
            await sess.commit()
            await sess.refresh(obj)
            return obj
        finally:
            if close_needed:
                await sess.close()
