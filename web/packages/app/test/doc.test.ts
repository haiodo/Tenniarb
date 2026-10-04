import assert from "node:assert/strict";
import { test } from "node:test";
import { readTenn } from "@tenniarb/core";
import { closeAction, docTitle, newDocumentText, parseFrames, pushRecent, recentLimit, untitledFile, untitledLabels, windowLabel } from "../src/doc.ts";

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

test("parseFrames keeps valid frames only", () => {
  const ok = { x: -10, y: 20.5, width: 944, height: 764 };
  assert.deepEqual(parseFrames(JSON.stringify({ "/a.tenn": ok, "/b.tenn": { x: 1 }, "/c.tenn": { ...ok, width: 0 }, "/d.tenn": null })), { "/a.tenn": ok });
  assert.deepEqual(parseFrames("not json"), {});
  assert.deepEqual(parseFrames("[1]"), {});
  assert.deepEqual(parseFrames("null"), {});
});

test("untitled files map to window labels and back", () => {
  assert.equal(untitledFile("untitled-5"), "untitled/untitled-5.tenn");
  assert.deepEqual(untitledLabels(["main.tenn", "untitled-5.tenn", ".DS_Store"]), ["main", "untitled-5"]);
});
