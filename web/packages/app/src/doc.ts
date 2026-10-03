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
