const UNSUPPORTED: &str = "Application interface automation is currently supported only on Windows";

pub(crate) mod capture {
    use crate::appui::types::UiBounds;

    pub fn bounds_png(_hwnd: &str, _bounds: &UiBounds) -> Result<Vec<u8>, String> {
        Err(super::UNSUPPORTED.to_owned())
    }
}

pub(crate) mod worker {
    use std::time::Duration;

    use serde_json::Value;

    use crate::appui::ui_action::UiaRequest;

    pub struct UiaWorker;

    impl UiaWorker {
        pub fn start() -> Result<Self, String> {
            Err(super::UNSUPPORTED.to_owned())
        }

        pub fn call(&self, _request: UiaRequest, _timeout: Duration) -> Result<Value, String> {
            Err(super::UNSUPPORTED.to_owned())
        }
    }
}
