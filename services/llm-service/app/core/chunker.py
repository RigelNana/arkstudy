from __future__ import annotations

from typing import List


def _encode_len(text: str) -> int:
    try:
        import tiktoken  # type: ignore
        enc = tiktoken.get_encoding("cl100k_base")
        return len(enc.encode(text))
    except Exception:
        # rough fallback
        return max(1, int(len(text) * 0.5))


def chunk_text(text: str, max_tokens: int = 512, overlap_tokens: int = 50) -> List[str]:
    """Split text by token budget with small overlaps.

    MVP: naive sliding window over words; fallback by character when needed.
    """
    if not text:
        return []
    if max_tokens <= 0:
        return [text]

    # simple whitespace tokenization first
    words = text.split()
    chunks: List[str] = []
    cur: List[str] = []
    cur_toks = 0
    for w in words:
        wt = _encode_len(w)
        if cur and cur_toks + wt > max_tokens:
            chunks.append(" ".join(cur))
            # overlap
            if overlap_tokens > 0 and chunks[-1]:
                tail = chunks[-1].split()
                keep = []
                t = 0
                for x in reversed(tail):
                    tx = _encode_len(x)
                    if t + tx > overlap_tokens:
                        break
                    keep.append(x)
                    t += tx
                cur = list(reversed(keep))
                cur_toks = sum(_encode_len(x) for x in cur)
            else:
                cur = []
                cur_toks = 0
        cur.append(w)
        cur_toks += wt
    if cur:
        chunks.append(" ".join(cur))
    # fallback: if we ended with too-large chunking, just return original
    return chunks or [text]
