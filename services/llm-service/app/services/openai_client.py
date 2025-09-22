from __future__ import annotations

from typing import List, Optional, AsyncIterator
import os
import httpx

from app.config import get_settings
import json


class OpenAIClient:
    """Minimal OpenAI-compatible client using HTTPX.

    Supports: chat completions and embeddings.
    Compatible with OpenAI and self-hosted (vLLM/Ollama) if they follow the same API surface.
    """

    def __init__(self) -> None:
        s = get_settings()
        self.base_url: Optional[str] = s.openai_base_url
        self.api_key: Optional[str] = s.openai_api_key
        self.chat_model: Optional[str] = s.openai_model
        self.embedding_model: Optional[str] = s.openai_embedding_model
        # Optional: allow env override directly
        self.base_url = os.getenv("OPENAI_BASE_URL", self.base_url)
        self.api_key = os.getenv("OPENAI_API_KEY", self.api_key)
        self.chat_model = os.getenv("OPENAI_MODEL", self.chat_model)
        self.embedding_model = os.getenv("OPENAI_EMBEDDING_MODEL", self.embedding_model)

    def is_enabled(self) -> bool:
        return bool(self.base_url and self.api_key)

    async def aembedding(self, text: str) -> List[float]:
        if not self.is_enabled():
            raise RuntimeError("OpenAI client not configured")
        model = self.embedding_model or "text-embedding-3-small"
        url = f"{self.base_url.rstrip('/')}/embeddings"
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }
        payload = {
            "model": model,
            "input": text,
        }
        async with httpx.AsyncClient(timeout=30.0) as client:
            r = await client.post(url, headers=headers, json=payload)
            r.raise_for_status()
            data = r.json()
            vec = data["data"][0]["embedding"]
            return [float(x) for x in vec]

    async def achat_stream(self, messages: list[dict]) -> AsyncIterator[str]:
        """Yield tokens from OpenAI-compatible streaming chat completions.

        Parses SSE lines like: "data: {json}" and stops on "data: [DONE]".
        Supports choices[0].delta.content (OpenAI) and a fallback for providers
        that send full message chunks.
        """
        if not self.is_enabled():
            raise RuntimeError("OpenAI client not configured")
        model = self.chat_model or "gpt-4o-mini"
        url = f"{self.base_url.rstrip('/')}/chat/completions"
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }
        payload = {
            "model": model,
            "messages": messages,
            "temperature": 0.3,
            "stream": True,
        }
        async with httpx.AsyncClient(timeout=None) as client:
            async with client.stream("POST", url, headers=headers, json=payload) as resp:
                resp.raise_for_status()
                async for line in resp.aiter_lines():
                    if not line:
                        continue
                    if line.startswith("data: "):
                        data_str = line[len("data: ") :].strip()
                        if data_str == "[DONE]":
                            break
                        try:
                            chunk = json.loads(data_str)
                        except Exception:
                            # best-effort: skip malformed lines
                            continue
                        # OpenAI format: choices[0].delta.content
                        try:
                            choices = chunk.get("choices") or []
                            if choices:
                                delta = choices[0].get("delta") or {}
                                token = delta.get("content")
                                if token:
                                    yield str(token)
                                else:
                                    # some providers send full messages per chunk
                                    msg = choices[0].get("message") or {}
                                    token2 = msg.get("content")
                                    if token2:
                                        yield str(token2)
                        except Exception:
                            # be tolerant to provider variations
                            pass

    async def achat(self, messages: list[dict]) -> str:
        # Aggregate streaming chunks into a single string for unary callers
        parts: list[str] = []
        async for tok in self.achat_stream(messages):
            parts.append(tok)
        return "".join(parts)
