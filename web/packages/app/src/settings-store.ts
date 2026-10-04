import { BaseDirectory, mkdir, readTextFile, writeTextFile } from "@tauri-apps/plugin-fs";
import { defaultSettings } from "@tenniarb/editor/settings";
import type { EditorSettings } from "@tenniarb/editor/settings";

const file = "settings.json";
const appData = { baseDir: BaseDirectory.AppData };
export const settingsEvent = "settings-changed";

export async function loadSettings(): Promise<EditorSettings> {
  try {
    return { ...defaultSettings, ...(JSON.parse(await readTextFile(file, appData)) as Partial<EditorSettings>) };
  } catch {
    return { ...defaultSettings };
  }
}

export async function saveSettings(s: EditorSettings): Promise<void> {
  await mkdir("", { ...appData, recursive: true });
  await writeTextFile(file, JSON.stringify(s, null, 2), appData);
}
