# Voice input

Dictate prompts in the TUI instead of typing. Prime captures the microphone
and transcribes speech with Whisper, then inserts the text at the cursor in the
prompt.

## Use it

| Action                            | Key |
| --------------------------------- | --- |
| Start or stop recording           | <kbd>alt+v</kbd> or <kbd>ctrl+shift+space</kbd> |
| Drop the recording                | <kbd>esc</kbd> |
| Dictation from the command palette | <kbd>ctrl+p</kbd> → Start Voice Dictation |

While recording, the status bar shows a red microphone badge with the elapsed
time, for example `● REC 0:07`. Stop with the same key and the badge changes to
`● WHISPER` while the audio is transcribed. The result is inserted where the
cursor already was, so you can dictate in the middle of a sentence.

Nothing is sent anywhere until you stop recording, and nothing is sent to the
model until you press enter.

## Install it in one step

```
prime voice setup
```

This downloads a prebuilt whisper.cpp engine and the default model, Whisper
Large V3 Turbo quantized to q5_0 (about 574 MB), into Prime's own data
directory, and installs a microphone recorder when none is present. Nothing is
written to system locations and no administrator rights are needed. Already
working pieces are left alone.

Pick a different model with `--model small`, preview without installing with
`--print`, or skip the prompt for scripts with `--yes`.

## The always-on server

Loading a half-gigabyte model on every dictation is what makes whisper.cpp
slow to answer, so Prime keeps a local server loaded instead:

```bash
prime voice server           # report state, model, pid, and idle time
prime voice server start     # load the model now instead of at first dictation
prime voice server touch     # restart the idle countdown
prime voice server stop      # free the model's memory right away
```

How it behaves:

- The TUI warms the server in the background at launch, and dictation prefers
  it over the one-shot CLI while it runs.
- It listens on `http://127.0.0.1:8000` only and uses every CPU core Prime
  can see, with your `option voice language` at startup (detection when unset).
- It stops itself about 15 minutes after the last dictation through a small
  detached watcher, so the RAM comes back even after Prime exits.
- If the terminal that owned the server was killed, the watcher restarts
  Prime in a fresh window and clears session files that died mid-turn. A live
  Prime elsewhere means it stays hands-off.
- Change the model or language in the options and the server restarts with
  the new settings on next use.
- If anything about the server fails, dictation transparently falls back to
  the `whisper-cli` command, so a broken server never blocks voice input.

A server you started yourself on `127.0.0.1:8000` is used as-is and Prime
never kills it.

Pinning `option voice language ru` skips per-request language detection,
which is worth roughly half the wait on short phrases. The default Large V3
Turbo model is accurate but not instant on CPU; a smaller model answers
faster if speed matters more than recognition quality.

## What Prime looks for

Prime does not bundle audio code. It uses tools that are already installed,
checked in this order:

**Microphone**

| Backend                | Platforms        | How to get it |
| ---------------------- | ---------------- | ------------- |
| `record-command`       | all              | `option voice record-command "..."` |
| Python + sounddevice   | all              | `pip install sounddevice` |
| ffmpeg                 | all              | `winget install Gyan.FFmpeg`, `brew install ffmpeg` |
| sox (`rec`)            | macOS, Linux     | `brew install sox`, `apt install sox` |
| `pw-record`, `arecord` | Linux            | shipped with PipeWire and ALSA |

**Whisper**

| Backend                | Notes |
| ---------------------- | ----- |
| `transcribe-command`   | set with `option voice transcribe-command` |
| whisper.cpp server     | Prime-managed and always-on, see below; auto-detected on `http://127.0.0.1:8000`, or point `base-url` at it |
| whisper.cpp CLI        | `whisper-cli` plus a ggml model file; the fallback whenever the server is unavailable |
| openai-whisper (Python)| `pip install -U openai-whisper` |
| whisper-ctranslate2    | `pip install whisper-ctranslate2` |
| OpenAI-compatible API  | `OPENAI_API_KEY` or `GROQ_API_KEY`, or set `base-url` and `api-key` |

Capture always runs at 16 kHz mono, which is what Whisper expects. Local
engines are preferred over hosted ones so audio stays on your machine unless
you configure otherwise.

## Examples

```bash
# Russian dictation with a local whisper.cpp server
option voice language ru
option voice base-url http://localhost:8000

# Hosted transcription, no local model
option voice base-url https://api.openai.com/v1
option voice api-key $OPENAI_API_KEY
option voice model whisper-1

# A bigger local model
option voice model ~/.cache/whisper.cpp/ggml-small.bin   # whisper.cpp
option voice model small                                  # Python engines
```

## Check the setup

```bash
prime voice          # list recorders and engines found, and the pair in use
prime voice test     # record three seconds and print the transcript
prime voice test --duration 6 --save dictation.wav
```

If a hotkey is already taken by your terminal, rebind it:

```bash
option voice hotkey f7
```

## Notes

- Recording is capped by `max-duration` (300 seconds by default) and then
  transcribed automatically.
- macOS requires microphone permission for the terminal app; the system prompts
  once.
- Tapping the hotkey for less than a third of a second is treated as a mistake
  and the audio is discarded.
