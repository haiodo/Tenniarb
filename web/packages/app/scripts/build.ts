// dist/: pages (document, settings, help), bundles and Inter fonts for the Tauri webview.
import { build } from "esbuild";
import { cpSync, mkdirSync, readFileSync } from "node:fs";
import "../../embed/scripts/build.ts"; // writes embed/dist, which workspace order builds after this package
import { fileURLToPath } from "node:url";

const dist = new URL("../dist/", import.meta.url);
mkdirSync(dist, { recursive: true });
await build({
  entryPoints: [fileURLToPath(new URL("../src/main.ts", import.meta.url)), fileURLToPath(new URL("../src/settings-page.ts", import.meta.url))],
  bundle: true,
  format: "esm",
  target: "es2022",
  outdir: fileURLToPath(dist),
});
cpSync(new URL("../../embed/dist/tenniarb-embed.standalone.min.js", import.meta.url), new URL("tenniarb-embed.min.js", dist)); // "Export as interactive HTML", fetched on use
cpSync(new URL("../index.html", import.meta.url), new URL("index.html", dist));
cpSync(new URL("../settings.html", import.meta.url), new URL("settings.html", dist));
cpSync(new URL("../../embed/fonts/", import.meta.url), new URL("fonts/", dist), { recursive: true });
// Help window: the Swift bundle readme.html (docs/) plus only the images it references.
const docs = new URL("../../../../docs/", import.meta.url);
const help = new URL("help/", dist);
mkdirSync(help, { recursive: true });
const readme = readFileSync(new URL("readme.html", docs), "utf8");
cpSync(new URL("readme.html", docs), new URL("index.html", help));
for (const [, img] of readme.matchAll(/src="\.\/([^"]+)"/g)) cpSync(new URL(`Images/${img}`, docs), new URL(img!, help));
