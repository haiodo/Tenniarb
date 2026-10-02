import { render } from "./render.ts";
import type { EmbedHandle, EmbedOptions } from "./render.ts";
import { parseBool } from "./util.ts";

export const SELECTOR = "pre.tenn, div.tenn, script[type='text/x-tenn']";
const DONE = "data-tenn-done";

export interface RunOptions extends Omit<EmbedOptions, "element"> {
  /** Nodes to process instead of querying the document. */
  nodes?: Iterable<HTMLElement>;
}

/** Per-node data-element / data-interactive / data-evaluate over the run() defaults. */
export function optionsFromAttrs(get: (name: string) => string | null, defaults: EmbedOptions = {}): EmbedOptions {
  return {
    ...defaults,
    element: get("data-element") ?? undefined,
    interactive: parseBool(get("data-interactive"), defaults.interactive ?? true),
    evaluate: parseBool(get("data-evaluate"), defaults.evaluate ?? true),
  };
}

/** data-encoding="base64": UTF-8 text behind base64 (the Swift HTML export), else the text as written. */
export function decodeSource(text: string, encoding: string | null): string {
  if (encoding?.toLowerCase() !== "base64") return text;
  return new TextDecoder().decode(Uint8Array.from(atob(text.trim()), (c) => c.charCodeAt(0)));
}

/** pre.tenn is replaced by a div, script[type=text/x-tenn] gets a div after it, div.tenn is rendered in place. */
export function run(opts: RunOptions = {}): Promise<EmbedHandle[]> {
  const nodes = [...(opts.nodes ?? document.querySelectorAll<HTMLElement>(SELECTOR))].filter((n) => !n.hasAttribute(DONE));
  return Promise.all(
    nodes.map((node) => {
      node.setAttribute(DONE, "");
      let text = node.textContent ?? "";
      try {
        text = decodeSource(text, node.getAttribute("data-encoding"));
      } catch {
        // invalid base64: falls through to the parser, which reports it inline
      }
      let target = node;
      if (node.tagName !== "DIV") {
        target = document.createElement("div");
        target.setAttribute(DONE, "");
        target.className = node.className;
        target.style.cssText = node.style.cssText;
        if (node.tagName === "PRE") {
          target.id = node.id;
          node.replaceWith(target);
        } else if (node.parentElement === document.head) document.body.append(target); // a div in <head> is never shown
        else node.after(target);
      }
      return render(target, text, optionsFromAttrs((n) => node.getAttribute(n), opts));
    }),
  );
}
