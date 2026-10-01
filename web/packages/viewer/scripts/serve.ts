// Static server for dist/; GET /doc serves the .tenn given on the command line. Usage: serve.ts [file.tenn]
import { createServer } from "node:http";
import { execFile } from "node:child_process";
import { readFile } from "node:fs/promises";
import { extname, join, normalize } from "node:path";

const dist = new URL("../dist/", import.meta.url).pathname;
const doc = process.argv[2];
const types: Record<string, string> = { ".html": "text/html", ".js": "text/javascript", ".ttf": "font/ttf" };

const server = createServer(async (req, res) => {
  const path = decodeURIComponent(new URL(req.url ?? "/", "http://x").pathname);
  try {
    if (path === "/doc" && doc !== undefined) {
      res.setHeader("content-type", "text/plain; charset=utf-8");
      res.end(await readFile(doc));
      return;
    }
    const file = normalize(join(dist, path === "/" ? "index.html" : path));
    if (!file.startsWith(dist)) throw new Error("outside dist");
    res.setHeader("content-type", types[extname(file)] ?? "application/octet-stream");
    res.end(await readFile(file));
  } catch {
    res.statusCode = 404;
    res.end("not found");
  }
});

server.listen(Number(process.env.PORT ?? 0), "127.0.0.1", () => {
  const { port } = server.address() as { port: number };
  const url = `http://127.0.0.1:${port}/${doc === undefined ? "" : "?file=/doc"}`;
  console.log(url);
  if (process.env.NO_OPEN) return;
  const opener = process.platform === "darwin" ? "open" : "xdg-open";
  execFile(opener, [url], () => {}); // no opener (headless Linux): the printed URL is enough
});
