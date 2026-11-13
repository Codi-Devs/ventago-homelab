#!/usr/bin/env python3
"""Run PP-OCRv6 in the paddleocr container and print {text, confidence} JSON."""
import json
import sys


def main():
    if len(sys.argv) < 2:
        print(json.dumps({"error": "missing path"}), file=sys.stderr)
        sys.exit(2)
    path = sys.argv[1]
    try:
        from paddleocr import PaddleOCR
    except Exception as exc:
        print(json.dumps({"error": "paddleocr_import", "detail": str(exc)}), file=sys.stderr)
        sys.exit(1)

    ocr = PaddleOCR(
        device="cpu",
        text_detection_model_name="PP-OCRv6_medium_det",
        text_recognition_model_name="PP-OCRv6_medium_rec",
        use_doc_orientation_classify=True,
        use_doc_unwarping=False,
        use_textline_orientation=False,
        enable_mkldnn=False,
        cpu_threads=8,
    )
    results = ocr.predict(path)
    texts = []
    scores = []
    for result in results or []:
        data = result.json
        if callable(data):
            data = data()
        res = data.get("res") if isinstance(data, dict) else None
        if not isinstance(res, dict):
            continue
        texts.extend(str(t) for t in (res.get("rec_texts") or []) if t)
        for score in res.get("rec_scores") or []:
            try:
                scores.append(float(score))
            except (TypeError, ValueError):
                pass
    confidence = sum(scores) / len(scores) if scores else 0.0
    print(json.dumps({"text": "\n".join(texts), "confidence": confidence}, ensure_ascii=False))


if __name__ == "__main__":
    main()
