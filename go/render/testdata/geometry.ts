// Dumps item boxes and link endpoints of every web/fixtures element as laid out by the TS renderer, with a
// text measurer of 0.6 em per code point (what gfx.EstimateText does), no expression engine and no
// images, so the numbers do not depend on a font. geometry_test.go compares the Go renderer to this file.
//   cd web && node ../go/render/testdata/geometry.ts > ../go/render/testdata/geometry.json
import { readdirSync, readFileSync } from "node:fs";
import { readTenn } from "../../../web/packages/core/src/index.ts";
import { layoutScene, setMeasureContext, DrawableLine } from "../../../web/packages/render/src/index.ts";

const dir = new URL("../../../web/fixtures/", import.meta.url).pathname;
const estimate = {
  font: "",
  measureText(t: string) {
    const size = Number(/(\d+(?:\.\d+)?)px/.exec(this.font)![1]);
    return { width: [...t].length * (size * 0.6) };
  },
};
setMeasureContext(estimate);

const out: Record<string, unknown> = {};
const walk = (el: any, path: string): void => {
  if (el.items.length > 0) {
    const scene = layoutScene(el, null, {});
    out[path] = {
      bounds: scene.getBounds(),
      items: el.items.map((it: any, i: number) => {
        const d = scene.drawables.get(it);
        if (d === undefined) return { i, none: true };
        const r: any = { i, kind: it.kind, bounds: d.getBounds(), selector: d.getSelectorBounds() };
        if (d instanceof DrawableLine) {
          r.source = d.source;
          r.target = d.target;
          r.extra = d.extraPoints;
        }
        return r;
      }),
    };
  }
  for (const c of el.elements) walk(c, `${path}/${c.name}`);
};
for (const f of readdirSync(dir).filter((n) => n.endsWith(".tenn") && !n.endsWith(".saved.tenn")).sort()) {
  const model = readTenn(readFileSync(dir + f, "utf8"));
  if (model === null) continue;
  walk(model, f.replace(/\.tenn$/, ""));
}
console.log(JSON.stringify(out, null, 1));
