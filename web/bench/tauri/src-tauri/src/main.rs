#[tauri::command]
fn save_results(json: String) {
    let p = std::env::var("BENCH_OUT").unwrap_or_else(|_| "/tmp/tauri-bench.json".into());
    std::fs::write(&p, &json).unwrap();
    println!("RESULTS_WRITTEN {}", p);
    if std::env::var("BENCH_EXIT").is_ok() { std::process::exit(0); }
}
fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![save_results])
        .run(tauri::generate_context!())
        .expect("run");
}
