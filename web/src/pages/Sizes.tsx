// Sizes: one video in the shape each platform shows best, like a 9:16 reel for TikTok, Instagram and Shorts and a 16:9
// video for Facebook and YouTube. A quick copy is made with ffmpeg (agent.ResizeVideo); for a video the agent built,
// asking the agent to lay it out again looks better, because titles and logos move instead of being cropped or shrunk.
import { useState } from "react";
import { ArrowsOutIcon, CropIcon, SparkleIcon } from "@phosphor-icons/react";
import { api, fail, FileInfo, Project, toast } from "../api";
import { Modal } from "../ui";

export const shapes = [
  { aspect: "9:16", label: "Vertical 9:16", sub: "Reels, TikTok, YouTube Shorts" },
  { aspect: "16:9", label: "Wide 16:9", sub: "Facebook, YouTube, LinkedIn, X" },
  { aspect: "4:5", label: "Portrait 4:5", sub: "Instagram and Facebook feed" },
  { aspect: "1:1", label: "Square 1:1", sub: "feeds everywhere" },
];

/** A file's shape: copies are named promo-9x16.mp4; the original has the project's shape.
 * ponytail: by name, not by probing the file, so a renamed copy reads as the project's shape. */
export const shapeOf = (f: FileInfo, p: Project) => f.name.match(/-(9x16|16x9|1x1|4x5)\.[^.]+$/)?.[1].replace("x", ":") ?? p.aspect;

/** Which shape each platform should get, best first: vertical where people scroll reels, wide where they watch. */
export const preferredShapes: Record<string, string[]> = {
  tiktok: ["9:16"], instagram: ["9:16", "4:5"], pinterest: ["9:16", "4:5"],
  youtube: ["16:9", "9:16"], facebook: ["16:9", "4:5", "1:1"],
  linkedin: ["16:9", "1:1"], x: ["16:9", "1:1"], bluesky: ["16:9", "1:1"], mastodon: ["16:9", "1:1"],
};

export function SizesModal({ p, file, onClose, onStarted, onAskAgent }: {
  p: Project; file: FileInfo; onClose: () => void; onStarted: () => void; onAskAgent?: (text: string) => void;
}) {
  const from = shapeOf(file, p);
  const options = shapes.filter((s) => s.aspect !== from);
  const [aspect, setAspect] = useState(options[0].aspect);
  const [mode, setMode] = useState<"fit" | "crop" | "agent">("fit");
  const [busy, setBusy] = useState(false);
  const target = shapes.find((s) => s.aspect === aspect)!;

  const go = async () => {
    if (mode === "agent") {
      onAskAgent?.(`Make a ${aspect} version of this video for ${target.sub}. Lay it out again for that shape, so titles, captions and the logo sit right in it instead of being cropped or shrunk, keep everything else the same, and render it as out/${file.name.replace(/(-(9x16|16x9|1x1|4x5))?\.[^.]+$/, "")}-${aspect.replace(":", "x")}.mp4 next to the current one, without replacing it.`);
      onClose();
      return;
    }
    setBusy(true);
    try {
      await api(`/projects/${p.id}/resize`, { body: { file: "out/" + file.path.replace(/^out\//, ""), aspect, mode } });
      toast(`Making the ${aspect} copy; it shows up in Versions when it's done`);
      onStarted();
      onClose();
    } catch (e) { fail(e); setBusy(false); }
  };

  return (
    <Modal title="Make another size" sub={`${file.name} is ${from}. Make a copy in the shape another platform shows best.`} help="make-other-sizes" onClose={onClose}
      footer={<>
        <button className="btn" onClick={onClose}>Cancel</button>
        <button className="btn primary" disabled={busy} onClick={go}>{mode === "agent" ? <><SparkleIcon size={15} /> Ask the agent</> : busy ? "Starting…" : `Make the ${aspect} copy`}</button>
      </>}>
      <div className="stack" style={{ gap: 16 }}>
        <div className="choices">
          {options.map((s) => (
            <button key={s.aspect} type="button" className={`choice ${aspect === s.aspect ? "on" : ""}`} aria-pressed={aspect === s.aspect} onClick={() => setAspect(s.aspect)}>
              <span className="stack" style={{ gap: 2, textAlign: "left" }}><b>{s.label}</b><span className="small muted">{s.sub}</span></span>
            </button>
          ))}
        </div>
        <div className="choices">
          <button type="button" className={`choice ${mode === "fit" ? "on" : ""}`} onClick={() => setMode("fit")}>
            <ArrowsOutIcon size={18} /><span className="stack" style={{ gap: 2, textAlign: "left" }}><b>Fit</b><span className="small muted">The whole picture, over a blurred copy of itself. Nothing is cut off.</span></span>
          </button>
          <button type="button" className={`choice ${mode === "crop" ? "on" : ""}`} onClick={() => setMode("crop")}>
            <CropIcon size={18} /><span className="stack" style={{ gap: 2, textAlign: "left" }}><b>Crop</b><span className="small muted">Fills the frame from the middle and cuts the edges. Check nothing important is lost.</span></span>
          </button>
          {onAskAgent && (
            <button type="button" className={`choice ${mode === "agent" ? "on" : ""}`} onClick={() => setMode("agent")}>
              <SparkleIcon size={18} /><span className="stack" style={{ gap: 2, textAlign: "left" }}><b>Ask the agent</b><span className="small muted">It lays the video out again for the new shape. Looks best; costs an agent turn.</span></span>
            </button>
          )}
        </div>
        {mode !== "agent" && <p className="small faint" style={{ margin: 0 }}>Quick and free: made on this computer in about the video's length. It lands in Versions as a new final, and Publish sends each platform the shape it shows best.</p>}
      </div>
    </Modal>
  );
}
