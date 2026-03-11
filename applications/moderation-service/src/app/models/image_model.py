from transformers import AutoImageProcessor, AutoModelForImageClassification
from PIL import Image
import torch


class ImageModerator:
    def __init__(self):
        model_name = "Falconsai/nsfw_image_detection"

        self.processor = AutoImageProcessor.from_pretrained(model_name)
        self.model = AutoModelForImageClassification.from_pretrained(model_name)

        self.model.eval()

    @torch.no_grad()
    def predict(self, image: Image.Image):
        inputs = self.processor(images=image, return_tensors="pt")

        outputs = self.model(**inputs)
        probs = torch.softmax(outputs.logits, dim=1)[0]
        score, idx = torch.max(probs, dim=0)

        label = self.model.config.id2label[int(idx)]
        return {
            "label": label,
            "score": float(score)
        }


model = ImageModerator()