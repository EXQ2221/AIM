from __future__ import annotations

from fastapi import APIRouter, File, Form, HTTPException, UploadFile

from .chunker import build_chunks
from .parser_core import ParsedDocument, parse_document
from .schemas import ParseChunk, ParseResponse, ParseSentence

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

    chunks = build_chunks(parsed.title, parsed.content, parsed.page_texts)

    return ParseResponse(
        title=parsed.title,
        sourceType=parsed.source_type,
        content=parsed.content,
        fileType=parsed.file_type,
        imageCount=parsed.image_count,
        usedVisionDescription=parsed.used_vision_description,
        chunks=[
            ParseChunk(
                index=item.index,
                chunkType=item.chunk_type,
                sectionTitle=item.section_title,
                content=item.content,
                pageStart=item.page_start,
                pageEnd=item.page_end,
                charStart=item.char_start,
                charEnd=item.char_end,
                sentences=[
                    ParseSentence(
                        sentenceIndex=sentence.sentence_index,
                        text=sentence.text,
                        pageStart=sentence.page_start,
                        pageEnd=sentence.page_end,
                        charStart=sentence.char_start,
                        charEnd=sentence.char_end,
                    )
                    for sentence in item.sentences
                ],
            )
            for item in chunks
        ],
    )
