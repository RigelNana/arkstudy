from __future__ import annotations

from datetime import datetime
from typing import List, Dict, Any

import os
from sqlalchemy import String, Integer, Text, JSON, DateTime, Float, Boolean, Index
from sqlalchemy.orm import Mapped, mapped_column
try:
    from pgvector.sqlalchemy import Vector as PGVector  # type: ignore
except Exception:  # fallback when pgvector not installed
    from sqlalchemy import Text as PGVector  # type: ignore

from app.core.database import Base


def _vector_to_text(vec: List[float]) -> str:
    # Simple, portable storage: JSON-encoded vector as text
    import json
    return json.dumps(vec)


def _text_to_vector(s: str) -> List[float]:
    import json
    try:
        arr = json.loads(s)
        if isinstance(arr, list):
            return [float(x) for x in arr]
    except Exception:
        pass
    return []


class KnowledgeChunk(Base):
    """Enhanced knowledge chunks table for intelligent retrieval"""
    __tablename__ = "knowledge_chunks"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    
    # 关联信息
    user_id: Mapped[str] = mapped_column(String(36), index=True)  # 用户私有化
    material_id: Mapped[str] = mapped_column(String(64), index=True)
    chunk_id: Mapped[str] = mapped_column(String(64), unique=True, index=True)  # 唯一标识
    
    # 内容信息
    content: Mapped[str] = mapped_column(Text)
    content_type: Mapped[str] = mapped_column(String(32), default="text")  # text, image, table, code
    language: Mapped[str] = mapped_column(String(16), default="zh", index=True)
    
    # 文档结构信息
    chunk_index: Mapped[int] = mapped_column(Integer, default=0)  # 在文档中的位置
    level: Mapped[int] = mapped_column(Integer, default=0)  # 层级（标题等级）
    parent_chunk_id: Mapped[str | None] = mapped_column(String(64), nullable=True, index=True)
    
    # 向量存储 - 支持多种嵌入模型
    _VEC_DIM = int(os.getenv("LLM_VECTOR_DIM", "128"))  # OpenAI ada-002 默认维度
    vector: Mapped[list[float] | None] = mapped_column(PGVector(dim=_VEC_DIM), nullable=True)
    vector_model: Mapped[str] = mapped_column(String(64), default="text-embedding-3-small")
    
    # 文本特征（用于混合检索）
    char_count: Mapped[int] = mapped_column(Integer, default=0)
    token_count: Mapped[int] = mapped_column(Integer, default=0)
    keywords: Mapped[List[str] | None] = mapped_column(JSON, nullable=True)  # 关键词列表
    
    # 质量评分
    quality_score: Mapped[float] = mapped_column(Float, default=0.0)  # 内容质量评分
    relevance_score: Mapped[float] = mapped_column(Float, default=0.0)  # 相关性评分
    
    # 元数据
    extra_metadata: Mapped[Dict[str, Any] | None] = mapped_column("metadata", JSON, nullable=True)
    
    # 索引和缓存
    indexed: Mapped[bool] = mapped_column(Boolean, default=False)  # 是否已建索引
    last_accessed: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    access_count: Mapped[int] = mapped_column(Integer, default=0)
    
    # 时间戳
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    # 添加复合索引以优化查询性能
    __table_args__ = (
        Index('ix_user_material', 'user_id', 'material_id'),
        Index('ix_material_chunk_index', 'material_id', 'chunk_index'),
        Index('ix_content_type_lang', 'content_type', 'language'),
        Index('ix_quality_relevance', 'quality_score', 'relevance_score'),
        Index('ix_last_accessed', 'last_accessed'),
    )

    def set_vector(self, vec: List[float]) -> None:
        """设置向量并更新相关字段"""
        try:
            self.vector = list(vec)
            self.indexed = True
            self.updated_at = datetime.utcnow()
        except Exception:
            self.vector = None
            self.indexed = False

    def get_vector(self) -> List[float]:
        """获取向量"""
        return list(self.vector) if self.vector else []

    def update_access(self) -> None:
        """更新访问统计"""
        self.last_accessed = datetime.utcnow()
        self.access_count += 1

    def to_dict(self) -> Dict[str, Any]:
        """转换为字典格式"""
        return {
            'id': self.id,
            'chunk_id': self.chunk_id,
            'user_id': self.user_id,
            'material_id': self.material_id,
            'content': self.content,
            'content_type': self.content_type,
            'language': self.language,
            'chunk_index': self.chunk_index,
            'level': self.level,
            'parent_chunk_id': self.parent_chunk_id,
            'vector_model': self.vector_model,
            'char_count': self.char_count,
            'token_count': self.token_count,
            'keywords': self.keywords,
            'quality_score': self.quality_score,
            'relevance_score': self.relevance_score,
            'metadata': self.extra_metadata,
            'indexed': self.indexed,
            'access_count': self.access_count,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
            'last_accessed': self.last_accessed.isoformat() if self.last_accessed else None,
        }


# 保持向后兼容
class MaterialChunk(Base):
    __tablename__ = "material_chunks"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    material_id: Mapped[str] = mapped_column(String(64), index=True)
    content: Mapped[str] = mapped_column(Text)
    # Store vector as JSON text for portability (can switch to pgvector later)
    vector_text: Mapped[str] = mapped_column(Text)
    # pgvector column (nullable for backward compatibility); dimension from env
    _VEC_DIM = int(os.getenv("LLM_VECTOR_DIM", "128"))
    vector: Mapped[list[float] | None] = mapped_column(PGVector(dim=_VEC_DIM), nullable=True)
    # Use 'extra' JSON field for arbitrary metadata
    extra: Mapped[Dict[str, Any] | None] = mapped_column("extra", JSON, default=None)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow)

    # Convenience helpers
    def set_vector(self, vec: List[float]) -> None:
        self.vector_text = _vector_to_text(vec)
        try:
            # also set pgvector column when available
            self.vector = list(vec)
        except Exception:
            self.vector = None

    def get_vector(self) -> List[float]:
        return _text_to_vector(self.vector_text)
