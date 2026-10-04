// Operation box (Swift OperationController): one input in a popup under the selection; Enter applies, Esc closes.
import type { Rect } from "@tenniarb/render";

/**
 * 300x50 popup below `anchor` (client coordinates), kept inside the window. `apply` returns false for invalid input:
 * the field turns red and stays open, as Swift. A press outside, resize and blur close it. Returns the closer.
 */
export function showOperation(anchor: Rect, apply: (text: string) => boolean, onClose: () => void): () => void {
  const ac = new AbortController();
  const close = (): void => {
    if (ac.signal.aborted) return;
    ac.abort();
    root.remove();
    onClose();
  };

  const root = document.createElement("div");
  root.className = "tn-search tn-op";
  const input = document.createElement("input");
  input.placeholder = "Operation...";
  input.spellcheck = false;
  root.append(input);
  input.oninput = () => input.classList.remove("bad");
  // Keys stay here: the canvas would take Backspace, x and the arrows.
  input.onkeydown = (ev) => {
    ev.stopPropagation();
    if (ev.key === "Escape") close();
    else if (ev.key !== "Enter") return;
    else if (apply(input.value)) close();
    else input.classList.add("bad");
    ev.preventDefault();
  };

  document.body.append(root);
  const [w, h] = [root.offsetWidth, root.offsetHeight];
  root.style.left = `${Math.max(0, Math.min(anchor.x + (anchor.width - w) / 2, innerWidth - w))}px`;
  root.style.top = `${Math.max(0, Math.min(anchor.y + anchor.height + 6, innerHeight - h))}px`;
  input.focus();
  const opt = { signal: ac.signal };
  document.addEventListener("pointerdown", (ev) => !root.contains(ev.target as Node) && close(), { ...opt, capture: true });
  addEventListener("resize", close, opt);
  addEventListener("blur", close, opt);
  return close;
}
