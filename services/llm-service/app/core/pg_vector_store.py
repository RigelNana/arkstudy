from typing import List, Dict, Tuple, Optional
from sqlalchemy import text, select, func
from ..models.models import KnowledgeChunk
from ..core.database import get_db_session
import logging

logger = logging.getLogger(__name__)


class PgVectorStore:
    """基于 pgvector 的向量存储"""
    
    def __init__(self):
        self.embedding_dim = 384  # 可配置

    async def add_chunk(
        self,
        content: str,
        embedding: List[float],
        metadata: Dict,
        chunk_type: str = "text",
        file_id: Optional[str] = None,
        user_id: Optional[str] = None
    ) -> str:
        """添加文档块到向量存储"""
        try:
            session = await get_db_session()
            async with session:
                chunk = KnowledgeChunk(
                    content=content,
                    embedding=embedding,
                    metadata=metadata,
                    chunk_type=chunk_type,
                    material_id=file_id,
                    user_id=user_id,
                    difficulty=metadata.get('difficulty', 'beginner'),
                    subject=metadata.get('subject', ''),
                    tags=metadata.get('tags', [])
                )
                
                session.add(chunk)
                await session.commit()
                await session.refresh(chunk)
                
                logger.info(f"Added chunk {chunk.id} to vector store")
                return chunk.id
                
        except Exception as e:
            logger.error(f"Error adding chunk to vector store: {e}")
            raise

    async def similarity_search(
        self,
        query_embedding: List[float],
        limit: int = 10,
        threshold: float = 0.7,
        filters: Optional[Dict] = None
    ) -> List[Tuple[KnowledgeChunk, float]]:
        """向量相似度搜索"""
        try:
            session = await get_db_session()
            async with session:
                # 构建基础查询
                query = select(
                    KnowledgeChunk,
                    func.cosine_distance(
                        KnowledgeChunk.embedding,
                        text(":query_embedding")
                    ).label("distance")
                ).order_by(
                    func.cosine_distance(
                        KnowledgeChunk.embedding,
                        text(":query_embedding")
                    )
                ).limit(limit)
                
                # 添加过滤条件
                if filters:
                    if 'user_id' in filters:
                        query = query.where(KnowledgeChunk.user_id == filters['user_id'])
                    if 'difficulty' in filters:
                        query = query.where(KnowledgeChunk.difficulty == filters['difficulty'])
                    if 'subject' in filters:
                        query = query.where(KnowledgeChunk.subject == filters['subject'])
                    if 'chunk_type' in filters:
                        query = query.where(KnowledgeChunk.chunk_type == filters['chunk_type'])
                
                # 执行查询
                result = await session.execute(
                    query,
                    {"query_embedding": str(query_embedding)}
                )
                
                # 处理结果
                chunks_with_scores = []
                for chunk, distance in result.fetchall():
                    similarity = 1.0 - distance  # 转换为相似度
                    if similarity >= threshold:
                        chunks_with_scores.append((chunk, similarity))
                
                logger.info(f"Found {len(chunks_with_scores)} similar chunks")
                return chunks_with_scores
                
        except Exception as e:
            logger.error(f"Error in similarity search: {e}")
            raise

    async def hybrid_search(
        self,
        query_embedding: List[float],
        query_text: str,
        limit: int = 10,
        vector_weight: float = 0.7,
        text_weight: float = 0.3,
        filters: Optional[Dict] = None
    ) -> List[Tuple[KnowledgeChunk, float]]:
        """混合搜索：向量 + 文本匹配"""
        try:
            session = await get_db_session()
            async with session:
                # 组合查询
                combined_query = select(
                    KnowledgeChunk,
                    (
                        vector_weight * (1.0 - func.cosine_distance(
                            KnowledgeChunk.embedding,
                            text(":query_embedding")
                        )) +
                        text_weight * func.ts_rank(
                            func.to_tsvector('chinese', KnowledgeChunk.content),
                            func.plainto_tsquery('chinese', text(":query_text"))
                        )
                    ).label("combined_score")
                ).order_by(
                    text("combined_score DESC")
                ).limit(limit)
                
                # 添加过滤条件
                if filters:
                    if 'user_id' in filters:
                        combined_query = combined_query.where(KnowledgeChunk.user_id == filters['user_id'])
                    if 'difficulty' in filters:
                        combined_query = combined_query.where(KnowledgeChunk.difficulty == filters['difficulty'])
                    if 'subject' in filters:
                        combined_query = combined_query.where(KnowledgeChunk.subject == filters['subject'])
                    if 'chunk_type' in filters:
                        combined_query = combined_query.where(KnowledgeChunk.chunk_type == filters['chunk_type'])
                
                result = await session.execute(
                    combined_query,
                    {
                        "query_embedding": str(query_embedding),
                        "query_text": query_text
                    }
                )
                
                chunks_with_scores = []
                for chunk, score in result.fetchall():
                    chunks_with_scores.append((chunk, float(score)))
                
                logger.info(f"Hybrid search found {len(chunks_with_scores)} chunks")
                return chunks_with_scores
                
        except Exception as e:
            logger.error(f"Error in hybrid search: {e}")
            # 降级到向量搜索
            return await self.similarity_search(query_embedding, limit, filters=filters)

    async def get_all_chunks_for_file(self, file_id: str) -> List[KnowledgeChunk]:
        """获取指定文件的所有chunk"""
        try:
            session = await get_db_session()
            async with session:
                result = await session.execute(
                    select(KnowledgeChunk).where(KnowledgeChunk.material_id == file_id)
                )
                chunks = result.scalars().all()
                return list(chunks)
        except Exception as e:
            logger.error(f"Failed to get all chunks for file {file_id}: {e}")
            return []

    async def delete_chunks_by_file(self, file_id: str) -> int:
        """删除文件的所有块"""
        try:
            session = await get_db_session()
            async with session:
                result = await session.execute(
                    text("DELETE FROM knowledge_chunks WHERE material_id = :file_id"),
                    {"file_id": file_id}
                )
                await session.commit()
                logger.info(f"Deleted {result.rowcount} chunks for file {file_id}")
                return result.rowcount
        except Exception as e:
            logger.error(f"Error deleting chunks by file: {e}")
            raise

    async def get_user_stats(self, user_id: str) -> Dict:
        """获取用户统计信息"""
        try:
            session = await get_db_session()
            async with session:
                # 总块数
                total_chunks = await session.execute(
                    select(func.count(KnowledgeChunk.id)).where(KnowledgeChunk.user_id == user_id)
                )
                
                # 按类型统计
                type_stats = await session.execute(
                    select(
                        KnowledgeChunk.chunk_type,
                        func.count(KnowledgeChunk.id)
                    ).where(
                        KnowledgeChunk.user_id == user_id
                    ).group_by(KnowledgeChunk.chunk_type)
                )
                
                # 按难度统计
                difficulty_stats = await session.execute(
                    select(
                        KnowledgeChunk.difficulty,
                        func.count(KnowledgeChunk.id)
                    ).where(
                        KnowledgeChunk.user_id == user_id
                    ).group_by(KnowledgeChunk.difficulty)
                )
                
                return {
                    "total_chunks": total_chunks.scalar(),
                    "by_type": dict(type_stats.fetchall()),
                    "by_difficulty": dict(difficulty_stats.fetchall())
                }
        except Exception as e:
            logger.error(f"Error getting user stats: {e}")
            raise


# 全局实例
pg_vector_store = PgVectorStore()