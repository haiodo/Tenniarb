import { emit } from "@tauri-apps/api/event";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { defaultSettings } from "@tenniarb/editor/settings";
import type { EditorSettings } from "@tenniarb/editor/settings";
import { loadSettings, saveSettings, settingsEvent } from "./settings-store.ts";

let settings = await loadSettings();
const field = (key: keyof EditorSettings): HTMLInputElement => document.getElementById(key) as HTMLInputElement;

async function update(patch: Partial<EditorSettings>): Promise<void> {
  settings = { ...settings, ...patch };
  await saveSettings(settings);
  await emit(settingsEvent, settings);
}

for (const key of Object.keys(defaultSettings) as (keyof EditorSettings)[]) {
  const el = field(key);
  const v = settings[key];
  if (typeof v === "boolean") el.checked = v;
  else el.value = String(v);
  el.addEventListener("change", () => {
    if (typeof v === "boolean") return void update({ [key]: el.checked });
    if (!el.checkValidity() || el.value === "") return void (el.value = String(settings[key]));
    if (typeof v === "number") return void update({ [key]: Math.trunc(el.valueAsNumber) });
    void update({ [key]: el.value.startsWith("#") ? el.value : `#${el.value}` });
    el.value = String(settings[key]);
  });
}

for (const b of document.querySelectorAll<HTMLButtonElement>("button[data-reset]")) {
  b.addEventListener("click", () => {
    const key = b.dataset.reset as "background" | "backgroundDark";
    field(key).value = defaultSettings[key];
    void update({ [key]: defaultSettings[key] });
  });
}

document.addEventListener("keydown", (ev) => ev.key === "Escape" && void getCurrentWindow().close());
