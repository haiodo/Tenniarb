use std::sync::Mutex;
use tauri::{Emitter, Manager, RunEvent};

// Files from argv / macOS "Open With"; the frontend drains them, so a cold start (no listener yet) loses nothing.
struct Opened(Mutex<Vec<String>>);

#[tauri::command]
fn take_opened(opened: tauri::State<Opened>) -> Vec<String> {
    std::mem::take(&mut *opened.0.lock().unwrap())
}

fn main() {
    let argv = std::env::args().skip(1).filter(|a| a.ends_with(".tenn")).collect();
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_fs::init())
        .manage(Opened(Mutex::new(argv)))
        .invoke_handler(tauri::generate_handler![take_opened])
        .build(tauri::generate_context!())
        .expect("error while building tauri application")
        .run(|app, event| {
            #[cfg(target_os = "macos")]
            if let RunEvent::Opened { urls } = event {
                let paths = urls.iter().filter_map(|u| u.to_file_path().ok()).map(|p| p.to_string_lossy().into_owned());
                app.state::<Opened>().0.lock().unwrap().extend(paths);
                if let Some(label) = app.webview_windows().keys().next() {
                    let _ = app.emit_to(label.as_str(), "open-paths", ());
                }
            }
            #[cfg(not(target_os = "macos"))]
            let _ = (app, event);
        });
}
