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
    page_start: int
    page_end: int
    char_start: int
    char_end: int
    sentences: list["SentenceSpan"]


@dataclass
class SentenceSpan:
    sentence_index: int
    text: str
    page_start: int
    page_end: int
    char_start: int
    char_end: int


def build_chunks(title: str, content: str, page_texts: list[str] | None = None) -> list[ChunkItem]:
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
            return _normalize_chunks(chunks, clean_content, page_texts or [])

    return _chunk_fallback(clean_title, clean_content, page_texts or [])


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
        ChunkItem(
            index=i,
            chunk_type=chunk_type,
            section_title=f"Chunk {i+1}",
            content=item,
            page_start=0,
            page_end=0,
            char_start=0,
            char_end=0,
            sentences=[],
        )
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


def _normalize_chunks(chunks: list[ChunkItem], full_content: str, page_texts: list[str]) -> list[ChunkItem]:
    normalized: list[ChunkItem] = []
    locators = _build_page_locators(full_content, page_texts)
    cursor = 0
    for i, item in enumerate(chunks):
        content = (item.content or "").strip()
        if content == "":
            continue
        chunk_type = (item.chunk_type or "PLAIN_TEXT").strip().upper()
        section_title = (item.section_title or "").strip() or f"Chunk {i+1}"
        char_start, char_end = _locate_chunk(full_content, content, cursor)
        if char_start >= 0:
            cursor = char_end
        else:
            char_start = 0
            char_end = 0
        page_start, page_end = _resolve_page_span(locators, char_start, char_end)
        sentences = _build_sentence_spans(content, char_start, locators)
        normalized.append(
            ChunkItem(
                index=len(normalized),
                chunk_type=chunk_type,
                section_title=section_title,
                content=content,
                page_start=page_start,
                page_end=page_end,
                char_start=char_start,
                char_end=char_end,
                sentences=sentences,
            )
        )
    return normalized


def _chunk_fallback(title: str, content: str, page_texts: list[str]) -> list[ChunkItem]:
    chunk_type = _detect_chunk_type(content)
    paragraphs = [part.strip() for part in content.split("\n\n") if part.strip()]
    if not paragraphs:
        paragraphs = [content.strip()]

    chunks: list[ChunkItem] = []
    current: list[str] = []
    current_len = 0
    locators = _build_page_locators(content, page_texts)
    cursor = 0
    for para in paragraphs:
        para_len = len(para)
        if current and current_len + 2 + para_len > CHUNKER_FALLBACK_CHARS:
            chunk_content = "\n\n".join(current).strip()
            char_start, char_end = _locate_chunk(content, chunk_content, cursor)
            if char_start >= 0:
                cursor = char_end
            else:
                char_start = 0
                char_end = 0
            page_start, page_end = _resolve_page_span(locators, char_start, char_end)
            sentences = _build_sentence_spans(chunk_content, char_start, locators)
            chunks.append(
                ChunkItem(
                    index=len(chunks),
                    chunk_type=chunk_type,
                    section_title=f"{title or 'Document'} - Chunk {len(chunks)+1}",
                    content=chunk_content,
                    page_start=page_start,
                    page_end=page_end,
                    char_start=char_start,
                    char_end=char_end,
                    sentences=sentences,
                )
            )
            current = []
            current_len = 0
        current.append(para)
        current_len += para_len + (2 if current_len > 0 else 0)
    if current:
        chunk_content = "\n\n".join(current).strip()
        char_start, char_end = _locate_chunk(content, chunk_content, cursor)
        if char_start >= 0:
            cursor = char_end
        else:
            char_start = 0
            char_end = 0
        page_start, page_end = _resolve_page_span(locators, char_start, char_end)
        sentences = _build_sentence_spans(chunk_content, char_start, locators)
        chunks.append(
            ChunkItem(
                index=len(chunks),
                chunk_type=chunk_type,
                section_title=f"{title or 'Document'} - Chunk {len(chunks)+1}",
                content=chunk_content,
                page_start=page_start,
                page_end=page_end,
                char_start=char_start,
                char_end=char_end,
                sentences=sentences,
            )
        )
    return chunks


@dataclass
class _PageLocator:
    page_no: int
    start: int
    end: int


def _build_page_locators(full_content: str, page_texts: list[str]) -> list[_PageLocator]:
    if not full_content or not page_texts:
        return []
    locators: list[_PageLocator] = []
    cursor = 0
    for page_no, raw_text in enumerate(page_texts, start=1):
        page_text = (raw_text or "").strip()
        if not page_text:
            continue
        idx = full_content.find(page_text, cursor)
        if idx < 0:
            idx = full_content.find(page_text)
        if idx < 0:
            continue
        end = idx + len(page_text)
        locators.append(_PageLocator(page_no=page_no, start=idx, end=end))
        cursor = end
    return locators


def _locate_chunk(full_content: str, chunk_content: str, cursor: int) -> tuple[int, int]:
    if not full_content or not chunk_content:
        return -1, -1
    idx = full_content.find(chunk_content, cursor)
    if idx < 0:
        idx = full_content.find(chunk_content)
    if idx < 0:
        return -1, -1
    return idx, idx + len(chunk_content)


def _resolve_page_span(locators: list[_PageLocator], char_start: int, char_end: int) -> tuple[int, int]:
    if char_start < 0 or char_end <= char_start or not locators:
        return 0, 0
    start_page = 0
    end_page = 0
    for item in locators:
        if start_page == 0 and char_start < item.end and char_end > item.start:
            start_page = item.page_no
        if char_end > item.start and char_start < item.end:
            end_page = item.page_no
    if start_page == 0 and end_page != 0:
        start_page = end_page
    if end_page == 0 and start_page != 0:
        end_page = start_page
    return start_page, end_page


@dataclass
class _SentenceOffset:
    text: str
    start: int
    end: int


def _build_sentence_spans(chunk_content: str, chunk_char_start: int, locators: list[_PageLocator]) -> list[SentenceSpan]:
    if chunk_char_start < 0:
        return []
    offsets = _split_sentences_with_offsets(chunk_content)
    if not offsets:
        return []
    result: list[SentenceSpan] = []
    for index, item in enumerate(offsets, start=1):
        abs_start = chunk_char_start + item.start
        abs_end = chunk_char_start + item.end
        page_start, page_end = _resolve_page_span(locators, abs_start, abs_end)
        result.append(
            SentenceSpan(
                sentence_index=index,
                text=item.text,
                page_start=page_start,
                page_end=page_end,
                char_start=abs_start,
                char_end=abs_end,
            )
        )
    return result


def _split_sentences_with_offsets(content: str) -> list[_SentenceOffset]:
    text = (content or "").strip()
    if text == "":
        return []
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    result: list[_SentenceOffset] = []
    current: list[str] = []
    start = -1
    cursor = 0

    def flush() -> None:
        nonlocal current, start
        value = "".join(current).strip()
        if value:
            result.append(_SentenceOffset(text=value, start=max(start, 0), end=cursor))
        current = []
        start = -1

    for ch in text:
        if start < 0:
            start = cursor
        current.append(ch)
        cursor += 1
        if ch in ("\n", "。", "！", "？", "；", ".", "!", "?", ";"):
            flush()
    flush()
    return result


def _detect_chunk_type(content: str) -> str:
    if _QUESTION_LINE_RE.search(content):
        return "QUESTION_BANK"
    if _SCRIPT_HINT_RE.search(content):
        return "SCRIPT"
    if _MARKDOWN_RE.search(content):
        return "MARKDOWN"
    return "PLAIN_TEXT"
