// Builds dist/ (ESM, IIFE, standalone IIFE with Inter inlined, fonts). `--swift` also writes the standalone to Tenniarb/web/ for the app.
import { build } from "esbuild";
import { copyFileSync, mkdirSync, readFileSync, readdirSync } from "node:fs";
import { gzipSync } from "node:zlib";

const root = new URL("../", import.meta.url);
const path = (p: string): string => new URL(p, root).pathname;
const dist = path("dist/");
mkdirSync(dist, { recursive: true });

const fonts = readdirSync(path("fonts/")).filter((f) => f.endsWith(".woff2")).sort();
for (const f of fonts) copyFileSync(path(`fonts/${f}`), `${dist}${f}`);
const inline = Object.fromEntries(fonts.map((f) => [f, readFileSync(path(`fonts/${f}`)).toString("base64")]));

const common = { bundle: true, minify: true, target: "es2022", legalComments: "none" } as const;
const iife = { ...common, format: "iife", globalName: "Tenniarb", entryPoints: [path("src/iife.ts")], define: { "import.meta.url": '""' } } as const;
const ofl = "/* Tenniarb embed. Includes Inter, (c) The Inter Project Authors, SIL Open Font License 1.1. */";

const variants = [
  { ...common, format: "esm", entryPoints: [path("src/index.ts")], outfile: `${dist}tenniarb-embed.esm.min.js` },
  { ...iife, outfile: `${dist}tenniarb-embed.min.js`, define: iife.define },
  { ...iife, outfile: `${dist}tenniarb-embed.standalone.min.js`, banner: { js: ofl }, define: { ...iife.define, __INLINE_FONTS__: JSON.stringify(inline) } },
] as const;
for (const v of variants) {
  await build(v);
  const buf = readFileSync(v.outfile);
  console.log(`${v.outfile.split("/").pop()}: ${buf.length} B, gzip ${gzipSync(buf, { level: 9 }).length} B`);
}

if (process.argv.includes("--swift")) {
  const out = new URL("../../../../Tenniarb/web/", import.meta.url).pathname;
  mkdirSync(out, { recursive: true });
  copyFileSync(`${dist}tenniarb-embed.standalone.min.js`, `${out}tenniarb-embed.min.js`);
}
