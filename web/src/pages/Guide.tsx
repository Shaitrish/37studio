import { useEffect, useState } from "react";
import { ArrowLeftIcon, ArrowRightIcon, ArrowSquareOutIcon, BookOpenIcon } from "@phosphor-icons/react";
import { repoURL } from "../api";
import { sections } from "../guide";
import { Empty, Md } from "../ui";

// The page a section talks about, one click away. Only here: links like these would be dead on GitHub.
const pages: Record<string, [string, string]> = {
  "connect-ai-services": ["Open AI services", "#/settings/ai"],
  "add-a-brand": ["Open Brands", "#/clients"],
  "make-a-video": ["Open Projects", "#/projects"],
  "work-on-a-project": ["Open Projects", "#/projects"],
  "long-recordings": ["Open Projects", "#/projects"],
  "library-and-templates": ["Open Library", "#/library"],
  campaigns: ["Open Campaigns", "#/campaigns"],
  "connect-social-accounts": ["Open Social apps", "#/settings/social"],
  publish: ["Open Projects", "#/projects"],
  autopilot: ["Open Brands", "#/clients"],
  inbox: ["Open Inbox", "#/inbox"],
  assistant: ["Open Assistant", "#/assistant"],
  "use-from-other-ai-apps": ["Open Other AI apps", "#/settings/agents"],
  "costs-and-spending": ["Open Costs", "#/costs"],
  settings: ["Open Settings", "#/settings"],
  "back-up-and-update": ["Open Storage", "#/settings/storage"],
};

export default function Guide({ section }: { section?: string }) {
  const [q, setQ] = useState("");
  const words = q.toLowerCase().split(/\s+/).filter(Boolean);
  const shown = sections.filter((s) => words.every((w) => s.text.includes(w)));
  const cur = shown.find((s) => s.id === section) ?? shown[0];
  const at = cur ? sections.indexOf(cur) : -1;
  const prev = sections[at - 1], next = sections[at + 1];
  const page = cur && pages[cur.id];
  // On phones the section list is a sideways strip: keep the open section in view.
  useEffect(() => { document.querySelector(".guide-nav .on")?.scrollIntoView({ block: "nearest", inline: "center" }); }, [cur?.id]);

  return (
    <div className="page">
      <div className="page-head">
        <div className="spacer">
          <h1>Guide</h1>
          <p>How to set up 37Studio and use every part of it.</p>
        </div>
        <input className="input guide-search" type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search the guide" aria-label="Search the guide" />
      </div>

      <div className="settings">
        <nav className="settings-nav guide-nav" aria-label="Guide sections">
          {shown.map((s) => (
            <button key={s.id} className={s === cur ? "on" : ""} aria-current={s === cur ? "page" : undefined} onClick={() => { location.hash = `/guide/${s.id}`; }}>
              <span>{s.title}</span>
            </button>
          ))}
        </nav>

        <div className="settings-body">
          {cur ? (
            <article className="card card-b">
              <div className="row wrap guide-title">
                <h2 className="spacer">{cur.title}</h2>
                {page && <a className="btn sm" href={page[1]}><ArrowSquareOutIcon size={14} /> {page[0]}</a>}
              </div>
              <Md text={cur.body} className="msg-agent guide-doc" />
              {!words.length && (
                <div className="row guide-pager">
                  {prev && <a className="btn ghost sm" href={`#/guide/${prev.id}`}><ArrowLeftIcon size={14} /> {prev.title}</a>}
                  <span className="spacer" />
                  {next && <a className="btn ghost sm" href={`#/guide/${next.id}`}>{next.title} <ArrowRightIcon size={14} /></a>}
                </div>
              )}
            </article>
          ) : (
            <div className="card">
              <Empty icon={<BookOpenIcon size={28} />} title="Nothing matches">
                Try other words, or <a className="link" href={`${repoURL}/issues`} target="_blank" rel="noreferrer">ask on GitHub</a>.
              </Empty>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
