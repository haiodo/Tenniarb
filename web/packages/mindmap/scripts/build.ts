// dist/: mindmap.js (the package, ESM) and the demo page with yjs.js for it. `--analyze`: the biggest inputs of the bundle.
import { analyzeMetafile, build } from "esbuild";
import { copyFileSync, mkdirSync, readFileSync } from "node:fs";
import { gzipSync } from "node:zlib";
import { fileURLToPath } from "node:url";

const path = (p: string): string => fileURLToPath(new URL(`../${p}`, import.meta.url));
mkdirSync(path("dist"), { recursive: true });
const common = { bundle: true, format: "esm", minify: true, target: "es2022", legalComments: "none" } as const;
const { metafile } = await build({ ...common, entryPoints: [path("src/index.ts")], outfile: path("dist/mindmap.js"), metafile: true });
const buf = readFileSync(path("dist/mindmap.js"));
console.log(`mindmap.js: ${buf.length} B, gzip ${gzipSync(buf, { level: 9 }).length} B`);
if (process.argv.includes("--analyze")) console.log(await analyzeMetafile(metafile));
await build({ ...common, entryPoints: [fileURLToPath(import.meta.resolve("yjs"))], outfile: path("dist/yjs.js") });
copyFileSync(path("demo.html"), path("dist/demo.html"));
