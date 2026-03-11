from fastapi import APIRouter, UploadFile
from app.schemas import TextRequest
from .models.text_model import model

router = APIRouter()

@router.post("/moderate/text")
async def moderate_text(req: TextRequest):
    result = model.predict(req.text)

    return {
        "input": req.text,
        "label": result["label"],
        "score": result["score"]
    }


@router.post("/moderate/image")
async def moderate_image(file: UploadFile):
    # dummy placeholder
    return {
        "label": "safe",
        "score": 0.05
    }