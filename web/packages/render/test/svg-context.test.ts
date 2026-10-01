import assert from 'node:assert/strict';
import { test } from 'node:test';
import { SvgContext } from '../src/svg-context.ts';

const mk = () => new SvgContext({ width: 100, height: 100, measureText: (_f, t) => ({ width: t.length * 6 }) });
const count = (s: string, re: RegExp) => (s.match(re) ?? []).length;

test('path survives save/restore', () => {
  const c = mk();
  c.save();
  c.beginPath();
  c.rect(1, 2, 3, 4);
  c.restore();
  c.stroke();
  c.fill();
  const s = c.toString();
  assert.equal(count(s, /<path /g), 2);
  assert.match(s, /d="M1 2L4 2L4 6L1 6ZM1 2"/);
});

test('beginPath inside save is not undone by restore', () => {
  const c = mk();
  c.rect(0, 0, 1, 1);
  c.save();
  c.beginPath();
  c.rect(5, 5, 1, 1);
  c.restore();
  c.fill();
  assert.doesNotMatch(c.toString(), /M0 0/);
});

test('transform is applied at segment time', () => {
  const c = mk();
  c.translate(10, 0);
  c.beginPath();
  c.moveTo(0, 0);
  c.translate(100, 0);
  c.lineTo(0, 0);
  c.resetTransform();
  c.stroke();
  assert.match(c.toString(), /d="M10 0L110 0"/);
});

test('scale scales line width and dash', () => {
  const c = mk();
  c.scale(2, 2);
  c.lineWidth = 3;
  c.setLineDash([1, 2]);
  c.beginPath();
  c.moveTo(0, 0);
  c.lineTo(5, 0);
  c.stroke();
  const s = c.toString();
  assert.match(s, /stroke-width="6"/);
  assert.match(s, /stroke-dasharray="2 4"/);
});

test('non-uniform scale strokes in user space under the CTM', () => {
  const c = mk();
  c.scale(2, 1);
  c.beginPath();
  c.moveTo(0, 0);
  c.lineTo(10, 0);
  c.stroke();
  const s = c.toString();
  assert.match(s, /d="M0 0L10 0"/);
  assert.match(s, /transform="matrix\(2 0 0 1 0 0\)"/);
});

test('odd dash arrays are doubled, invalid ones ignored', () => {
  const c = mk();
  c.setLineDash([3]);
  assert.deepEqual(c.getLineDash(), [3, 3]);
  c.setLineDash([-1, 2]);
  assert.deepEqual(c.getLineDash(), [3, 3]);
});

test('clip after save works, restore drops it', () => {
  const c = mk();
  c.save();
  c.beginPath();
  c.rect(0, 0, 10, 10);
  c.clip();
  c.beginPath();
  c.rect(1, 1, 2, 2);
  c.fill();
  c.restore();
  c.beginPath();
  c.rect(50, 50, 2, 2);
  c.fill();
  const s = c.toString();
  assert.equal(count(s, /<clipPath /g), 1);
  const body = s.slice(s.indexOf('</defs>'));
  assert.match(body, /<g clip-path="url\(#c1\)"><path [^>]*\/><\/g><path d="M50 50/);
});

test('nested clips intersect via parent clip-path', () => {
  const c = mk();
  c.beginPath();
  c.rect(0, 0, 10, 10);
  c.clip();
  c.save();
  c.beginPath();
  c.rect(5, 5, 10, 10);
  c.clip();
  c.fill();
  c.restore();
  c.fill();
  const s = c.toString();
  assert.match(s, /<clipPath id="c1"><path/);
  assert.match(s, /<clipPath id="c2" clip-path="url\(#c1\)">/);
  const body = s.slice(s.indexOf('</defs>'));
  // inner fill under c2, outer fill back under c1 only
  assert.ok(body.indexOf('#c2') < body.lastIndexOf('#c1'));
});

test('clip rule evenodd', () => {
  const c = mk();
  c.rect(0, 0, 4, 4);
  c.clip('evenodd');
  assert.match(c.toString(), /clip-rule="evenodd"/);
});

test('shadow filters are deduplicated by parameters, blur -> stdDeviation blur/2', () => {
  const c = mk();
  for (const x of [0, 30]) {
    c.save();
    c.shadowColor = 'rgba(0,0,0,0.35)';
    c.shadowBlur = 6;
    c.shadowOffsetX = 2;
    c.shadowOffsetY = 2;
    c.beginPath();
    c.rect(x, 0, 20, 20);
    c.fill();
    c.restore();
  }
  c.shadowColor = 'red';
  c.shadowBlur = 4;
  c.beginPath();
  c.rect(0, 40, 20, 20);
  c.fill();
  const s = c.toString();
  assert.equal(count(s, /<filter /g), 2);
  assert.match(s, /stdDeviation="3"/);
  assert.match(s, /stdDeviation="2"/);
  assert.match(s, /flood-opacity="0.35"/);
  assert.equal(count(s, /filter="url\(#f1\)"/g), 2);
});

test('no shadow without color alpha or without blur/offset', () => {
  const c = mk();
  c.shadowBlur = 5;
  c.rect(0, 0, 5, 5);
  c.fill();
  c.shadowColor = 'black';
  c.shadowBlur = 0;
  c.fill();
  assert.doesNotMatch(c.toString(), /filter/);
});

test('rgba fill split into color and opacity, globalAlpha multiplies', () => {
  const c = mk();
  c.fillStyle = 'rgba(255, 0, 0, 0.5)';
  c.globalAlpha = 0.5;
  c.rect(0, 0, 1, 1);
  c.fill();
  assert.match(c.toString(), /fill="rgb\(255,0,0\)" fill-opacity="0.25"/);
});

test('arcTo draws a tangent arc', () => {
  const c = mk();
  c.moveTo(0, 0);
  c.arcTo(10, 0, 10, 10, 5);
  c.stroke();
  assert.match(c.toString(), /d="M0 0L5 0C[^"]* 10 5"/);
});

test('text: font, anchor, baseline, escaping, transform', () => {
  const c = mk();
  c.font = 'italic bold 13px "Inter", sans-serif';
  c.textAlign = 'center';
  c.textBaseline = 'middle';
  c.fillText('a < b', 10, 20);
  c.save();
  c.translate(5, 5);
  c.fillText('x', 1, 1);
  c.rotate(Math.PI / 2);
  c.textBaseline = 'alphabetic';
  c.fillText('y', 0, 0);
  c.restore();
  const s = c.toString();
  assert.match(s, /<text x="10" y="20" font-family="&quot;Inter&quot;, sans-serif" font-size="13" font-style="italic" font-weight="bold" text-anchor="middle" dominant-baseline="central" fill="#000000">a &lt; b<\/text>/);
  assert.match(s, /<text x="6" y="6"/);
  assert.match(s, /x="0" y="0"[^>]*transform="matrix\(0 1 -1 0 5 5\)"/);
});

test('text maxWidth squeezes only when too wide; measureText delegates', () => {
  const c = mk();
  assert.equal(c.measureText('abcd').width, 24);
  c.fillText('abcd', 0, 0, 10);
  c.fillText('abcd', 0, 0, 100);
  const s = c.toString();
  assert.equal(count(s, /textLength="10"/g), 1);
  assert.equal(count(s, /textLength/g), 1);
});

test('measureText without a measurer throws', () => {
  assert.throws(() => new SvgContext({ width: 1, height: 1 }).measureText('x'));
});

test('drawImage: data URL emitted once in defs, reused', () => {
  const c = mk();
  const img = { src: 'data:image/png;base64,AAAA', naturalWidth: 4, naturalHeight: 2 };
  c.drawImage(img, 0, 0);
  c.drawImage(img, 10, 10, 8, 4);
  const s = c.toString();
  assert.equal(count(s, /data:image\/png/g), 1);
  assert.equal(count(s, /<use /g), 2);
  assert.match(s, /transform="matrix\(2 0 0 2 10 10\)"/);
});

test('id prefix', () => {
  const c = new SvgContext({ width: 1, height: 1, idPrefix: 'a-' });
  c.rect(0, 0, 1, 1);
  c.clip();
  assert.match(c.toString(), /id="a-c1"/);
});

test('numbers rounded to 3 decimals', () => {
  const c = mk();
  c.moveTo(1 / 3, 2 / 3);
  c.lineTo(1.00049, -0.0001);
  c.stroke();
  assert.match(c.toString(), /d="M0.333 0.667L1 0"/);
});

test('fillRect leaves the current path alone and honours the CTM', () => {
  const c = mk();
  c.rect(0, 0, 1, 1);
  c.translate(5, 5);
  c.scale(2, 2);
  c.fillRect(1, 1, 3, 4);
  c.fill();
  const s = c.toString();
  assert.match(s, /<rect x="7" y="7" width="6" height="8" fill="#000000"\/>/);
  assert.match(s, /<path d="M0 0L1 0L1 1L0 1ZM0 0"/);
});
