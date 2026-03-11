from fastapi import APIRouter, UploadFile, File
from PIL import Image
import io

from app.schemas import TextRequest
from .models.text_model import model as text_model
from .models.image_model import model as image_model

router = APIRouter()

@router.post("/moderate/text")
async def moderate_text(req: TextRequest):
    result = text_model.predict(req.text)

    return {
        "input": req.text,
        "label": result["label"],
        "score": result["score"]
    }


@router.post("/moderate/image")
async def moderate_image(file: UploadFile = File(...)):
    contents = await file.read()
    image = Image.open(io.BytesIO(contents)).convert("RGB")
    result = image_model.predict(image)
    return result
