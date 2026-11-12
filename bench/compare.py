#!/usr/bin/env python3
"""Compare a render to the reference picture of a visual scenario.

usage: bench/compare.py <reference.png> <render.png> [--out diff.png] [--json]

Writes JSON to stdout: mae (0..255), match (share of pixels within tolerance), blocks (the worst 12x8
block and its mean colour distance), sizes. The pictures must have the same size; a size difference is
reported as such, since a 1:1 task asks for the same frame.
"""
import argparse
import json
import os
import sys

from PIL import Image, ImageChops, ImageDraw


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("reference")
    ap.add_argument("render")
    ap.add_argument("--out")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--tolerance", type=int, default=12)
    args = ap.parse_args()

    ref = Image.open(args.reference).convert("RGB")
    got = Image.open(args.render).convert("RGB")
    result = {"reference": os.path.basename(args.reference), "render": os.path.basename(args.render),
              "reference_size": list(ref.size), "render_size": list(got.size)}
    if ref.size != got.size:
        result.update({"mae": None, "match": None, "pass": False, "reason": "size differs"})
        print(json.dumps(result))
        return 1

    diff = ImageChops.difference(ref, got).convert("L")
    hist = diff.histogram()
    total = ref.size[0] * ref.size[1]
    mae = sum(i * n for i, n in enumerate(hist)) / total
    match = sum(hist[: args.tolerance + 1]) / total

    # The frame in 12x8 blocks: where the two pictures differ in colour on average, which tells layout
    # apart from font antialiasing.
    cols, rows = 12, 8
    bw, bh = ref.size[0] / cols, ref.size[1] / rows
    worst = {"col": 0, "row": 0, "distance": 0.0}
    for row in range(rows):
        for col in range(cols):
            box = (int(col * bw), int(row * bh), int((col + 1) * bw), int((row + 1) * bh))
            a = ref.crop(box).resize((1, 1), Image.Resampling.BOX).getpixel((0, 0))
            b = got.crop(box).resize((1, 1), Image.Resampling.BOX).getpixel((0, 0))
            d = sum(abs(a[i] - b[i]) for i in range(3)) / 3
            if d > worst["distance"]:
                worst = {"col": col, "row": row, "distance": round(d, 1)}

    result.update({"mae": round(mae, 2), "match": round(match, 3), "tolerance": args.tolerance,
                   "worst_block": worst})

    if args.out:
        heat = Image.new("RGB", ref.size)
        px_ref = ref.load()
        px_diff = diff.load()
        heat_px = heat.load()
        for y in range(ref.size[1]):
            for x in range(ref.size[0]):
                d = px_diff[x, y]
                heat_px[x, y] = (min(255, 40 + d * 3), max(0, 220 - d * 3), max(0, 220 - d * 3))
        side = Image.new("RGB", (ref.size[0] * 3 + 20, ref.size[1]), (245, 245, 245))
        side.paste(ref, (0, 0))
        side.paste(got, (ref.size[0] + 10, 0))
        side.paste(heat, (ref.size[0] * 2 + 20, 0))
        side.save(args.out)
        result["diff_png"] = os.path.basename(args.out)

    print(json.dumps(result))
    return 0 if result["match"] is not None else 1


if __name__ == "__main__":
    sys.exit(main())
