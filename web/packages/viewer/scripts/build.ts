import { build } from "esbuild";
import { cpSync, mkdirSync } from "node:fs";
import { fileURLToPath } from "node:url";

const dist = new URL("../dist/", import.meta.url);
mkdirSync(dist, { recursive: true });
await build({
  entryPoints: [fileURLToPath(new URL("../src/main.ts", import.meta.url))],
  bundle: true,
  format: "esm",
  target: "es2022",
  outfile: fileURLToPath(new URL("main.js", dist)),
});
cpSync(new URL("../index.html", import.meta.url), new URL("index.html", dist));
cpSync(new URL("../../embed/fonts/", import.meta.url), new URL("fonts/", dist), { recursive: true });
