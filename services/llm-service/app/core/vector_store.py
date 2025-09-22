from __future__ import annotations

from dataclasses import dataclass
from typing import List, Dict, Any, Tuple
import numpy as np
import asyncio
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import text, select
from ..models.models import KnowledgeChunk
from ..core.database import get_db_session

from .embedding import embed_text


@dataclass
class VectorItem:
    material_id: str
    content: str
    vector: List[float]
    metadata: Dict[str, str]


class InMemoryVectorStore:
    """A tiny in-memory vector store for MVP validation."""

    def __init__(self, dim: int = 128):
        self.dim = dim
        self.items: list[VectorItem] = []

    def upsert(self, material_id: str, content: str, metadata: Dict[str, str] | None = None) -> VectorItem:
        metadata = metadata or {}
        vec = embed_text(content, dim=self.dim)
        item = VectorItem(material_id=material_id, content=content, vector=vec, metadata=metadata)
        # naive: append; no dedup for MVP
        self.items.append(item)
        return item

    def upsert_with_vector(self, material_id: str, content: str, vector: List[float], metadata: Dict[str, str] | None = None) -> VectorItem:
        metadata = metadata or {}
        item = VectorItem(material_id=material_id, content=content, vector=list(vector), metadata=metadata)
        self.items.append(item)
        return item

    def search(self, query: str, top_k: int = 5) -> List[Tuple[VectorItem, float]]:
        qv = embed_text(query, dim=self.dim)
        # cosine similarity
        def cosine(a: List[float], b: List[float]) -> float:
            return sum(x * y for x, y in zip(a, b))

        scored = [(it, cosine(qv, it.vector)) for it in self.items]
        scored.sort(key=lambda x: x[1], reverse=True)
        return scored[:top_k]

    def search_by_vector(self, query_vector: List[float], top_k: int = 5) -> List[Tuple[VectorItem, float]]:
        def cosine(a: List[float], b: List[float]) -> float:
            # dot product; assume vectors already normalized upstream when coming from external models
            return sum(x * y for x, y in zip(a, b))

        scored = [(it, cosine(query_vector, it.vector)) for it in self.items]
        scored.sort(key=lambda x: x[1], reverse=True)
        return scored[:top_k]
