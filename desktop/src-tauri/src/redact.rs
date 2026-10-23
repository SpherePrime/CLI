use serde_json::Value;

pub fn sensitive_key(key: &str) -> bool {
    matches!(
        key.to_ascii_lowercase().as_str(),
        "api_key"
            | "apikey"
            | "access_token"
            | "refresh_token"
            | "id_token"
            | "secret"
            | "authorization"
            | "headers"
            | "env"
            | "oauth"
            | "oauth_token"
            | "oauth_client_secret"
    )
}

#[cfg(test)]
mod tests {
    #[test]
    fn nested_credentials_are_removed_and_oauth_flag_preserved() {
        let result = super::value(
            serde_json::json!({"api_key":"secret","oauth":true,"provider":{"refresh_token":"secret","name":"saved"}}),
        );
        assert_eq!(
            result,
            serde_json::json!({"oauth":true,"provider":{"name":"saved"}})
        );
    }
}

pub fn value(input: Value) -> Value {
    match input {
        Value::Object(items) => Value::Object(
            items
                .into_iter()
                .filter(|(key, item)| key == "oauth" && item.is_boolean() || !sensitive_key(key))
                .map(|(key, item)| (key, value(item)))
                .collect(),
        ),
        Value::Array(items) => Value::Array(items.into_iter().map(value).collect()),
        item => item,
    }
}
