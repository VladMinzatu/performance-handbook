from fastapi import FastAPI
from app.api import router

app = FastAPI(title="Moderation Service")

app.include_router(router)

@app.get("/health")
def health():
    return {"status": "ok"}