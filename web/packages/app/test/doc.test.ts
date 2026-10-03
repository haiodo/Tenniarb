import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn } from "@tenniarb/core";
import { closeAction, docTitle, newDocumentText, pushRecent, recentLimit, windowLabel } from "../src/doc.ts";

test("docTitle", () => {
  assert.equal(docTitle(null, false), "Untitled");
  assert.equal(docTitle("/a/b/Plan.tenn", false), "Plan");
  assert.equal(docTitle("/a/b/Plan.tenn", true), "Plan*");
  assert.equal(docTitle("/a/notes.txt", true), "notes.txt*");
});

test("pushRecent dedupes, moves to front and limits", () => {
  assert.deepEqual(pushRecent(["/a", "/b"], "/b"), ["/b", "/a"]);
  const many = Array.from({ length: recentLimit }, (_, i) => `/f${i}`);
  const next = pushRecent(many, "/new");
  assert.equal(next.length, recentLimit);
  assert.equal(next[0], "/new");
  assert.ok(!next.includes(`/f${recentLimit - 1}`));
});

test("closeAction", () => {
  assert.equal(closeAction(null, false), "close");
  assert.equal(closeAction("/a.tenn", false), "close");
  assert.equal(closeAction("/a.tenn", true), "save");
  assert.equal(closeAction(null, true), "ask");
});

test("windowLabel is stable, distinct and label-safe", () => {
  assert.equal(windowLabel("/a/b.tenn"), windowLabel("/a/b.tenn"));
  assert.notEqual(windowLabel("/a/b.tenn"), windowLabel("/a/c.tenn"));
  assert.match(windowLabel("/a/b.tenn"), /^[a-zA-Z0-9-/:_]+$/);
});

test("newDocumentText parses", () => {
  assert.notEqual(readTenn(newDocumentText), null);
});
