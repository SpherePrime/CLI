use fuzzy_matcher::FuzzyMatcher;
use fuzzy_matcher::skim::SkimMatcherV2;
use hyper::{Response, StatusCode};

use super::api::{json_response, BoxBody};

pub fn find(query: &str) -> Result<Response<BoxBody>, std::convert::Infallible> {
    let candidates = agent_ui::files::file_candidates();
    let matcher = SkimMatcherV2::default();

    let mut scored: Vec<(i64, String)> = if query.is_empty() {
        candidates.into_iter().map(|path| (0, path)).collect()
    } else {
        candidates
            .into_iter()
            .filter_map(|path| matcher.fuzzy_match(&path, query).map(|score| (score, path)))
            .collect()
    };

    scored.sort_by(|a, b| b.0.cmp(&a.0).then_with(|| a.1.cmp(&b.1)));
    scored.truncate(20);

    let data: Vec<serde_json::Value> = scored
        .into_iter()
        .map(|(_, path)| {
            let entry_type = {
                let full = std::path::Path::new(&path);
                if full.is_dir() {
                    "directory"
                } else {
                    "file"
                }
            };
            serde_json::json!({ "path": path, "type": entry_type })
        })
        .collect();

    Ok(json_response(&serde_json::json!({ "data": data })))
}

#[allow(dead_code)]
fn _unused_status() -> StatusCode {
    StatusCode::OK
}