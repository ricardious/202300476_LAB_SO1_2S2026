use axum::{extract::{State, DefaultBodyLimit}, http::{StatusCode, HeaderMap}, routing::{get, post}, Json, Router};
use serde::{Deserialize, Serialize};
use std::{sync::Arc, time::Duration};
#[derive(Clone)] struct App { http: reqwest::Client, upstream: String }
#[derive(Deserialize, Serialize)] #[serde(deny_unknown_fields)]
struct Report { country: String, warplanes_in_air: i32, warships_in_water: i32, timestamp: String }
#[derive(Serialize)] struct Forward<'a> { #[serde(flatten)] report: &'a Report, report_id: String }
fn valid(r: &Report) -> bool {
    ["USA","RUS","CHN","ESP","GTM"].contains(&r.country.as_str()) &&
    (0..=50).contains(&r.warplanes_in_air) && (0..=30).contains(&r.warships_in_water) &&
    chrono::DateTime::parse_from_rfc3339(&r.timestamp).is_ok()
}
async fn receive(State(app): State<Arc<App>>, headers: HeaderMap, Json(r): Json<Report>) -> (StatusCode, Json<serde_json::Value>) {
    if !valid(&r) { return (StatusCode::UNPROCESSABLE_ENTITY, Json(serde_json::json!({"error":"invalid_report"}))); }
    let id = headers.get("idempotency-key").and_then(|h|h.to_str().ok()).filter(|s|!s.is_empty() && s.len()<=128).map(str::to_owned).unwrap_or_else(||uuid::Uuid::new_v4().to_string());
    match app.http.post(&app.upstream).json(&Forward{report:&r,report_id:id.clone()}).send().await {
        Ok(response) if response.status().is_success() => {
            println!("report_id={} country={} status=queued", id, r.country);
            (StatusCode::ACCEPTED,Json(serde_json::json!({"status":"queued","report_id":id})))
        }
        Ok(response) => { eprintln!("report_id={} upstream_status={}",id,response.status()); (StatusCode::BAD_GATEWAY,Json(serde_json::json!({"error":"upstream_failed"}))) }
        Err(e) => { eprintln!("report_id={} upstream_error={}",id,e); (StatusCode::SERVICE_UNAVAILABLE,Json(serde_json::json!({"error":"upstream_unavailable"}))) }
    }
}
#[tokio::main(flavor="multi_thread", worker_threads=2)]
async fn main() {
    let http=reqwest::Client::builder().timeout(Duration::from_secs(5)).connect_timeout(Duration::from_secs(1)).pool_max_idle_per_host(8).build().unwrap();
    let app=Arc::new(App{http,upstream:std::env::var("GO_API_URL").unwrap_or("http://go-ingest:8080/report".into())});
    let router=Router::new().route("/healthz",get(||async{"ok"})).route("/grpc-202300476",post(receive)).layer(DefaultBodyLimit::max(4096)).with_state(app);
    let listener=tokio::net::TcpListener::bind("0.0.0.0:8080").await.unwrap();
    axum::serve(listener,router).with_graceful_shutdown(async{let _=tokio::signal::ctrl_c().await;}).await.unwrap();
}
#[cfg(test)] mod tests {
 use super::*;
 #[test] fn validates_boundaries_and_timestamp() {
  let mut r=Report{country:"ESP".into(),warplanes_in_air:0,warships_in_water:30,timestamp:"2026-10-19T12:00:00Z".into()};
  assert!(valid(&r));r.country="GMT".into();assert!(!valid(&r));r.country="ESP".into();r.warplanes_in_air=51;assert!(!valid(&r));r.warplanes_in_air=50;r.timestamp="invalid".into();assert!(!valid(&r));
 }
}
