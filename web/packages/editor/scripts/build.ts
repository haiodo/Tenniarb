// dist/: demo page, ESM bundle, embed bundle for the interactive HTML export, Inter fonts and the docs/Example.tenn sample.
import { build } from "esbuild";
import { cpSync, mkdirSync } from "node:fs";
import "../../embed/scripts/build.ts"; // writes embed/dist, which workspace order builds after this package
import { fileURLToPath } from "node:url";

const dist = new URL("../dist/", import.meta.url);
mkdirSync(dist, { recursive: true });
await build({
  entryPoints: [fileURLToPath(new URL("../src/index.ts", import.meta.url))],
  bundle: true,
  format: "esm",
  target: "es2022",
  outfile: fileURLToPath(new URL("editor.js", dist)),
});
cpSync(new URL("../../embed/dist/tenniarb-embed.standalone.min.js", import.meta.url), new URL("tenniarb-embed.min.js", dist)); // "Export as interactive HTML", fetched on use
cpSync(new URL("../demo.html", import.meta.url), new URL("demo.html", dist));
cpSync(new URL("../../embed/fonts/", import.meta.url), new URL("fonts/", dist), { recursive: true });
cpSync(new URL("../../../../docs/Example.tenn", import.meta.url), new URL("Example.tenn", dist));
