from __future__ import annotations

from fastapi import FastAPI, Body
import grpc
# TODO: Re-enable when prometheus issue is resolved
# from prometheus_fastapi_instrumentator import Instrumentator

from app.proto.llm import llm_pb2_grpc
from app.grpc_handlers.llm_handler import LLMServiceHandler
from app.config import get_settings
from app.core.database import init_db
from app.core.chunker import chunk_text
from app.services.llm_service import LLMService
from app.services.kafka_file_processor import kafka_file_processor
import asyncio

app = FastAPI(title="LLM Service")

# TODO: Re-enable when prometheus issue is resolved
# 启用 Prometheus 指标
# instrumentator = Instrumentator()
# instrumentator.instrument(app).expose(app)

_svc = None


@app.on_event("startup")
async def on_startup() -> None:
    global _svc
    # init optional database (creates tables if URL is provided)
    await init_db(echo=False)
    
    # 数据库初始化后，创建 LLMService 实例
    _svc = LLMService()
    
    # 创建并启动 gRPC 服务器，放入 app.state 以便优雅关闭
    server = grpc.aio.server()
    llm_pb2_grpc.add_LLMServiceServicer_to_server(LLMServiceHandler(), server)
    settings = get_settings()
    listen_addr = f"{settings.grpc_host}:{settings.grpc_port}"
    server.add_insecure_port(listen_addr)
    print(f"Starting gRPC server on {listen_addr}")
    await server.start()
    app.state.grpc_server = server
    
    # 启动 Kafka 文件处理器
    asyncio.create_task(kafka_file_processor.start())
    print("Kafka file processor started")


@app.on_event("shutdown")
async def on_shutdown() -> None:
    # 停止 Kafka 文件处理器
    await kafka_file_processor.stop()
    
    # 优雅停止 gRPC 服务器，避免 event loop is closed 警告
    server: grpc.aio.Server | None = getattr(app.state, "grpc_server", None)
    if server is not None:
        await server.stop(grace=None)


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.post("/ingest/text")
async def ingest_text(
    user_id: str = Body("", embed=True),
    material_id: str = Body(..., embed=True),
    content: str = Body(..., embed=True),
    max_chunk_tokens: int = Body(512, embed=True),
):
    """Ingest raw text: chunk -> embed -> store (in-memory + DB if enabled).

    This is an MVP endpoint to support vectorization before queries.
    """
    if _svc is None:
        raise RuntimeError("LLM Service not initialized")
    
    chunks = chunk_text(content, max_tokens=max_chunk_tokens)
    inserted = 0
    for ch in chunks:
        await _svc.generate_embeddings(ch, material_id=material_id, content_type="text")
        inserted += 1
    return {"success": True, "inserted": inserted, "material_id": material_id, "user_id": user_id}


@app.post("/process/file")
async def process_file(
    file_id: str = Body(..., embed=True),
    file_path: str = Body(..., embed=True),
    user_id: str = Body(..., embed=True),
    file_type: str = Body("unknown", embed=True),
):
    """处理文件并入库向量"""
    try:
        chunks = await kafka_file_processor.process_file_message({
            "file_id": file_id,
            "file_path": file_path,
            "user_id": user_id,
            "file_type": file_type
        })
        return {
            "success": True,
            "file_id": file_id,
            "chunks_created": len(chunks) if chunks else 0
        }
    except Exception as e:
        return {
            "success": False,
            "error": str(e),
            "file_id": file_id
        }
