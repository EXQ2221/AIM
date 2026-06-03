from __future__ import annotations

from pydantic import BaseModel


class HealthResponse(BaseModel):
    ok: bool


class ParseSentence(BaseModel):
    sentenceIndex: int
    text: str
    pageStart: int = 0
    pageEnd: int = 0
    charStart: int = 0
    charEnd: int = 0


class ParseChunk(BaseModel):
    index: int
    chunkType: str
    sectionTitle: str
    content: str
    pageStart: int = 0
    pageEnd: int = 0
    charStart: int = 0
    charEnd: int = 0
    sentences: list[ParseSentence] = []


class ParseResponse(BaseModel):
    title: str
    sourceType: str
    content: str
    fileType: str
    imageCount: int
    usedVisionDescription: bool
    chunks: list[ParseChunk]
