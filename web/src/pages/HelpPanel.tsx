import { useEffect, useRef } from "react";
import { ArrowSquareOutIcon, XIcon } from "@phosphor-icons/react";
import { sections } from "../guide";
import { Md } from "../ui";

// Help opens beside the page instead of going to the Guide: the desktop window has no back button, and leaving a form
// would lose what's typed. There's no backdrop, so the steps stay readable while you follow them on the page.
export default function HelpPanel({ section, onSection, onClose }: { section: string; onSection: (id: string) => void; onClose: () => void }) {
  const cur = sections.find((s) => s.id === section) ?? sections[0];
  const panel = useRef<HTMLElement>(null), body = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const before = document.activeElement as HTMLElement | null;
    panel.current?.focus();
    // Capture, so Esc closes this and not the modal or the running agent underneath.
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") { e.stopPropagation(); onClose(); } };
    addEventListener("keydown", onKey, true);
    return () => { removeEventListener("keydown", onKey, true); before?.focus?.(); };
  }, [onClose]);
  useEffect(() => { body.current?.scrollTo(0, 0); }, [cur.id]);

  return (
    <aside className="help-panel" ref={panel} tabIndex={-1} aria-label="Help">
      <div className="help-h">
        <div className="spacer">
          <div className="eyebrow">Help</div>
          <h2>{cur.title}</h2>
        </div>
        <button className="btn ghost sm icon" onClick={onClose} aria-label="Close help" title="Close (Esc)"><XIcon size={16} /></button>
      </div>
      <div className="help-b" ref={body} onClick={(e) => {
        const to = (e.target as Element).closest?.('a[href^="#/guide/"]')?.getAttribute("href");
        if (to) { e.preventDefault(); onSection(to.slice("#/guide/".length)); }
      }}>
        <Md text={cur.body} className="msg-agent guide-doc" />
      </div>
      <div className="help-f">
        <select className="input spacer" value={cur.id} onChange={(e) => onSection(e.target.value)} aria-label="Help topic">
          {sections.map((s) => <option key={s.id} value={s.id}>{s.title}</option>)}
        </select>
        <a className="btn sm" href={`#/guide/${cur.id}`} onClick={onClose}><ArrowSquareOutIcon size={14} /> Full guide</a>
      </div>
    </aside>
  );
}
