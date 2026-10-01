import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

export const dir = fileURLToPath(new URL("../../../fixtures/", import.meta.url));

// Throws at import time (failing the test file) when the fixtures are missing, so a moved directory cannot pass silently.
export function fixtureNames(): string[] {
  assert.ok(existsSync(dir), `fixtures directory missing: ${dir}`);
  const names = readdirSync(dir).filter((f) => f.endsWith(".tenn") && !f.endsWith(".saved.tenn"));
  assert.ok(names.length > 0, `no .tenn fixtures in ${dir}`);
  return names;
}

// Swift reads fixtures with String(contentsOf:), which drops a leading BOM; readFileSync keeps it.
export function readSource(name: string): string {
  return readFileSync(dir + name, "utf8").replace(/^﻿/, "");
}

// readFileSync throws on a missing file, so a fixture without its golden fails.
export function readGolden(name: string): string {
  return readFileSync(dir + name, "utf8");
}
