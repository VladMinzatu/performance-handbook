from transformers import AutoTokenizer, AutoModelForSequenceClassification
import torch


class TextModerator:
    def __init__(self):
        model_name = "unitary/toxic-bert"

        self.tokenizer = AutoTokenizer.from_pretrained(model_name)
        self.model = AutoModelForSequenceClassification.from_pretrained(model_name)

        self.model.eval()

        self.labels = ["toxic", "severe_toxic", "obscene", "threat", "insult", "identity_hate"]

    @torch.no_grad()
    def predict(self, text: str):
        inputs = self.tokenizer(
            text,
            return_tensors="pt",
            truncation=True,
            padding=True
        )

        outputs = self.model(**inputs)

        probs = torch.sigmoid(outputs.logits)[0]

        score, idx = torch.max(probs, dim=0)

        return {
            "label": self.labels[idx],
            "score": float(score)
        }


model = TextModerator()