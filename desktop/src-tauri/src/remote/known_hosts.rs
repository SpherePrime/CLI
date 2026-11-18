use base64::engine::general_purpose::STANDARD;
use base64::Engine;
use russh::keys::ssh_key::PublicKey;
use russh::keys::Algorithm;
use sha1::{Digest, Sha1};
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum KnownHostCheck {
    Matched,
    Mismatch,
    NotFound,
}

pub fn check_user_known_hosts(host: &str, port: u16, public_key: &PublicKey) -> KnownHostCheck {
    let Some(path) = user_known_hosts_path() else {
        return KnownHostCheck::NotFound;
    };
    check_known_hosts_file(host, port, public_key, &path)
}

pub fn check_known_hosts_file(
    host: &str,
    port: u16,
    public_key: &PublicKey,
    path: &Path,
) -> KnownHostCheck {
    let Ok(contents) = std::fs::read_to_string(path) else {
        return KnownHostCheck::NotFound;
    };
    let normalized_host = host.trim().to_ascii_lowercase();
    if normalized_host.is_empty() || port == 0 {
        return KnownHostCheck::NotFound;
    }

    let mut matched = false;
    for raw_line in contents.lines() {
        let trimmed = raw_line.trim();
        if trimmed.is_empty() || trimmed.starts_with('#') {
            continue;
        }
        let Some((revoked, hosts_field, key_type, key_b64)) = parse_line(trimmed) else {
            continue;
        };
        if !hosts_field_matches(hosts_field, &normalized_host, port) {
            continue;
        }
        let Ok(entry_key) = russh::keys::parse_public_key_base64(key_b64) else {
            continue;
        };
        if entry_key.algorithm().as_str() != key_type {
            continue;
        }
        if revoked {
            if &entry_key == public_key {
                return KnownHostCheck::Mismatch;
            }
            continue;
        }
        if &entry_key == public_key {
            matched = true;
        } else if same_algorithm(&entry_key.algorithm(), &public_key.algorithm()) {
            return KnownHostCheck::Mismatch;
        }
    }

    if matched {
        KnownHostCheck::Matched
    } else {
        KnownHostCheck::NotFound
    }
}

fn user_known_hosts_path() -> Option<PathBuf> {
    #[cfg(windows)]
    let home = std::env::var_os("USERPROFILE")
        .filter(|value| !value.is_empty())
        .or_else(|| std::env::var_os("HOME").filter(|value| !value.is_empty()));
    #[cfg(not(windows))]
    let home = std::env::var_os("HOME")
        .filter(|value| !value.is_empty())
        .or_else(|| std::env::var_os("USERPROFILE").filter(|value| !value.is_empty()));
    home.map(|dir| PathBuf::from(dir).join(".ssh").join("known_hosts"))
}

fn parse_line(line: &str) -> Option<(bool, &str, &str, &str)> {
    let mut parts = line.split_whitespace();
    let first = parts.next()?;
    if first.starts_with('@') {
        if first == "@revoked" {
            let hosts = parts.next()?;
            let key_type = parts.next()?;
            let key_b64 = parts.next()?;
            return Some((true, hosts, key_type, key_b64));
        }
        return None;
    }
    let key_type = parts.next()?;
    let key_b64 = parts.next()?;
    Some((false, first, key_type, key_b64))
}

fn same_algorithm(a: &Algorithm, b: &Algorithm) -> bool {
    matches!(
        (a, b),
        (Algorithm::Rsa { .. }, Algorithm::Rsa { .. })
    ) || a == b
}

fn hosts_field_matches(hosts_field: &str, host_lower: &str, port: u16) -> bool {
    let mut matched = false;
    for raw in hosts_field.split(',') {
        let pattern = raw.trim();
        if pattern.is_empty() {
            continue;
        }
        if let Some(negated) = pattern.strip_prefix('!') {
            if !negated.is_empty() && host_pattern_matches(negated, host_lower, port) {
                return false;
            }
        } else if host_pattern_matches(pattern, host_lower, port) {
            matched = true;
        }
    }
    matched
}

fn host_pattern_matches(pattern: &str, host_lower: &str, port: u16) -> bool {
    if pattern.starts_with("|1|") {
        return hashed_host_matches(pattern, host_lower, port);
    }
    if pattern.starts_with('|') {
        return false;
    }
    if let Some(rest) = pattern.strip_prefix('[') {
        let Some((pat_host, pat_port_str)) = rest.split_once("]:") else {
            return false;
        };
        let Ok(pat_port) = pat_port_str.parse::<u16>() else {
            return false;
        };
        if pat_port != port {
            return false;
        }
        return wildcard_match(&pat_host.to_ascii_lowercase(), host_lower);
    }
    if port != 22 {
        return false;
    }
    wildcard_match(&pattern.to_ascii_lowercase(), host_lower)
}

fn hashed_host_matches(pattern: &str, host_lower: &str, port: u16) -> bool {
    let Some(rest) = pattern.strip_prefix("|1|") else {
        return false;
    };
    let Some((salt_b64, hash_b64)) = rest.split_once('|') else {
        return false;
    };
    if hash_b64.contains('|') {
        return false;
    }
    let Ok(salt) = STANDARD.decode(salt_b64) else {
        return false;
    };
    let Ok(expected_hash) = STANDARD.decode(hash_b64) else {
        return false;
    };
    if salt.is_empty() || expected_hash.len() != 20 {
        return false;
    }
    let target = if port == 22 {
        host_lower.to_owned()
    } else {
        format!("[{host_lower}]:{port}")
    };
    let computed = hmac_sha1(&salt, target.as_bytes());
    computed.as_slice() == expected_hash.as_slice()
}

fn hmac_sha1(key: &[u8], data: &[u8]) -> [u8; 20] {
    let mut key_block = [0u8; 64];
    if key.len() > 64 {
        let hashed = Sha1::digest(key);
        key_block[..20].copy_from_slice(&hashed);
    } else {
        key_block[..key.len()].copy_from_slice(key);
    }
    let mut ipad = [0x36u8; 64];
    let mut opad = [0x5cu8; 64];
    for index in 0..64 {
        ipad[index] ^= key_block[index];
        opad[index] ^= key_block[index];
    }
    let mut inner = Sha1::new();
    inner.update(ipad);
    inner.update(data);
    let inner_digest = inner.finalize();

    let mut outer = Sha1::new();
    outer.update(opad);
    outer.update(inner_digest);
    outer.finalize().into()
}

fn wildcard_match(pattern: &str, text: &str) -> bool {
    let pattern_chars: Vec<char> = pattern.chars().collect();
    let text_chars: Vec<char> = text.chars().collect();
    let mut pi = 0usize;
    let mut ti = 0usize;
    let mut star = None;
    let mut match_ti = 0usize;

    while ti < text_chars.len() {
        if pi < pattern_chars.len()
            && (pattern_chars[pi] == '?' || pattern_chars[pi] == text_chars[ti])
        {
            pi += 1;
            ti += 1;
        } else if pi < pattern_chars.len() && pattern_chars[pi] == '*' {
            star = Some(pi);
            match_ti = ti;
            pi += 1;
        } else if let Some(star_pi) = star {
            pi = star_pi + 1;
            match_ti += 1;
            ti = match_ti;
        } else {
            return false;
        }
    }

    while pi < pattern_chars.len() && pattern_chars[pi] == '*' {
        pi += 1;
    }
    pi == pattern_chars.len()
}

#[cfg(test)]
mod tests {
    use super::*;
    use russh::keys::key::safe_rng;
    use russh::keys::ssh_key::EcdsaCurve;
    use russh::keys::{PrivateKey, PublicKeyBase64};
    use std::io::Write;

    fn random_ed25519() -> PublicKey {
        PrivateKey::random(&mut safe_rng(), Algorithm::Ed25519)
            .unwrap()
            .public_key()
            .clone()
    }

    fn random_ecdsa() -> PublicKey {
        PrivateKey::random(
            &mut safe_rng(),
            Algorithm::Ecdsa {
                curve: EcdsaCurve::NistP256,
            },
        )
        .unwrap()
        .public_key()
        .clone()
    }

    fn key_line(hosts: &str, key: &PublicKey) -> String {
        format!(
            "{} {} {}",
            hosts,
            key.algorithm().as_str(),
            key.public_key_base64()
        )
    }

    fn write_known_hosts(contents: &str) -> tempfile::NamedTempFile {
        let mut file = tempfile::NamedTempFile::new().unwrap();
        file.write_all(contents.as_bytes()).unwrap();
        file.flush().unwrap();
        file
    }

    fn hashed_entry(host: &str, port: u16, salt: &[u8]) -> String {
        let target = if port == 22 {
            host.to_ascii_lowercase()
        } else {
            format!("[{}]:{}", host.to_ascii_lowercase(), port)
        };
        let digest = hmac_sha1(salt, target.as_bytes());
        format!("|1|{}|{}", STANDARD.encode(salt), STANDARD.encode(digest))
    }

    #[test]
    fn matches_plain_host_port_22_case_insensitive() {
        let key = random_ed25519();
        let file = write_known_hosts(&format!("{}\n", key_line("Example.COM", &key)));
        assert_eq!(
            check_known_hosts_file("example.com", 22, &key, file.path()),
            KnownHostCheck::Matched
        );
        assert_eq!(
            check_known_hosts_file("example.com", 2222, &key, file.path()),
            KnownHostCheck::NotFound
        );
    }

    #[test]
    fn matches_bracketed_host_and_port() {
        let key = random_ed25519();
        let file = write_known_hosts(&format!("{}\r\n", key_line("[10.0.0.5]:2222", &key)));
        assert_eq!(
            check_known_hosts_file("10.0.0.5", 2222, &key, file.path()),
            KnownHostCheck::Matched
        );
        assert_eq!(
            check_known_hosts_file("10.0.0.5", 22, &key, file.path()),
            KnownHostCheck::NotFound
        );
    }

    #[test]
    fn allows_multiple_algorithms_for_same_host() {
        let ecdsa = random_ecdsa();
        let ed = random_ed25519();
        let contents = format!(
            "{}\n{}\n",
            key_line("multi.example.com", &ecdsa),
            key_line("multi.example.com", &ed)
        );
        let file = write_known_hosts(&contents);
        assert_eq!(
            check_known_hosts_file("multi.example.com", 22, &ed, file.path()),
            KnownHostCheck::Matched
        );
        assert_eq!(
            check_known_hosts_file("multi.example.com", 22, &ecdsa, file.path()),
            KnownHostCheck::Matched
        );
    }

    #[test]
    fn detects_same_algorithm_mismatch() {
        let stored = random_ed25519();
        let presented = random_ed25519();
        let file = write_known_hosts(&format!("{}\n", key_line("server.local", &stored)));
        assert_eq!(
            check_known_hosts_file("server.local", 22, &presented, file.path()),
            KnownHostCheck::Mismatch
        );
    }

    #[test]
    fn rejects_revoked_key_even_if_also_listed() {
        let key = random_ed25519();
        let other = random_ed25519();
        let contents = format!(
            "{}\n@revoked {}\n",
            key_line("revoked.example.com", &key),
            key_line("revoked.example.com", &key)
        );
        let file = write_known_hosts(&contents);
        assert_eq!(
            check_known_hosts_file("revoked.example.com", 22, &key, file.path()),
            KnownHostCheck::Mismatch
        );
        assert_eq!(
            check_known_hosts_file("revoked.example.com", 22, &other, file.path()),
            KnownHostCheck::Mismatch
        );
    }

    #[test]
    fn revoked_old_key_does_not_block_new_key() {
        let old_key = random_ed25519();
        let new_key = random_ed25519();
        let contents = format!(
            "@revoked {}\n{}\n",
            key_line("rotated.example.com", &old_key),
            key_line("rotated.example.com", &new_key)
        );
        let file = write_known_hosts(&contents);
        assert_eq!(
            check_known_hosts_file("rotated.example.com", 22, &old_key, file.path()),
            KnownHostCheck::Mismatch
        );
        assert_eq!(
            check_known_hosts_file("rotated.example.com", 22, &new_key, file.path()),
            KnownHostCheck::Matched
        );
    }

    #[test]
    fn matches_hashed_entries_for_default_and_custom_ports() {
        let key = random_ed25519();
        let salt1 = [1u8; 20];
        let salt2 = [2u8; 20];
        let h22 = hashed_entry("hashed.example.com", 22, &salt1);
        let h2222 = hashed_entry("hashed.example.com", 2222, &salt2);
        let contents = format!("{}\n{}\n", key_line(&h22, &key), key_line(&h2222, &key));
        let file = write_known_hosts(&contents);
        assert_eq!(
            check_known_hosts_file("HASHED.example.com", 22, &key, file.path()),
            KnownHostCheck::Matched
        );
        assert_eq!(
            check_known_hosts_file("hashed.example.com", 2222, &key, file.path()),
            KnownHostCheck::Matched
        );
        assert_eq!(
            check_known_hosts_file("hashed.example.com", 2200, &key, file.path()),
            KnownHostCheck::NotFound
        );
    }

    #[test]
    fn honors_negated_patterns_and_comma_lists() {
        let key = random_ed25519();
        let contents = format!(
            "{}\n",
            key_line("*.internal.net,!bastion.internal.net,10.1.2.3", &key)
        );
        let file = write_known_hosts(&contents);
        assert_eq!(
            check_known_hosts_file("app.internal.net", 22, &key, file.path()),
            KnownHostCheck::Matched
        );
        assert_eq!(
            check_known_hosts_file("bastion.internal.net", 22, &key, file.path()),
            KnownHostCheck::NotFound
        );
        assert_eq!(
            check_known_hosts_file("10.1.2.3", 22, &key, file.path()),
            KnownHostCheck::Matched
        );
    }

    #[test]
    fn skips_comments_cert_authority_and_malformed_lines() {
        let key = random_ed25519();
        let contents = format!(
            "\n# comment line\n@cert-authority *.example.com ssh-ed25519 AAAA\nbroken_line_only_two_tokens ssh-ed25519\nbadkey.example.com ssh-ed25519 not_valid_base64!!!\n{}\n",
            key_line("good.example.com", &key)
        );
        let file = write_known_hosts(&contents);
        assert_eq!(
            check_known_hosts_file("good.example.com", 22, &key, file.path()),
            KnownHostCheck::Matched
        );
        assert_eq!(
            check_known_hosts_file("badkey.example.com", 22, &key, file.path()),
            KnownHostCheck::NotFound
        );
    }
}
