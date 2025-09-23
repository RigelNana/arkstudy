#!/bin/bash

# Start the gRPC server in the background
uv run python -c 'import asyncio; from app.main import serve_grpc; asyncio.run(serve_grpc())' &

# Start the FastAPI server in the foreground
uv run uvicorn app.main:app --host 0.0.0.0 --port 8000