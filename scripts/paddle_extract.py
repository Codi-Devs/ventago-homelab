#!/usr/bin/env python3
"""Run PaddleOCR on a local file and print {text, confidence} JSON."""
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

    ocr = PaddleOCR(use_angle_cls=True, lang="es", show_log=False)
    result = ocr.ocr(path, cls=True)
    lines = []
    scores = []
    for page in result or []:
        if not page:
            continue
        for row in page:
            if not row or len(row) < 2:
                continue
            rec = row[1]
            if rec is None:
                continue
            text = rec[0] if isinstance(rec, (list, tuple)) else str(rec)
            conf = rec[1] if isinstance(rec, (list, tuple)) and len(rec) > 1 else 0.0
            if text:
                lines.append(str(text))
                try:
                    scores.append(float(conf))
                except (TypeError, ValueError):
                    pass
    confidence = sum(scores) / len(scores) if scores else 0.0
    print(json.dumps({"text": "\n".join(lines), "confidence": confidence}, ensure_ascii=False))


if __name__ == "__main__":
    main()
