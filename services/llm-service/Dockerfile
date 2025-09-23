

FROM python:3.13-slim

# Install uv for fast package management
RUN pip install --no-cache-dir uv

WORKDIR /app

# Copy project files
COPY pyproject.toml uv.lock requirements.txt ./
RUN uv venv
RUN . .venv/bin/activate && uv pip install -r requirements.txt --no-cache

COPY app ./app

# Expose ports: 8000 for HTTP, 50054 for gRPC (internal listener)
EXPOSE 8000 50054

# Default envs (can be overridden)
ENV LLM_HTTP_HOST=0.0.0.0 \
    LLM_HTTP_PORT=8000 \
    LLM_GRPC_HOST=[::] \
    LLM_GRPC_PORT=50054

# Start FastAPI app (which starts the gRPC server on startup)
CMD ["uv", "run", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
