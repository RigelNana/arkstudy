from __future__ import annotations

from dataclasses import dataclass
from typing import Dict, List
import asyncio


@dataclass
class MemoryMessage:
    role: str  # "user" | "assistant" | "system"
    content: str


class MemoryStore:
    """Abstract memory store interface for chat history.

    Methods are designed to be awaitable for future backends (e.g., Redis/DB).
    """

    async def append(self, session_id: str, role: str, content: str) -> None:  # pragma: no cover - interface
        raise NotImplementedError

    async def history(self, session_id: str, max_turns: int) -> List[MemoryMessage]:  # pragma: no cover - interface
        raise NotImplementedError


class InMemoryMemoryStore(MemoryStore):
    """A simple in-memory memory store.

    Stores recent messages per session. Not process-safe.
    """

    # soft cap to avoid unbounded growth per session
    MAX_MESSAGES_PER_SESSION = 200

    def __init__(self) -> None:
        # session_id -> list[MemoryMessage]
        self._data: Dict[str, List[MemoryMessage]] = {}
        # a simple lock to avoid concurrent mutation issues in async
        self._lock = asyncio.Lock()

    async def append(self, session_id: str, role: str, content: str) -> None:
        async with self._lock:
            arr = self._data.setdefault(session_id, [])
            arr.append(MemoryMessage(role=role, content=content))
            # clamp to soft cap
            if len(arr) > self.MAX_MESSAGES_PER_SESSION:
                overflow = len(arr) - self.MAX_MESSAGES_PER_SESSION
                del arr[:overflow]

    async def history(self, session_id: str, max_turns: int) -> List[MemoryMessage]:
        # Return last 2*max_turns messages (user+assistant pairs) if available
        async with self._lock:
            arr = list(self._data.get(session_id, []))
        if max_turns <= 0:
            return arr
        # keep last 2*max_turns messages
        n = max_turns * 2
        if len(arr) > n:
            arr = arr[-n:]
        return arr
