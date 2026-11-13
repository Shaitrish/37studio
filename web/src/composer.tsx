// The message box under the editor agent's and the Assistant's chat, laid out like Claude Code's: what you type on
// top, with the microphone in its corner; below it + to attach, / for quick actions, how many subagents are
// working, the model and its effort, the files that go with the message, the turn's running time, and Send.
import { ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { ArrowUpIcon, FileIcon, MicrophoneIcon, PlusIcon, SpinnerGapIcon, SquareIcon, XIcon } from "@phosphor-icons/react";
import { clock, parseTime, type Quality } from "./api";
import { liveStatus, modelName, Parsed, useDictation, usePromptHistory } from "./chat";
import { ModelOptions } from "./ui";

const effortName: Record<string, string> = { low: "Low", medium: "Medium", high: "High", xhigh: "Max" };
/** "Opus 5.5 · High", the way Claude Code names a model and its effort. */
export const modelLabel = (q: Quality) => (q.harness ? q.label : `${modelName(q.model)}${effortName[q.effort] ? ` · ${effortName[q.effort]}` : ""}`);

export interface Action { label: string; send?: boolean } // send: goes out at once; otherwise it fills the box
/** Something on screen that rides along with the message as one line the agent understands, e.g. [viewing: out/a.mp4]
 *  or [paused at 0:12.4 in the live preview; …]. */
export interface Context { label: string; title?: string; line: string; icon?: ReactNode }

export function Composer({ events, running, queue, onSend, onStop, qualities, quality, onQuality, placeholder, actions = [], attach, fileUrl, context }: {
  events: Parsed[]; running: boolean;
  queue?: boolean; // while the agent works, a message waits for the turn to end (the editor agent) instead of being refused
  onSend: (text: string, quality: string) => Promise<unknown> | void; onStop: () => void;
  qualities: Quality[]; quality: string; onQuality: (id: string) => void;
  placeholder: string; actions?: Action[];
  attach?: (files: File[]) => Promise<string[]>; // uploads images and returns their paths, sent as [image: …] lines
  fileUrl?: (path: string) => string;
  context?: Context[];
}) {
  const [text, setText] = useState("");
  const [files, setFiles] = useState<string[]>([]);
  const [hidden, setHidden] = useState<string[]>([]); // chips you closed, until something else is on screen
  const ctx = (context ?? []).filter((c) => !hidden.includes(c.line));
  const box = useRef<HTMLTextAreaElement>(null);
  const mic = useDictation((said) => setText((t) => (t ? t.trimEnd() + " " : "") + said));
  const history = usePromptHistory(events, setText);
  const agents = useMemo(() => (running ? liveStatus(events).agents : 0), [events, running]);

  // "/" opens the quick actions, as in Claude Code; typing after it filters them.
  const [menu, setMenu] = useState(false);
  const [pick, setPick] = useState(0);
  const filter = text.startsWith("/") ? text.slice(1).toLowerCase() : "";
  const shown = actions.filter((a) => a.label.toLowerCase().includes(filter));
  useEffect(() => { setMenu(text.startsWith("/") && !text.includes("\n") && actions.length > 0); setPick(0); }, [text]);

  // How long this turn has been going, from the last message you sent.
  const since = useMemo(() => {
    if (!running) return 0;
    const u = [...events].reverse().find((e) => e.kind === "user");
    return u ? parseTime(u.at).getTime() : Date.now();
  }, [events, running]);
  const [, tick] = useState(0);
  useEffect(() => {
    if (!running) return;
    const t = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(t);
  }, [running]);

  const blocked = running && !queue;
  const send = async (t = text) => {
    if ((!t.trim() && !files.length) || blocked) return;
    mic.stop();
    const lines = [t.trim(), ...ctx.map((c) => c.line), ...files.map((f) => `[image: ${f}]`)].filter(Boolean);
    const kept = { t, files };
    setText(""); setFiles([]); setMenu(false);
    try { await onSend(lines.join("\n"), quality); } catch { setText(kept.t); setFiles(kept.files); }
  };
  const choose = (a: Action) => {
    setMenu(false);
    if (a.send) send(a.label); else { setText(a.label); box.current?.focus(); }
  };
  const add = async (list: FileList | File[]) => {
    if (!attach) return;
    const imgs = [...list].filter((f) => f.type.startsWith("image/"));
    if (imgs.length) setFiles([...files, ...(await attach(imgs).catch(() => []))]);
  };

  const q = qualities.find((x) => x.id === quality);
  return (
    <div className="composer">
      <div className={`composer-box ${mic.listening ? "listening" : ""}`}
        onDragOver={(e) => attach && e.preventDefault()} onDrop={(e) => { if (!attach) return; e.preventDefault(); add(e.dataTransfer.files); }}>
        {menu && shown.length > 0 && (
          <div className="slash-menu" role="listbox" aria-label="Quick actions">
            {shown.map((a, i) => (
              <button key={a.label} role="option" aria-selected={i === pick} className={i === pick ? "on" : ""}
                onMouseEnter={() => setPick(i)} onMouseDown={(e) => { e.preventDefault(); choose(a); }}>
                <span className="slash">/</span>{a.label}
              </button>
            ))}
          </div>
        )}
        <div className="composer-top">
          <textarea ref={box} rows={1}
            value={mic.interim ? text + (text ? " " : "") + mic.interim : text} readOnly={!!mic.interim}
            placeholder={mic.busy ? "Writing down what you said…" : mic.listening ? "Listening… click the microphone to stop" : placeholder}
            aria-label={placeholder}
            onChange={(e) => setText(e.target.value)}
            onPaste={(e) => { if (attach && e.clipboardData.files.length) { e.preventDefault(); add(e.clipboardData.files); } }}
            onKeyDown={(e) => {
              if (menu && shown.length) {
                if (e.key === "ArrowDown" || e.key === "ArrowUp") { e.preventDefault(); setPick((pick + (e.key === "ArrowDown" ? 1 : shown.length - 1)) % shown.length); return; }
                if (e.key === "Enter" || e.key === "Tab") { e.preventDefault(); choose(shown[pick]); return; }
                if (e.key === "Escape") { e.preventDefault(); setMenu(false); return; }
              }
              if (!mic.listening) history(e);
              if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); send(); }
              if (e.key === "Escape" && running) onStop(); // like Claude Code: Esc interrupts
            }} />
          {mic.supported && (
            <div className="mic">
              <button className={`cbtn ${mic.listening ? "rec" : ""}`} onClick={mic.listening ? mic.stop : mic.start} disabled={mic.busy}
                title={mic.busy ? "Writing down what you said" : mic.listening ? "Stop dictation" : "Dictate (English)"} aria-label={mic.listening ? "Stop dictation" : "Dictate"}>
                {mic.busy ? <SpinnerGapIcon size={16} className="spin" /> : mic.listening ? <SquareIcon size={13} weight="fill" /> : <MicrophoneIcon size={16} />}
              </button>
            </div>
          )}
        </div>
        <div className="composer-bar">
          {attach && (
            <label className="cbtn" title="Attach screenshots (or paste or drop them)" aria-label="Attach images">
              <PlusIcon size={16} />
              <input type="file" accept="image/*" multiple hidden onChange={(e) => { add(e.target.files!); e.target.value = ""; }} />
            </label>
          )}
          {actions.length > 0 && (
            <button className="cbtn slash-btn" onClick={() => { setText("/"); box.current?.focus(); }} title="Quick actions (type /)" aria-label="Quick actions">/</button>
          )}
          <span className={`cpill ${agents ? "busy" : ""}`} title="Subagents working for the agent right now">
            <i className="dot" />{agents} agent{agents === 1 ? "" : "s"}
          </span>
          <label className="cpill model" title={q?.note}>
            <span className="ellipsis">{q ? modelLabel(q) : "Model"}</span>
            <select value={quality} onChange={(e) => onQuality(e.target.value)} aria-label="Model for this message">
              <ModelOptions qualities={qualities} claudeLabel={modelLabel} />
            </select>
          </label>
          {(ctx.length > 0 || files.length > 0) && <span className="csep" />}
          <div className="cfiles">
            {ctx.map((c) => (
              <span key={c.line} className="cfile" title={c.title ?? "Sent with your message, so the agent knows what you're looking at"}>
                {c.icon ?? <FileIcon size={14} />}<span className="ellipsis">{c.label}</span>
                <button onClick={() => setHidden([...hidden, c.line])} aria-label={`Don't send ${c.label}`}><XIcon size={12} /></button>
              </span>
            ))}
            {files.map((f) => (
              <span key={f} className="cfile" title={f}>
                {fileUrl ? <img src={fileUrl(f)} alt="" /> : <FileIcon size={14} />}<span className="ellipsis">{f.replace(/^.*\//, "")}</span>
                <button onClick={() => setFiles(files.filter((x) => x !== f))} aria-label="Remove image"><XIcon size={12} /></button>
              </span>
            ))}
          </div>
          <span className="spacer" />
          {running && <span className="ctime" title="How long the agent has been on this">{clock((Date.now() - since) / 1000)}</span>}
          {running && <button className="cbtn stop" onClick={onStop} title="Stop (Esc)" aria-label="Stop the agent"><SquareIcon size={13} weight="fill" /></button>}
          <button className="cbtn send" disabled={(!text.trim() && !files.length) || blocked} onClick={() => send()}
            title={blocked ? "Wait for the reply, or stop it" : running ? "Queue: sends when this turn ends" : "Send (Enter)"} aria-label={running ? "Queue message" : "Send"}>
            <ArrowUpIcon size={16} />
          </button>
        </div>
      </div>
    </div>
  );
}
