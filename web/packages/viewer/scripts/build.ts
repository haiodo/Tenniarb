import { build } from "esbuild";
import { cpSync, mkdirSync } from "node:fs";

const dist = new URL("../dist/", import.meta.url);
mkdirSync(dist, { recursive: true });
await build({
  entryPoints: [new URL("../src/main.ts", import.meta.url).pathname],
  bundle: true,
  format: "esm",
  target: "es2022",
  outfile: new URL("main.js", dist).pathname,
});
cpSync(new URL("../index.html", import.meta.url), new URL("index.html", dist));
cpSync(new URL("../../render/fonts/", import.meta.url), new URL("fonts/", dist), { recursive: true });
