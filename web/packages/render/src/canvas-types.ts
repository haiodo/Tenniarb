// The exact Canvas2D subset the renderer calls. CanvasRenderingContext2D (browser), @napi-rs/canvas and the SVG backend all fit it.
// Property types are widened (`object`, `unknown`) so that each backend's own CanvasGradient/Image classes still assign.
export interface Canvas2D {
  save(): void;
  restore(): void;
  translate(x: number, y: number): void;
  scale(x: number, y: number): void;

  beginPath(): void;
  moveTo(x: number, y: number): void;
  lineTo(x: number, y: number): void;
  arcTo(x1: number, y1: number, x2: number, y2: number, radius: number): void;
  quadraticCurveTo(cpx: number, cpy: number, x: number, y: number): void;
  closePath(): void;
  rect(x: number, y: number, w: number, h: number): void;
  ellipse(x: number, y: number, radiusX: number, radiusY: number, rotation: number, startAngle: number, endAngle: number): void;
  clip(): void;
  fill(): void;
  stroke(): void;
  fillRect(x: number, y: number, w: number, h: number): void;

  fillStyle: string | object;
  strokeStyle: string | object;
  lineWidth: number;
  lineDashOffset: number;
  lineCap?: string;
  setLineDash(segments: number[]): void;

  shadowColor: string;
  shadowBlur: number;
  shadowOffsetX: number;
  shadowOffsetY: number;

  font: string;
  textBaseline: string;
  fillText(text: string, x: number, y: number): void;
  measureText(text: string): { width: number };

  drawImage(image: any, dx: number, dy: number, dw: number, dh: number): void;
}
