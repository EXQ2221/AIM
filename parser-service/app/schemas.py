from __future__ import annotations

from pydantic import BaseModel


class HealthResponse(BaseModel):
    ok: bool


class ParseChunk(BaseModel):
    index: int
    chunkType: str
    sectionTitle: str
    content: str


class ParseResponse(BaseModel):
    title: str
    sourceType: str
    content: str
    fileType: str
    imageCount: int
    usedVisionDescription: bool
    chunks: list[ParseChunk]
