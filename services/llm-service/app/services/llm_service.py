from __future__ import annotations

from typing import Dict, List
import uuid

from app.core.vector_store import InMemoryVectorStore
from app.core.database import get_session_factory
from app.repository.chunk_repository import ChunkRepository
from app.services.openai_client import OpenAIClient
from app.services.memory import InMemoryMemoryStore, MemoryStore


class LLMService:
    def __init__(self) -> None:
        # MVP: in-memory vector store
        self.store = InMemoryVectorStore(dim=128)
        # optional persistence
        session_factory = get_session_factory()
        self._db_enabled = session_factory is not None
        print(f"[DEBUG] Database enabled: {self._db_enabled}, session_factory: {session_factory}")
        # optional OpenAI-compatible client
        self._oa = OpenAIClient()
        # session memory (pluggable)
        self._memory: MemoryStore = InMemoryMemoryStore()

    # ---- History selection helpers (token-budget first, turns as fallback) ----
    def _get_encoding_name(self) -> str:
        # Map common OpenAI models to tiktoken encoding. Default to cl100k_base.
        model = (self._oa.chat_model or "").lower() if getattr(self, "_oa", None) else ""
        # A small mapping; tiktoken handles many automatically via encoding_for_model
        if "gpt-4o" in model or "gpt-4" in model or "gpt-3.5" in model:
            return "cl100k_base"
        # fallback
        return "cl100k_base"

    def _estimate_tokens(self, text: str) -> int:
        # Prefer tiktoken when available; fallback to heuristic
        try:
            import tiktoken  # type: ignore
            enc = None
            # Try encoding_for_model if a model name exists
            if getattr(self, "_oa", None) and self._oa.chat_model:
                try:
                    enc = tiktoken.encoding_for_model(self._oa.chat_model)
                except Exception:
                    enc = None
            if enc is None:
                enc = tiktoken.get_encoding(self._get_encoding_name())
            return len(enc.encode(text))
        except Exception:
            # Lightweight estimate to avoid external deps; safe upper bound for CJK
            return max(1, int(len(text) * 0.5))

    def _messages_tokens(self, msgs: List[Dict]) -> int:
        return sum(self._estimate_tokens((m.get("role") or "") + (m.get("content") or "")) for m in msgs)

    def _trim_history_by_tokens(self, history: List[Dict], limit_tokens: int) -> List[Dict]:
        if limit_tokens <= 0 or not history:
            return []
        acc: list[Dict] = []
        used = 0
        for m in reversed(history):  # newest first
            t = self._estimate_tokens((m.get("role") or "") + (m.get("content") or ""))
            if used + t > limit_tokens:
                break
            acc.append(m)
            used += t
        return list(reversed(acc))

    def _pick_history(self, all_history: List[Dict], context: Dict[str, str]) -> tuple[List[Dict], int, int]:
        """
        Returns (trimmed_history, used_turns, used_tokens)
        Priority: max_history_tokens > max_history_turns > default(3 if has session_id)
        """
        max_tokens = 0
        try:
            if context and context.get("max_history_tokens"):
                max_tokens = int(context.get("max_history_tokens", "0") or "0")
        except Exception:
            max_tokens = 0
        if max_tokens > 0:
            trimmed = self._trim_history_by_tokens(all_history, max_tokens)
            used_tokens = self._messages_tokens(trimmed)
            used_turns = max(0, len(trimmed) // 2)
            return trimmed, used_turns, used_tokens

        # turns fallback
        had_session = bool(context.get("session_id")) if context else False
        max_turns = 3 if had_session else 0
        try:
            if context and "max_history_turns" in context:
                max_turns = int(context.get("max_history_turns", "0"))
        except Exception:
            max_turns = 0
        if max_turns < 0:
            max_turns = 0
        if max_turns > 20:
            max_turns = 20
        if max_turns == 0 or not all_history:
            return [], 0, 0
        k = max(0, len(all_history) - 2 * max_turns)
        trimmed2 = all_history[k:]
        return trimmed2, max_turns, self._messages_tokens(trimmed2)

    async def _build_messages(self, question: str, user_id: str, context: Dict[str, str], hits: List[Dict]) -> tuple[List[Dict], str, int, int]:
        # session id and nominal turns (for deciding how much to fetch from store)
        session_id, nominal_turns = self._get_session_params(context or {})
        history_msgs: list[Dict] = []
        if session_id:
            # fetch generously to allow token trimming to work; at least 20 turns
            fetch_turns = max(20, nominal_turns)
            prev = await self._memory.history(session_id, max_turns=fetch_turns)
            for m in prev:
                history_msgs.append({"role": m.role, "content": m.content})

        # apply token-based trimming (or turns fallback)
        trimmed_history, used_turns, used_tokens = self._pick_history(history_msgs, context or {})

        context_snippets = "\n\n".join(h["content"] for h in hits)
        messages = [
            {"role": "system", "content": "You are a helpful study assistant. Answer concisely using the provided context."},
            *trimmed_history,
            {"role": "user", "content": f"Question: {question}\n\nContext:\n{context_snippets}"},
        ]
        return messages, session_id, used_turns, used_tokens

    def _get_session_params(self, context: Dict[str, str]) -> tuple[str, int]:
        had_session = bool(context.get("session_id")) if context else False
        session_id = (context.get("session_id") or "") if context else ""
        # default: if caller provided a session_id but no explicit turns, use 3
        mht = 3 if had_session else 0
        if context and "max_history_turns" in context:
            try:
                mht = int(context.get("max_history_turns", "0"))
            except Exception:
                mht = 0
        # clamp
        if mht < 0:
            mht = 0
        if mht > 20:
            mht = 20
        # auto-generate a session if not provided, so caller can chain turns
        if not session_id:
            session_id = uuid.uuid4().hex
        return session_id, mht

    async def generate_embeddings(self, content: str, material_id: str, content_type: str) -> Dict:
        # prefer external embeddings when configured
        if self._oa.is_enabled():
            vec = await self._oa.aembedding(content)
            item = self.store.upsert_with_vector(
                material_id=material_id, content=content, vector=vec, metadata={"content_type": content_type}
            )
        else:
            item = self.store.upsert(material_id=material_id, content=content, metadata={"content_type": content_type})
        # persist if DB is configured
        if self._db_enabled:
            try:
                repo = ChunkRepository()
                # fire-and-forget friendly, but keep as await in future if we make this async
                import asyncio

                async def _save():
                    try:
                        print(f"[DEBUG] Saving to DB: material_id={material_id}, content_len={len(content)}")
                        result = await repo.create(
                            material_id=material_id,
                            content=content,
                            vector=item.vector,
                            metadata=item.metadata,
                        )
                        print(f"[DEBUG] Saved to DB successfully: id={result.id}")
                    except Exception as e:
                        print(f"[ERROR] Failed to save to DB: {e}")
                        import traceback
                        traceback.print_exc()

                # schedule without blocking caller
                asyncio.create_task(_save())
            except Exception as e:
                # do not break core flow on persistence failure in MVP
                print(f"[ERROR] Failed to create save task: {e}")
                pass
        # return embedding only for API compatibility
        return {
            "embedding": item.vector,
            "embedding_id": f"{material_id}:0",
        }

    async def upsert_chunks(self, *, user_id: str, material_id: str, chunks: list[dict]) -> int:
        """Batch upsert chunks: embed -> write in-memory store -> persist to DB if configured.

        chunks: list of { content: str, timecode?: str, page?: int, metadata?: dict }
        Returns number of inserted chunks.
        """
        if not chunks:
            return 0
        # prepare embeddings (external provider if configured)
        embedded: list[tuple[Dict, List[float]]] = []
        if self._oa.is_enabled():
            # sequential for MVP; consider concurrency with asyncio.gather later
            for ch in chunks:
                vec = await self._oa.aembedding(ch.get("content", ""))
                embedded.append((ch, vec))
        else:
            # local embedding
            for ch in chunks:
                # InMemoryVectorStore.upsert computes embeddings; reuse helper here by calling private embed path
                from app.core.embedding import embed_text
                vec = embed_text(ch.get("content", ""), dim=self.store.dim)
                embedded.append((ch, vec))

        # write into in-memory store and accumulate DB rows
        db_rows: list[dict] = []
        for ch, vec in embedded:
            meta = dict(ch.get("metadata") or {})
            # enrich metadata minimally
            if ch.get("timecode"):
                meta["timecode"] = str(ch.get("timecode"))
            if ch.get("page") not in (None, 0):
                meta["page"] = str(int(ch.get("page")))
            item = self.store.upsert_with_vector(material_id=material_id, content=ch.get("content", ""), vector=vec, metadata=meta)
            db_rows.append({
                "material_id": material_id,
                "content": item.content,
                "vector": item.vector,
                "metadata": item.metadata,
            })

        # persist to DB if enabled
        if self._db_enabled and db_rows:
            try:
                repo = ChunkRepository()
                await repo.create_many(db_rows)
            except Exception:
                # best-effort persistence
                pass
        return len(db_rows)

    async def semantic_search(self, query: str, user_id: str, top_k: int, material_ids: List[str] | None = None) -> List[Dict]:
        # Prefer DB-side vector search when pgvector is available (DB enabled)
        use_db = self._db_enabled
        qv: List[float] | None = None
        if self._oa.is_enabled():
            qv = await self._oa.aembedding(query)
        if use_db and qv is not None:
            # SQLAlchemy async query using pgvector distance operator; fallback safe if not available
            try:
                from sqlalchemy import text
                sess_factory = get_session_factory()
                assert sess_factory is not None
                async with sess_factory() as sess:
                    params = {"q": qv, "limit": int(top_k or 5)}
                    where = ""
                    if material_ids:
                        where = "WHERE material_id = ANY(:mids)"
                        params["mids"] = material_ids
                    sql = f"""
                        SELECT material_id, content, extra, 1 - (vector <#> :q) AS sim
                        FROM material_chunks
                        {where}
                        ORDER BY vector <#> :q
                        LIMIT :limit
                    """
                    res = await sess.execute(text(sql), params)
                    out = []
                    for row in res:
                        out.append({
                            "material_id": row[0],
                            "content": row[1],
                            "similarity_score": float(row[3]),
                            "metadata": row[2] or {},
                        })
                    return out
            except Exception:
                # fall back to in-memory search
                pass

        # In-memory fallback
        if qv is not None:
            results = self.store.search_by_vector(qv, top_k=top_k or 5)
        else:
            results = self.store.search(query, top_k=top_k or 5)
        allowed = set(material_ids or [])
        out = []
        for item, score in results:
            if allowed and item.material_id not in allowed:
                continue
            out.append({
                "material_id": item.material_id,
                "content": item.content,
                "similarity_score": float(score),
                "metadata": item.metadata,
            })
        return out

    async def ask_question(self, question: str, user_id: str, material_ids: List[str], context: Dict[str, str]) -> Dict:
        # use semantic search as grounding
        hits = await self.semantic_search(question, user_id=user_id, top_k=3, material_ids=material_ids or None)
        # build messages with token/turns aware history
        base_msgs, session_id, used_turns, used_tokens = await self._build_messages(
            question, user_id=user_id, context=context or {}, hits=hits
        )

        # synthesize an answer
        if self._oa.is_enabled():
            answer = await self._oa.achat(base_msgs)
        else:
            # trivial answer for MVP
            answer = f"Based on {len(hits)} context passages, here is a placeholder answer to: {question}"

        # persist memory (best-effort)
        try:
            await self._memory.append(session_id, role="user", content=question)
            await self._memory.append(session_id, role="assistant", content=answer)
        except Exception:
            pass

        # also vectorize the conversational turns for future retrieval (best-effort)
        try:
            conv_material_id = f"session:{session_id}"
            await self.generate_embeddings(question, material_id=conv_material_id, content_type="conversation")
            await self.generate_embeddings(answer, material_id=conv_material_id, content_type="conversation")
        except Exception:
            pass

        return {
            "answer": answer,
            "confidence": 0.5,
            "sources": [
                {
                    "material_id": h["material_id"],
                    "content_snippet": h["content"][:120],
                    "relevance_score": h["similarity_score"],
                }
                for h in hits
            ],
            "metadata": {
                "note": "mvp",
                "session_id": session_id,
                "used_history_turns": used_turns,
                "used_history_tokens": used_tokens,
            },
        }
