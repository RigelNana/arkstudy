from __future__ import annotations

import hashlib
import math
from typing import List


def _tokenize(text: str) -> list[str]:
    return [t for t in text.lower().split() if t]


def embed_text(text: str, dim: int = 128) -> List[float]:
    """
    A tiny, dependency-free text embedding: hashed bag-of-words into fixed dims, L2-normalized.
    Not semantically strong, but good enough for MVP wiring and tests.
    """
    vec = [0.0] * dim
    for tok in _tokenize(text):
        h = int(hashlib.sha256(tok.encode("utf-8")).hexdigest(), 16)
        idx = h % dim
        vec[idx] += 1.0

    # L2 normalize
    norm = math.sqrt(sum(v * v for v in vec)) or 1.0
    return [v / norm for v in vec]
