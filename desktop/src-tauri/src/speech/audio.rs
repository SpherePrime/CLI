use crate::error::{HostError, HostResult};

fn pcm_sample(sample: f32) -> i16 {
    ((sample.clamp(-1.0, 1.0) as f64 * 32767.0) + 0.5).floor() as i16
}

pub fn synthesize_audible(
    mut synthesize: impl FnMut() -> HostResult<Vec<f32>>,
) -> HostResult<Vec<f32>> {
    for _ in 0..2 {
        let samples = synthesize()?;
        if samples
            .iter()
            .any(|sample| !sample.is_finite() || pcm_sample(*sample) != 0)
        {
            return Ok(samples);
        }
    }
    Err(HostError::new(
        "SPEECH_SILENT",
        "Speech synthesis returned silent audio twice",
    ))
}

pub fn encode_wav(samples: &[f32], sample_rate: u32) -> HostResult<Vec<u8>> {
    if sample_rate == 0 || sample_rate > 192000 || samples.len() > sample_rate as usize * 600 {
        return Err(HostError::new(
            "SPEECH_AUDIO",
            "Speech audio exceeds ten minutes",
        ));
    }
    let length = samples
        .len()
        .checked_mul(2)
        .and_then(|length| u32::try_from(length).ok())
        .ok_or_else(|| HostError::new("SPEECH_AUDIO", "Speech audio is too large"))?;
    let mut output = Vec::with_capacity(44 + length as usize);
    output.extend_from_slice(b"RIFF");
    output.extend_from_slice(&(36 + length).to_le_bytes());
    output.extend_from_slice(b"WAVEfmt ");
    output.extend_from_slice(&16u32.to_le_bytes());
    output.extend_from_slice(&1u16.to_le_bytes());
    output.extend_from_slice(&1u16.to_le_bytes());
    output.extend_from_slice(&sample_rate.to_le_bytes());
    output.extend_from_slice(&(sample_rate * 2).to_le_bytes());
    output.extend_from_slice(&2u16.to_le_bytes());
    output.extend_from_slice(&16u16.to_le_bytes());
    output.extend_from_slice(b"data");
    output.extend_from_slice(&length.to_le_bytes());
    for sample in samples {
        if !sample.is_finite() {
            return Err(HostError::new(
                "SPEECH_AUDIO",
                "Model returned non-finite audio samples",
            ));
        }
        let sample = pcm_sample(*sample);
        output.extend_from_slice(&sample.to_le_bytes());
    }
    Ok(output)
}
