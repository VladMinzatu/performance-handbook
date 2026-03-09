from fastapi import APIRouter, UploadFile
from app.schemas import TextRequest

router = APIRouter()

@router.post("/moderate/text")
async def moderate_text(req: TextRequest):
    # dummy placeholder
    return {
        "label": "safe",
        "score": 0.12
    }


@router.post("/moderate/image")
async def moderate_image(file: UploadFile):
    # dummy placeholder
    return {
        "label": "safe",
        "score": 0.05
    }