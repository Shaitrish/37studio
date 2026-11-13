"""37Studio's dictation worker: keeps faster-whisper loaded and turns short English voice clips into text.

Reads requests on stdin, each a JSON line {"n": <bytes>, "final": true|false} followed by n bytes of 16 kHz mono
16-bit little-endian PCM, and answers each with one JSON line {"text": ...} or {"error": ...}. Prints {"ready": true}
once the models are loaded. Drafts (final false) use tiny.en, fast enough to show the words while you speak; final
phrases use small.en, which gets names and accents right far more often.
"""
import json
import sys

import numpy as np


def say(obj):
    sys.stdout.write(json.dumps(obj, ensure_ascii=False) + "\n")
    sys.stdout.flush()


def main():
    try:
        import ctranslate2
        from faster_whisper import WhisperModel

        cuda = ctranslate2.get_cuda_device_count() > 0
        device, compute = ("cuda", "float16") if cuda else ("cpu", "int8")
        # The first run downloads both models once (about 75 MB and 480 MB).
        fast = WhisperModel("tiny.en", device=device, compute_type=compute)
        good = WhisperModel("small.en", device=device, compute_type=compute)
    except Exception as e:  # noqa: BLE001 - whatever it is, the app shows it
        say({"error": f"couldn't load the speech model: {e}"})
        return 1
    say({"ready": True})

    inp = sys.stdin.buffer
    while True:
        line = inp.readline()
        if not line:
            return 0
        try:
            req = json.loads(line)
            data = inp.read(int(req["n"]))
            audio = np.frombuffer(data, dtype="<i2").astype(np.float32) / 32768.0
            final = bool(req.get("final"))
            # Greedy decoding and no timestamps: a voice note needs speed, not word timings. VAD on the final pass
            # drops silence, where Whisper otherwise invents a "Thank you."
            segs, _ = (good if final else fast).transcribe(
                audio, language="en", beam_size=1, without_timestamps=True,
                condition_on_previous_text=False, vad_filter=final)
            say({"text": " ".join(s.text.strip() for s in segs).strip()})
        except Exception as e:  # noqa: BLE001
            say({"error": str(e)})


if __name__ == "__main__":
    sys.exit(main())
