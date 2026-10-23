#!/usr/bin/env python3
"""37Studio's local transcription: faster-whisper, sized for this computer. Free; the audio never leaves the machine.

Usage: transcribe.py MEDIA --out DIR [--language bn] [--quality auto|fast|best] [--model NAME]

Writes DIR/transcript.json (segments with word timestamps), transcript.srt and transcript.txt, prints progress with an
estimate to stderr, and one JSON summary line to stdout. Sequential beam search on purpose: on a CPU, faster-whisper's
batched mode was no faster at beam 5 and dropped or garbled Bangla (Devanagari letters, lost sentences) when it was. A second run on the same file and settings reuses the result.
"""
import argparse
import hashlib
import json
import os
import subprocess
import sys
import time


def log(msg):
    print(msg, file=sys.stderr, flush=True)


def ram_gb():
    """The container's memory limit when there is one (Docker), else the machine's RAM."""
    for path in ("/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory/memory.limit_in_bytes"):
        try:
            v = open(path).read().strip()
            if v.isdigit() and int(v) < 1 << 50:
                return int(v) / 2**30
        except OSError:
            pass
    try:
        for line in open("/proc/meminfo"):
            if line.startswith("MemTotal:"):
                return int(line.split()[1]) / 2**20
    except OSError:
        pass
    try:  # macOS, outside Docker
        return int(subprocess.check_output(["sysctl", "-n", "hw.memsize"])) / 2**30
    except Exception:
        return 8.0


def cpu_count():
    try:  # a Docker CPU limit
        quota, period = open("/sys/fs/cgroup/cpu.max").read().split()
        if quota != "max":
            return max(1, int(int(quota) / int(period)))
    except Exception:
        pass
    return os.cpu_count() or 4


def load_audio(path):
    """Mono 16 kHz samples via ffmpeg, which reads anything (faster-whisper's own PyAV decoder breaks on newer PyAV)."""
    import numpy as np
    raw = subprocess.run(["ffmpeg", "-nostdin", "-v", "error", "-i", path, "-vn", "-ac", "1", "-ar", "16000", "-f", "s16le", "-"],
                         capture_output=True, check=True).stdout
    return np.frombuffer(raw, np.int16).astype(np.float32) / 32768


def pick_model(quality, cuda, ram):
    """(model, device, compute type). large-v3-turbo is the best speed/quality trade on a CPU, Bangla included:
    several times faster than large-v3 for a small loss. large-v3 is the most accurate, and the default on a GPU."""
    if quality == "best" or cuda:
        return ("large-v3", "cuda" if cuda else "cpu", "float16" if cuda else "int8")
    if quality == "fast" or ram < 4:
        return ("small", "cpu", "int8")  # much weaker on Bangla and other non-English speech
    return ("large-v3-turbo", "cpu", "int8")


def clock(s):
    s = int(s)
    return f"{s // 3600}:{s % 3600 // 60:02d}:{s % 60:02d}"


def srt_time(s):
    ms = int(round(s * 1000))
    return f"{ms // 3600000:02d}:{ms % 3600000 // 60000:02d}:{ms % 60000 // 1000:02d},{ms % 1000:03d}"


def main():
    ap = argparse.ArgumentParser(description="Transcribe audio or video on this computer with faster-whisper.")
    ap.add_argument("media")
    ap.add_argument("--out", required=True, help="output folder, e.g. work/transcript")
    ap.add_argument("--language", default=None, help="language code, e.g. bn for Bangla, en for English; detected when left out")
    ap.add_argument("--quality", default="auto", choices=["auto", "fast", "best"])
    ap.add_argument("--model", default=None, help="override the model (any faster-whisper name or CTranslate2 model on Hugging Face)")
    a = ap.parse_args()

    import ctranslate2  # installed with faster-whisper
    from faster_whisper import WhisperModel

    # ponytail: an NVIDIA card counts only outside Docker, and needs CUDA 12 cuBLAS/cuDNN installed; add a --device flag
    # if someone's GPU is visible but those libraries aren't.
    cuda = ctranslate2.get_cuda_device_count() > 0
    ram, cores = ram_gb(), cpu_count()
    model, device, compute = pick_model(a.quality, cuda, ram)
    model = a.model or model
    os.makedirs(a.out, exist_ok=True)

    st = os.stat(a.media)
    # Not the path: videos queued from one class share its recording by hard link, and with it this transcript.
    key = hashlib.sha1(f"{st.st_size}|{int(st.st_mtime)}|{model}|{a.language}".encode()).hexdigest()
    meta_path = os.path.join(a.out, "meta.json")
    if os.path.exists(meta_path) and os.path.exists(os.path.join(a.out, "transcript.json")):
        meta = json.load(open(meta_path))
        if meta.get("key") == key:
            log("Already transcribed with these settings; reusing it.")
            print(json.dumps(meta["summary"]))
            return

    audio = load_audio(a.media)
    total = len(audio) / 16000
    log(f"Machine: {cores} CPU cores, {ram:.0f} GB memory, {'NVIDIA GPU' if cuda else 'no GPU'}. "
        f"Model: {model} on {device} ({compute}). Audio: {clock(total)}.")
    log("The first run downloads the model once (large-v3-turbo about 1.6 GB, large-v3 about 3 GB).")
    started = time.time()
    m = WhisperModel(model, device=device, compute_type=compute, cpu_threads=cores if device == "cpu" else 0)
    segments, info = m.transcribe(
        audio,
        language=a.language,
        beam_size=5,
        vad_filter=True,  # classes have long silences: skipping them saves time and stops made-up text
        word_timestamps=True,  # cut points for reels come from these
        condition_on_previous_text=False,  # keeps hours-long recordings from looping on one phrase
    )
    log(f"Language: {info.language} ({info.language_probability:.0%} sure). Transcribing…")

    out, last = [], 0.0
    transcribe_start = time.time()
    for seg in segments:
        out.append({
            "start": round(seg.start, 2), "end": round(seg.end, 2), "text": seg.text.replace("\ufffd", "").strip(),  # a Bangla letter split at a segment edge
            "words": [{"start": round(w.start, 2), "end": round(w.end, 2), "word": w.word, "p": round(w.probability, 3)} for w in (seg.words or [])],
        })
        now = time.time()
        if total and now - last > 15:
            last = now
            done = min(seg.end / total, 1)
            spent = now - transcribe_start
            eta = spent / done - spent if done > 0.01 else 0
            log(f"{done:.0%} · {clock(seg.end)} of {clock(total)} · about {clock(eta)} left")

    took = time.time() - started
    result = {"language": info.language, "duration": round(info.duration, 2), "model": model, "segments": out}
    with open(os.path.join(a.out, "transcript.json"), "w", encoding="utf-8") as f:
        json.dump(result, f, ensure_ascii=False, indent=1)
    with open(os.path.join(a.out, "transcript.srt"), "w", encoding="utf-8") as f:
        for i, s in enumerate(out, 1):
            f.write(f"{i}\n{srt_time(s['start'])} --> {srt_time(s['end'])}\n{s['text']}\n\n")
    with open(os.path.join(a.out, "transcript.txt"), "w", encoding="utf-8") as f:
        for s in out:
            f.write(f"[{clock(s['start'])}] {s['text']}\n")
    summary = {"ok": True, "language": info.language, "seconds": round(total, 1), "segments": len(out), "model": model,
               "took_seconds": round(took), "json": os.path.join(a.out, "transcript.json"),
               "srt": os.path.join(a.out, "transcript.srt"), "txt": os.path.join(a.out, "transcript.txt")}
    json.dump({"key": key, "summary": summary}, open(meta_path, "w"))
    log(f"Done in {clock(took)}.")
    print(json.dumps(summary))


if __name__ == "__main__":
    main()
