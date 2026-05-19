from __future__ import annotations

from fastapi import FastAPI

from .schemas import HealthResponse
from .service import router

app = FastAPI(title="AIM Parser Service", version="1.0.0")
app.include_router(router, prefix="/v1")


@app.get("/healthz", response_model=HealthResponse)
def healthz() -> HealthResponse:
    return HealthResponse(ok=True)
