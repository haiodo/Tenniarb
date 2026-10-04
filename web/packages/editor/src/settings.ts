// Swift PreferencesController keys that have a consumer in the web editor.
export interface EditorSettings {
  /** preferences.structure.auto_expand: expand the outline on open. */
  autoExpand: boolean;
  /** preferences.structure.expand_level */
  expandLevel: number;
  /** preferences.colors.background / background_dark, "#rrggbb[aa]". */
  background: string;
  backgroundDark: string;
  /** preferences.ui.transparent_background */
  transparent: boolean;
  /** preferences.ui.quick_panel_top ("Show popup quick panel"): the style panel above the selected item. */
  quickPanel: boolean;
  /** preferences.render.enable_background: fill the PNG export. */
  exportBackground: boolean;
  /** preferences.render.enable_hidpi */
  exportNativeScale: boolean;
}

export const defaultSettings: EditorSettings = {
  autoExpand: true,
  expandLevel: 2,
  background: "#e7e9ebff",
  backgroundDark: "#2e2e2eff",
  transparent: false,
  quickPanel: true,
  exportBackground: true,
  exportNativeScale: true,
};
