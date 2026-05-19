from __future__ import annotations

from fastapi import APIRouter, File, Form, HTTPException, UploadFile

from .parser_core import ParsedDocument, parse_document
from .schemas import ParseResponse

router = APIRouter()


@router.post("/parse", response_model=ParseResponse)
async def parse_file(
    file: UploadFile = File(...),
    title: str | None = Form(default=None),
) -> ParseResponse:
    raw = await file.read()
    try:
        parsed: ParsedDocument = parse_document(
            filename=file.filename or "",
            content_type=file.content_type or "",
            data=raw,
            title_override=title,
        )
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"parse failed: {exc}") from exc

    return ParseResponse(
        title=parsed.title,
        sourceType=parsed.source_type,
        content=parsed.content,
        fileType=parsed.file_type,
        imageCount=parsed.image_count,
        usedVisionDescription=parsed.used_vision_description,
    )
