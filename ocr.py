#!/usr/bin/env python3
"""Run PaddleOCR on an image and output JSON to stdout."""

import json
import sys
import os

# Suppress paddle/paddleocr log noise
os.environ["GLOG_minloglevel"] = "2"
os.environ["PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK"] = "True"

from paddleocr import PaddleOCR


def main():
    if len(sys.argv) != 2:
        print(json.dumps({"error": "usage: ocr.py <image_path>"}), file=sys.stderr)
        sys.exit(1)

    image_path = sys.argv[1]

    ocr = PaddleOCR(
        use_doc_orientation_classify=False,
        use_doc_unwarping=False,
        use_textline_orientation=False,
    )

    result = ocr.predict(image_path)

    texts = []
    scores = []

    # OCRResult is a dict subclass — access via keys, not attributes
    for item in result:
        if isinstance(item, dict):
            texts.extend(item.get("rec_texts", []))
            scores.extend([float(s) for s in item.get("rec_scores", [])])

    print(json.dumps({"texts": texts, "scores": scores}))


if __name__ == "__main__":
    main()