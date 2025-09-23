from __future__ import annotations

import grpc

from app.proto.llm import llm_pb2, llm_pb2_grpc
from app.services.llm_service import LLMService


class LLMServiceHandler(llm_pb2_grpc.LLMServiceServicer):
    def __init__(self) -> None:
        self.svc = LLMService()

    async def AskQuestion(self, request: llm_pb2.QuestionRequest, context: grpc.aio.ServicerContext) -> llm_pb2.QuestionResponse:
        result = await self.svc.ask_question(
            question=request.question,
            user_id=request.user_id,
            material_ids=list(request.material_ids),
            context=dict(request.context),
        )
        # Ensure protobuf map<string,string> compatibility
        def _str_map(d: dict | None) -> dict[str, str]:
            if not d:
                return {}
            return {str(k): str(v) for k, v in d.items()}
        return llm_pb2.QuestionResponse(
            answer=result["answer"],
            confidence=float(result["confidence"]),
            sources=[
                llm_pb2.SourceReference(
                    material_id=s["material_id"],
                    content_snippet=s["content_snippet"],
                    relevance_score=float(s["relevance_score"]),
                )
                for s in result["sources"]
            ],
            metadata=_str_map(result.get("metadata")),
        )

    async def AskQuestionStream(self, request: llm_pb2.QuestionRequest, context: grpc.aio.ServicerContext):
        """Server-streaming tokens using OpenAI-compatible streaming.
        Fallback to single-shot answer if streaming model is not configured.
        """
        # try streaming if available
        if getattr(self.svc, "_oa", None) and self.svc._oa.is_enabled():
            # prepare retrieval + memory context (token-aware)
            hits = await self.svc.semantic_search(request.question, user_id=request.user_id, top_k=3)
            messages, session_id, used_turns, used_tokens = await self.svc._build_messages(
                request.question, user_id=request.user_id, context=dict(request.context), hits=hits
            )

            final_parts: list[str] = []
            async for tok in self.svc._oa.achat_stream(messages):
                final_parts.append(tok)
                yield llm_pb2.TokenChunk(content=tok, is_final=False)
            final_answer = "".join(final_parts)
            # write memory best-effort
            if session_id:
                try:
                    await self.svc._memory.append(session_id, role="user", content=request.question)
                    await self.svc._memory.append(session_id, role="assistant", content=final_answer)
                except Exception:
                    pass
            # final marker with session + usage metadata for clients to capture
            yield llm_pb2.TokenChunk(
                content="",
                is_final=True,
                metadata={
                    "session_id": session_id or "",
                    "used_history_turns": str(used_turns),
                    "used_history_tokens": str(used_tokens),
                },
            )
            return

        # fallback: produce a single response as one chunk
        single = await self.AskQuestion(request, context)
        # In fallback, also include metadata with session if AskQuestion generated one
        sid = ""
        try:
            # AskQuestion mirrored metadata in map; but we cannot access it here.
            # Recompute session for consistency (no-op if client provided)
            sid, _ = self.svc._get_session_params(dict(request.context))
        except Exception:
            pass
        yield llm_pb2.TokenChunk(content=single.answer, is_final=True, metadata={"session_id": sid})

    async def SemanticSearch(self, request: llm_pb2.SearchRequest, context: grpc.aio.ServicerContext) -> llm_pb2.SearchResponse:
        print(f"[DEBUG] Received SemanticSearch request: query='{request.query}', user_id='{request.user_id}', top_k={request.top_k}, material_ids={list(request.material_ids)}")
        hits = await self.svc.semantic_search(
            query=request.query, 
            user_id=request.user_id, 
            top_k=request.top_k or 5,
            material_ids=list(request.material_ids) if request.material_ids else None
        )
        print(f"[DEBUG] SemanticSearch found {len(hits)} hits")
        def _str_map(d: dict | None) -> dict[str, str]:
            if not d:
                return {}
            return {str(k): str(v) for k, v in d.items()}
        return llm_pb2.SearchResponse(
            results=[
                llm_pb2.SearchResult(
                    material_id=h["material_id"],
                    content=h["content"],
                    similarity_score=float(h["similarity_score"]),
                    metadata=_str_map(h.get("metadata")),
                )
                for h in hits
            ]
        )

    async def GenerateEmbeddings(self, request: llm_pb2.EmbeddingRequest, context: grpc.aio.ServicerContext) -> llm_pb2.EmbeddingResponse:
        res = await self.svc.generate_embeddings(
            content=request.content, material_id=request.material_id, content_type=request.content_type
        )
        return llm_pb2.EmbeddingResponse(embedding=list(res["embedding"]), embedding_id=res["embedding_id"])

    async def UpsertChunks(self, request: llm_pb2.UpsertChunksRequest, context: grpc.aio.ServicerContext) -> llm_pb2.UpsertChunksResponse:
        # Batch vectorize and persist chunks for a given material
        items = []
        for ch in request.chunks:
            items.append({
                "content": ch.content,
                "timecode": ch.timecode,
                "page": ch.page,
                "metadata": dict(ch.metadata),
            })
        inserted = await self.svc.upsert_chunks(user_id=request.user_id, material_id=request.material_id, chunks=items)
        return llm_pb2.UpsertChunksResponse(inserted=int(inserted))
