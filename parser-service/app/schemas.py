from __future__ import annotations

from pydantic import BaseModel


class HealthResponse(BaseModel):
    ok: bool


class ParseResponse(BaseModel):
    title: str
    sourceType: str
    content: str
    fileType: str
    imageCount: int
    usedVisionDescription: bool
