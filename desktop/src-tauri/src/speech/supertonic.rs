use super::{
    assets, runtime, text,
    voices::{self, Style, StyleTensor},
};
use crate::error::{HostError, HostResult};
use ort::{
    session::{builder::GraphOptimizationLevel, RunOptions, Session},
    value::Tensor,
};
use rand::Rng;
use serde::Deserialize;
use std::{
    collections::{HashMap, VecDeque},
    path::{Path, PathBuf},
    sync::atomic::{AtomicBool, Ordering},
};

#[derive(Deserialize)]
struct Config {
    ae: Autoencoder,
    ttl: TextToLatent,
}
#[derive(Deserialize)]
struct Autoencoder {
    sample_rate: u32,
    base_chunk_size: usize,
}
#[derive(Deserialize)]
struct TextToLatent {
    chunk_compress_factor: usize,
    latent_dim: usize,
}

pub struct Supertonic {
    config: Config,
    indexer: Vec<i64>,
    duration: Session,
    encoder: Session,
    estimator: Session,
    vocoder: Session,
    directory: PathBuf,
    styles: HashMap<String, Style>,
    style_order: VecDeque<String>,
}

impl Supertonic {
    pub fn load(directory: &Path, stop: &AtomicBool) -> HostResult<Self> {
        runtime::initialize(stop)?;
        let onnx = directory.join("onnx");
        let config: Config = serde_json::from_slice(&std::fs::read(onnx.join("tts.json"))?)?;
        let indexer: Vec<i64> =
            serde_json::from_slice(&std::fs::read(onnx.join("unicode_indexer.json"))?)?;
        if config.ae.sample_rate == 0
            || config.ae.sample_rate > 192000
            || config.ae.base_chunk_size == 0
            || config.ttl.chunk_compress_factor == 0
            || config.ttl.latent_dim == 0
        {
            return Err(error("Invalid Supertonic model configuration"));
        }
        let threads = (std::thread::available_parallelism()
            .map(usize::from)
            .unwrap_or(2)
            / 2)
        .clamp(1, 4);
        let create = |name: &str| -> HostResult<Session> {
            if stop.load(Ordering::Acquire) {
                return Err(assets::cancelled());
            }
            Session::builder()
                .map_err(ort_error)?
                .with_optimization_level(GraphOptimizationLevel::Level3)
                .map_err(ort_error)?
                .with_intra_threads(threads)
                .map_err(ort_error)?
                .with_inter_threads(1)
                .map_err(ort_error)?
                .commit_from_file(onnx.join(name))
                .map_err(ort_error)
        };
        let duration = create("duration_predictor.onnx")?;
        let encoder = create("text_encoder.onnx")?;
        let estimator = create("vector_estimator.onnx")?;
        let vocoder = create("vocoder.onnx")?;
        Ok(Self {
            config,
            indexer,
            duration,
            encoder,
            estimator,
            vocoder,
            directory: directory.into(),
            styles: HashMap::new(),
            style_order: VecDeque::new(),
        })
    }

    pub fn sample_rate(&self) -> u32 {
        self.config.ae.sample_rate
    }
    pub fn forget(&mut self, voice: &str) {
        self.styles.remove(voice);
        self.style_order.retain(|item| item != voice);
    }

    fn style(&mut self, voice: &str) -> HostResult<Style> {
        if let Some(style) = self.styles.get(voice) {
            return Ok(style.clone());
        }
        let style = voices::read(&self.directory, voice)?;
        if self.styles.len() >= 8 {
            if let Some(oldest) = self.style_order.pop_front() {
                self.styles.remove(&oldest);
            }
        }
        self.styles.insert(voice.into(), style.clone());
        self.style_order.push_back(voice.into());
        Ok(style)
    }

    pub fn synthesize(
        &mut self,
        input: &str,
        voice: &str,
        speed: f32,
        stop: &AtomicBool,
        options: &RunOptions,
    ) -> HostResult<Vec<f32>> {
        let style = self.style(voice)?;
        let prepared = text::prepare(input, text::detect_language(input));
        let codes = text::text_codes(&prepared, &self.indexer);
        if codes.len() > 16000 {
            return Err(error("Normalized speech text exceeds the token limit"));
        }
        let length = codes.len();
        let ids = Tensor::from_array(([1usize, length], codes)).map_err(ort_error)?;
        let mask =
            Tensor::from_array(([1usize, 1, length], vec![1.0f32; length])).map_err(ort_error)?;
        let style_dp = style_tensor(style.dp)?;
        let style_ttl = style_tensor(style.ttl)?;
        if stop.load(Ordering::Acquire) {
            return Err(assets::cancelled());
        }
        let seconds = {
            let output = self
                .duration
                .run_with_options(
                    ort::inputs!["text_ids" => &ids, "style_dp" => &style_dp, "text_mask" => &mask],
                    options,
                )
                .map_err(ort_error)?;
            let (_, duration) = output["duration"]
                .try_extract_tensor::<f32>()
                .map_err(ort_error)?;
            *duration
                .first()
                .ok_or_else(|| error("Model returned no duration"))?
                / speed
        };
        if !seconds.is_finite() || seconds <= 0.0 || seconds > 600.0 {
            return Err(error(
                "Predicted speech duration must be between zero and ten minutes",
            ));
        }
        let embedding = {
            let output = self.encoder.run_with_options(ort::inputs!["text_ids" => &ids, "style_ttl" => &style_ttl, "text_mask" => &mask], options).map_err(ort_error)?;
            let (shape, values) = output["text_emb"]
                .try_extract_tensor::<f32>()
                .map_err(ort_error)?;
            Tensor::from_array((shape.to_vec(), values.to_vec())).map_err(ort_error)?
        };
        let samples = (seconds * self.sample_rate() as f32).floor() as usize;
        #[cfg(test)]
        eprintln!("speech duration={seconds} samples={samples} text_length={length} speed={speed}");
        let chunk = self
            .config
            .ae
            .base_chunk_size
            .checked_mul(self.config.ttl.chunk_compress_factor)
            .ok_or_else(|| error("Invalid latent chunk size"))?;
        let latent_length = samples.div_ceil(chunk).max(1);
        let dimension = self
            .config
            .ttl
            .latent_dim
            .checked_mul(self.config.ttl.chunk_compress_factor)
            .ok_or_else(|| error("Invalid latent dimension"))?;
        let count = dimension
            .checked_mul(latent_length)
            .filter(|count| *count <= 16 * 1024 * 1024)
            .ok_or_else(|| error("Speech latent tensor exceeds the memory limit"))?;
        let mut random = rand::rng();
        let mut latent = (0..count)
            .map(|_| {
                let u1 = random.random::<f32>().max(1e-10);
                (-2.0 * u1.ln()).sqrt()
                    * (2.0 * std::f32::consts::PI * random.random::<f32>()).cos()
            })
            .collect::<Vec<_>>();
        #[cfg(test)]
        trace("noise", &latent);
        let latent_mask =
            Tensor::from_array(([1usize, 1, latent_length], vec![1.0f32; latent_length]))
                .map_err(ort_error)?;
        let total = Tensor::from_array(([1usize], vec![5.0f32])).map_err(ort_error)?;
        for step in 0..5 {
            if stop.load(Ordering::Acquire) {
                return Err(assets::cancelled());
            }
            let noisy = Tensor::from_array(([1usize, dimension, latent_length], latent))
                .map_err(ort_error)?;
            let current = Tensor::from_array(([1usize], vec![step as f32])).map_err(ort_error)?;
            let output = self.estimator.run_with_options(ort::inputs!["noisy_latent" => noisy, "text_emb" => &embedding, "style_ttl" => &style_ttl, "text_mask" => &mask, "latent_mask" => &latent_mask, "total_step" => &total, "current_step" => current], options).map_err(ort_error)?;
            let (_, values) = output["denoised_latent"]
                .try_extract_tensor::<f32>()
                .map_err(ort_error)?;
            if values.len() != count {
                return Err(error("Estimator returned an unexpected latent shape"));
            }
            latent = values.to_vec();
            #[cfg(test)]
            trace(&format!("step-{step}"), &latent);
        }
        if stop.load(Ordering::Acquire) {
            return Err(assets::cancelled());
        }
        let latent =
            Tensor::from_array(([1usize, dimension, latent_length], latent)).map_err(ort_error)?;
        let output = self
            .vocoder
            .run_with_options(ort::inputs!["latent" => latent], options)
            .map_err(ort_error)?;
        let (_, waveform) = output["wav_tts"]
            .try_extract_tensor::<f32>()
            .map_err(ort_error)?;
        #[cfg(test)]
        {
            trace("waveform-full", waveform);
            trace("waveform-trimmed", &waveform[..waveform.len().min(samples)]);
        }
        Ok(waveform[..waveform.len().min(samples)].to_vec())
    }
}

#[cfg(test)]
fn trace(stage: &str, values: &[f32]) {
    let peak = values
        .iter()
        .fold(0.0f32, |peak, value| peak.max(value.abs()));
    let energy: f64 = values.iter().map(|value| (*value as f64).powi(2)).sum();
    eprintln!(
        "speech stage={stage} count={} peak={peak} energy={energy}",
        values.len()
    );
}

fn style_tensor(style: StyleTensor) -> HostResult<Tensor<f32>> {
    Tensor::from_array((style.dims, style.data)).map_err(ort_error)
}
pub fn ort_error(error: impl std::fmt::Display) -> HostError {
    HostError::new("SPEECH_INFERENCE", error.to_string())
}
fn error(message: &str) -> HostError {
    HostError::new("SPEECH_MODEL", message)
}
