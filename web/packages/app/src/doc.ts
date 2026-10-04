export const recentLimit = 10;

/** Text of a freshly created document: one empty diagram, as ElementModelFactory in Swift. */
export const newDocumentText = 'element "Unnamed diagram" {\n}\n';

export function fileName(path: string): string {
  return path.slice(path.lastIndexOf("/") + 1);
}

/** Same rule as Swift `updateWindowTitle`: no ".tenn", "*" when modified. */
export function docTitle(path: string | null, dirty: boolean): string {
  const name = path === null ? "Untitled" : fileName(path).replace(/\.tenn$/, "");
  return dirty ? `${name}*` : name;
}

/** Most recent first, no duplicates, at most `recentLimit`. */
export function pushRecent(list: string[], path: string): string[] {
  return [path, ...list.filter((p) => p !== path)].slice(0, recentLimit);
}

/**
 * Close policy. With autosave a saved document is flushed silently (Swift: autosavesInPlace);
 * an untitled one has nowhere to flush to and asks.
 */
export function closeAction(path: string | null, dirty: boolean): "close" | "save" | "ask" {
  if (!dirty) return "close";
  return path === null ? "ask" : "save";
}

// Tauri window labels allow [a-zA-Z0-9-/:_]; a path hash makes "open the same file twice" find the existing window.
export function windowLabel(path: string): string {
  let h = 5381;
  for (let i = 0; i < path.length; i++) h = (Math.imul(h, 33) + path.charCodeAt(i)) >>> 0;
  return `doc-${h.toString(16)}`;
}

export interface Frame {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** windows.json: a frame per document path (Swift: `window.pos.<uri>` in prefs). Malformed entries are dropped. */
export function parseFrames(text: string): Record<string, Frame> {
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return {};
  }
  const out: Record<string, Frame> = {};
  if (typeof raw !== "object" || raw === null) return out;
  for (const [path, f] of Object.entries(raw)) {
    const { x, y, width, height } = (f ?? {}) as Partial<Frame>;
    if ([x, y, width, height].every((n) => typeof n === "number" && Number.isFinite(n)) && width! > 0 && height! > 0) out[path] = { x: x!, y: y!, width: width!, height: height! };
  }
  return out;
}

// Edited untitled windows live in AppData/untitled/<window label>.tenn: a file per window, so windows never race on one file.
export const untitledDir = "untitled";
export const untitledFile = (label: string): string => `${untitledDir}/${label}.tenn`;

export function untitledLabels(names: string[]): string[] {
  return names.filter((n) => n.endsWith(".tenn")).map((n) => n.slice(0, -".tenn".length));
}
