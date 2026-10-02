// Entry of the <script> bundles: exposes window.Tenniarb and runs on page load unless data-manual is set.
import { run } from "./run.ts";

export { render, run } from "./index.ts";

const self = typeof document !== "undefined" ? document.currentScript : null;
if (self !== null && !self.hasAttribute("data-manual")) {
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", () => void run());
  else void run();
}
