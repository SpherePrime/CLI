use tonic::Request;
use tonic::metadata::MetadataValue;

pub const TOKEN_HEADER: &str = "x-prime-asr-token";

pub mod pb {
    tonic::include_proto!("prime.asr.v1");
}

pub fn with_token<T>(payload: T, token: &str) -> Result<Request<T>, String> {
    let mut request = Request::new(payload);
    let value = MetadataValue::try_from(token)
        .map_err(|err| format!("invalid prime-asr auth token metadata: {err}"))?;
    request.metadata_mut().insert(TOKEN_HEADER, value);
    Ok(request)
}
