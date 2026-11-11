// A video or photo from your computer, posted to the brand's accounts: no agent, just the file, AI captions and Publish.
// It is a small project of kind "post", so posting, scheduling, the Posts tab and the numbers all work as for videos.
import { useEffect, useState } from "react";
import { ImageIcon, PaperPlaneTiltIcon, VideoIcon } from "@phosphor-icons/react";
import { api, bytes, fail, go, Project, upload, useTenants } from "../api";
import { Avatar, DropZone, Field, Modal, Progress } from "../ui";

export function NewPostModal({ onClose, tenantId }: { onClose: () => void; tenantId?: number }) {
  const tenants = useTenants();
  const [tenant, setTenant] = useState<number | undefined>(tenantId);
  const [file, setFile] = useState<File>();
  const [note, setNote] = useState("");
  const [pct, setPct] = useState<number>();
  const [preview, setPreview] = useState<string>();
  useEffect(() => { if (!tenant && tenants?.length === 1) setTenant(tenants[0].id); }, [tenants]);
  useEffect(() => {
    if (!file) return;
    const u = URL.createObjectURL(file);
    setPreview(u);
    return () => URL.revokeObjectURL(u);
  }, [file]);

  const pickFile = (fs: File[]) => {
    const f = fs.find((x) => x.type.startsWith("video/") || x.type.startsWith("image/"));
    if (f) setFile(f); else if (fs.length) fail(new Error("Pick a video or a photo (MP4, MOV, JPG, PNG or WebP)."));
  };
  const create = async () => {
    if (!tenant || !file) return;
    setPct(0);
    try {
      const title = note.split("\n")[0].trim().slice(0, 80) || file.name.replace(/\.[^.]+$/, "");
      const p = await api<Project>("/projects", { body: { tenant_id: tenant, title, brief: note.trim(), kind: "post" } });
      await upload(`/projects/${p.id}/outputs`, [file], setPct);
      onClose();
      go(`/projects/${p.id}`);
    } catch (e) { fail(e); setPct(undefined); }
  };

  const video = file?.type.startsWith("video/");
  return (
    <Modal title="New post" sub="Post a video or photo from your computer to the brand's accounts, with captions written for each platform."
      help="post-from-your-computer" onClose={pct !== undefined ? () => {} : onClose}
      footer={<>
        <button className="btn" onClick={onClose} disabled={pct !== undefined}>Cancel</button>
        <button className="btn primary" disabled={!tenant || !file || pct !== undefined} onClick={create}>
          <PaperPlaneTiltIcon size={16} /> {pct !== undefined ? "Uploading…" : "Next: captions and accounts"}
        </button>
      </>}>
      <div className="form">
        {!tenantId && (tenants?.length ?? 0) > 1 && (
          <div className="field full">
            <span>Brand</span>
            <div className="chips" style={{ flexWrap: "wrap" }}>
              {tenants!.map((t) => (
                <button key={t.id} className={`chip ${tenant === t.id ? "on" : ""}`} onClick={() => setTenant(t.id)}>
                  <Avatar tenant={t} size="xs" /> {t.name}
                </button>
              ))}
            </div>
          </div>
        )}
        {/* a div, not Field's <label>: a label forwards clicks to the drop zone's file input and opens it twice */}
        <div className="field full">
          <span>Video or photo</span>
          {file && preview ? (
            <div className="post-pick">
              {video ? <video src={preview} controls playsInline /> : <img src={preview} alt="" />}
              <div className="small">
                {video ? <VideoIcon size={14} /> : <ImageIcon size={14} />} <b>{file.name}</b> · {bytes(file.size)}
                <button className="btn ghost sm" onClick={() => setFile(undefined)} disabled={pct !== undefined}>Change</button>
              </div>
            </div>
          ) : (
            <DropZone accept="video/*,image/*" onFiles={pickFile} label="Drop a video or photo here, or click to choose one" />
          )}
        </div>
        <Field full label="What's it about?" hint="Optional. A line or two helps the AI write better captions; it also looks at the file itself.">
          <textarea className="textarea" rows={3} value={note} onChange={(e) => setNote(e.target.value)}
            placeholder="New batch for the BCS exam starts 1 December. Seats are limited." />
        </Field>
        {!video && file && (
          <p className="small faint" style={{ margin: 0, gridColumn: "1 / -1" }}>Photos post to Facebook Pages, X, LinkedIn, Pinterest, Bluesky and Mastodon. Instagram and TikTok only take photos from a public web address, and YouTube takes no photos, so those accounts are left out.</p>
        )}
        {pct !== undefined && <Progress pct={pct} />}
      </div>
    </Modal>
  );
}
