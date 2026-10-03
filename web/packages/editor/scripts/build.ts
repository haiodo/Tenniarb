// dist/: demo page, ESM bundle, Inter fonts and the docs/Example.tenn sample.
import { build } from "esbuild";
import { cpSync, mkdirSync } from "node:fs";

const dist = new URL("../dist/", import.meta.url);
mkdirSync(dist, { recursive: true });
await build({
  entryPoints: [new URL("../src/index.ts", import.meta.url).pathname],
  bundle: true,
  format: "esm",
  target: "es2022",
  outfile: new URL("editor.js", dist).pathname,
});
cpSync(new URL("../demo.html", import.meta.url), new URL("demo.html", dist));
cpSync(new URL("../../embed/fonts/", import.meta.url), new URL("fonts/", dist), { recursive: true });
cpSync(new URL("../../../../docs/Example.tenn", import.meta.url), new URL("Example.tenn", dist));
