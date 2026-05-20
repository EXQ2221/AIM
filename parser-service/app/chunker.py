from __future__ import annotations

import json
import os
import re
from dataclasses import dataclass
from typing import Any

import requests


ENABLE_LLM_CHUNKING = os.getenv("PARSER_ENABLE_LLM_CHUNKING", "true").strip().lower() in {
    "1",
    "true",
    "yes",
    "on",
}
CHUNKER_BASE_URL = os.getenv("CHUNKER_BASE_URL", "").strip()
CHUNKER_API_KEY = os.getenv("CHUNKER_API_KEY", "").strip()
CHUNKER_MODEL = os.getenv("CHUNKER_MODEL", "deepseek-v4-flash").strip()
CHUNKER_TIMEOUT_SECONDS = int(os.getenv("CHUNKER_TIMEOUT_SECONDS", "90"))
CHUNKER_MAX_CONTENT_CHARS = int(os.getenv("CHUNKER_MAX_CONTENT_CHARS", "120000"))
CHUNKER_FALLBACK_CHARS = int(os.getenv("CHUNKER_FALLBACK_CHARS", "1200"))

_QUESTION_LINE_RE = re.compile(r"(?m)^\s*\d+\s*[、.．]\s*")
_SCRIPT_HINT_RE = re.compile(r"(scene\s+\d+|act\s+\d+|dialogue|script|play|cast|narrator|角色|场景|人物|旁白|全剧终)")
_MARKDOWN_RE = re.compile(r"(?m)^\s{0,3}#{1,6}\s+")


@dataclass
class ChunkItem:
    index: int
    chunk_type: str
    section_title: str
    content: str


def build_chunks(title: str, content: str) -> list[ChunkItem]:
    clean_title = (title or "").strip()
    clean_content = (content or "").strip()
    if clean_content == "":
        return []

    if _llm_chunker_available():
        try:
            chunks = _chunk_by_llm(clean_content)
        except Exception:
            chunks = []
        if chunks:
            return _normalize_chunks(chunks)

    return _chunk_fallback(clean_title, clean_content)


def _llm_chunker_available() -> bool:
    return ENABLE_LLM_CHUNKING and CHUNKER_BASE_URL != "" and CHUNKER_API_KEY != "" and CHUNKER_MODEL != ""


def _chunk_by_llm(content: str) -> list[ChunkItem]:
    request_content = content
    if len(request_content) > CHUNKER_MAX_CONTENT_CHARS:
        request_content = request_content[:CHUNKER_MAX_CONTENT_CHARS]

    payload = {
        "model": CHUNKER_MODEL,
        "messages": [
            {
                "role": "system",
                "content": (
                    "You are a RAG chunking preprocessor. "
                    "Do not rewrite, summarize, or expand source text. "
                    "Return exactly one JSON string array where each element is one chunk. "
                    "Do not return markdown, explanation, or extra wrapper text."
                ),
            },
            {
                "role": "user",
                "content": (
                    "Split the input into semantically complete chunks.\n"
                    "- For question banks: keep each question complete with options/answer/explanation.\n"
                    "- For scripts: split by scene/act boundaries, never cut mid-sentence.\n"
                    "- For normal docs: split by paragraph/topic.\n"
                    "- Preferred chunk size: about 300 to 800 Chinese chars (or similar semantic size).\n"
                    "Return JSON string array only.\n\n"
                    f"{request_content}"
                ),
            },
        ],
        "temperature": 0.1,
    }

    url = CHUNKER_BASE_URL.rstrip("/") + "/chat/completions"
    headers = {
        "Authorization": f"Bearer {CHUNKER_API_KEY}",
        "Content-Type": "application/json",
    }
    response = requests.post(
        url,
        headers=headers,
        data=json.dumps(payload, ensure_ascii=False),
        timeout=CHUNKER_TIMEOUT_SECONDS,
    )
    response.raise_for_status()
    body = response.json()
    output = str(body["choices"][0]["message"]["content"]).strip()
    chunks = _parse_json_array(output)
    if not chunks:
        return []
    chunk_type = _detect_chunk_type(content)
    return [
        ChunkItem(index=i, chunk_type=chunk_type, section_title=f"Chunk {i+1}", content=item)
        for i, item in enumerate(chunks)
        if item.strip() != ""
    ]


def _parse_json_array(raw: str) -> list[str]:
    value: Any
    try:
        value = json.loads(raw)
    except json.JSONDecodeError:
        start = raw.find("[")
        end = raw.rfind("]")
        if start < 0 or end <= start:
            return []
        try:
            value = json.loads(raw[start : end + 1])
        except json.JSONDecodeError:
            return []
    if not isinstance(value, list):
        return []
    result: list[str] = []
    for item in value:
        if isinstance(item, str):
            text = item.strip()
            if text:
                result.append(text)
    return result


def _normalize_chunks(chunks: list[ChunkItem]) -> list[ChunkItem]:
    normalized: list[ChunkItem] = []
    for i, item in enumerate(chunks):
        content = (item.content or "").strip()
        if content == "":
            continue
        chunk_type = (item.chunk_type or "PLAIN_TEXT").strip().upper()
        section_title = (item.section_title or "").strip() or f"Chunk {i+1}"
        normalized.append(
            ChunkItem(
                index=len(normalized),
                chunk_type=chunk_type,
                section_title=section_title,
                content=content,
            )
        )
    return normalized


def _chunk_fallback(title: str, content: str) -> list[ChunkItem]:
    chunk_type = _detect_chunk_type(content)
    paragraphs = [part.strip() for part in content.split("\n\n") if part.strip()]
    if not paragraphs:
        paragraphs = [content.strip()]

    chunks: list[ChunkItem] = []
    current: list[str] = []
    current_len = 0
    for para in paragraphs:
        para_len = len(para)
        if current and current_len + 2 + para_len > CHUNKER_FALLBACK_CHARS:
            chunks.append(
                ChunkItem(
                    index=len(chunks),
                    chunk_type=chunk_type,
                    section_title=f"{title or 'Document'} - Chunk {len(chunks)+1}",
                    content="\n\n".join(current).strip(),
                )
            )
            current = []
            current_len = 0
        current.append(para)
        current_len += para_len + (2 if current_len > 0 else 0)
    if current:
        chunks.append(
            ChunkItem(
                index=len(chunks),
                chunk_type=chunk_type,
                section_title=f"{title or 'Document'} - Chunk {len(chunks)+1}",
                content="\n\n".join(current).strip(),
            )
        )
    return chunks


def _detect_chunk_type(content: str) -> str:
    if _QUESTION_LINE_RE.search(content):
        return "QUESTION_BANK"
    if _SCRIPT_HINT_RE.search(content):
        return "SCRIPT"
    if _MARKDOWN_RE.search(content):
        return "MARKDOWN"
    return "PLAIN_TEXT"
