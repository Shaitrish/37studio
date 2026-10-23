import { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  ChartPieSliceIcon, ChatsCircleIcon, FilmSlateIcon, GearSixIcon, IconContext, MegaphoneIcon, MonitorIcon, MoonIcon, SidebarSimpleIcon,
  SquaresFourIcon, SunIcon, UsersThreeIcon,
} from "@phosphor-icons/react";
import "@hyperframes/player";
import "./styles.css";
import { api, fail, License, licenseURL, setDisplay, setToast, toast, useApi } from "./api";
import { Mark, Modal } from "./ui";
import Overview from "./pages/Overview";
import Projects from "./pages/Projects";
import ProjectPage from "./pages/Project";
import Clients from "./pages/Clients";
import ClientPage from "./pages/Client";
import Costs from "./pages/Costs";
import Campaigns, { CampaignPage } from "./pages/Campaigns";
import Settings from "./pages/Settings";
import Inbox from "./pages/Inbox";

const nav = [
  { group: "Studio", items: [
    { path: "/", label: "Overview", icon: SquaresFourIcon },
    { path: "/projects", label: "Projects", icon: FilmSlateIcon },
    { path: "/campaigns", label: "Campaigns", icon: MegaphoneIcon },
    { path: "/clients", label: "Companies", icon: UsersThreeIcon },
  ] },
  { group: "Social", items: [
    { path: "/inbox", label: "Inbox", icon: ChatsCircleIcon },
    { path: "/costs", label: "Costs", icon: ChartPieSliceIcon },
  ] },
];

// Per-browser UI preferences. Storage can throw (private mode); the defaults still work.
const pref = (k: string, v?: string) => { try { if (v === undefined) return localStorage.getItem("37studio:" + k); localStorage.setItem("37studio:" + k, v); } catch {} return v ?? null; };
const themes = ["system", "light", "dark"] as const;
type Theme = (typeof themes)[number];
const themeIcon = { system: MonitorIcon, light: SunIcon, dark: MoonIcon };
const applyTheme = (t: Theme) => { if (t === "system") delete document.documentElement.dataset.theme; else document.documentElement.dataset.theme = t; };

function useHash() {
  const read = () => location.hash.slice(1) || "/";
  const [value, setHash] = useState(read);
  const hash = useRef(value);
  useEffect(() => {
    const page = (h: string) => h.split("/").slice(0, 3).join("/"); // tab changes keep the scroll position
    const on = () => {
      if (page(read()) !== page(hash.current)) window.scrollTo(0, 0);
      hash.current = read();
      setHash(read());
    };
    addEventListener("hashchange", on);
    return () => removeEventListener("hashchange", on);
  }, []);
  return value;
}

/** Opens when the server answers 402: the free trial is used up, or a feature needs a license. */
function LicenseDialog({ message, onClose, onActivated }: { message: string; onClose: () => void; onActivated: () => void }) {
  const [key, setKey] = useState("");
  const activate = () => api<License>("/license", { method: "PUT", body: { key } })
    .then((l) => { toast(`Activated for ${l.name}. Thank you!`); onActivated(); }, fail);
  return (
    <Modal title="Activate 37Studio" sub={message.split(" Add your key")[0]} onClose={onClose}
      footer={<><button className="btn" onClick={onClose}>Not now</button>
        <a className="btn primary" href={licenseURL} target="_blank" rel="noreferrer">Get a license</a></>}>
      <div className="stack">
        <p className="muted" style={{ margin: 0 }}>A one-time license unlocks unlimited videos, Autopilot, publishing and inbox auto-replies, with every future update included.</p>
        <div className="row wrap">
          <input className="input mono spacer" value={key} onChange={(e) => setKey(e.target.value)} placeholder="Already have a key? Paste it here (37S1.…)" aria-label="License key" style={{ minWidth: 240 }} />
          <button className="btn" disabled={!key.trim()} onClick={activate}>Activate</button>
        </div>
      </div>
    </Modal>
  );
}

function App() {
  const hash = useHash();
  const [, section, id, sub] = hash.split("/");
  const [toast, setT] = useState<{ msg: string; err?: boolean }>();
  useEffect(() => {
    let timer: number;
    setToast((msg, err) => { setT({ msg, err }); clearTimeout(timer); timer = window.setTimeout(() => setT(undefined), err ? 6000 : 3000); });
  }, []);

  const page = (() => {
    const n = Number(id);
    switch (section) {
      case "projects": return id ? <ProjectPage key={n} id={n} /> : <Projects />;
      case "clients": return id ? <ClientPage key={n} id={n} tab={sub} /> : <Clients />;
      case "campaigns": return id ? <CampaignPage key={n} id={n} /> : <Campaigns />;
      case "costs": return <Costs />;
      case "inbox": return <Inbox />;
      case "settings": return <Settings />;
      default: return <Overview />;
    }
  })();
  const active = "/" + (section ?? "");

  const [collapsed, setCollapsed] = useState(pref("side") === "1");
  const [theme, setTheme] = useState<Theme>((pref("theme") as Theme) || "system");
  useEffect(() => { pref("side", collapsed ? "1" : "0"); }, [collapsed]);
  useEffect(() => { applyTheme(theme); pref("theme", theme); }, [theme]);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "b" && !(e.target as HTMLElement).closest("input, textarea, [contenteditable]")) {
        e.preventDefault();
        setCollapsed((c) => !c);
      }
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, []);
  const { data: lic, reload: reloadLic } = useApi<License>("/license");
  const [needLicense, setNeedLicense] = useState<string>();
  useEffect(() => {
    const on = (e: Event) => setNeedLicense((e as CustomEvent<string>).detail || "This needs a 37Studio license.");
    addEventListener("license-required", on);
    return () => removeEventListener("license-required", on);
  }, []);
  const { data: upd } = useApi<{ newer?: boolean; latest?: string }>("/update");
  const ThemeIcon = themeIcon[theme];
  const nextTheme = themes[(themes.indexOf(theme) + 1) % themes.length];

  return (
    <IconContext.Provider value={{ weight: "regular" }}>
      <div className={`app ${collapsed ? "collapsed" : ""}`}>
        <aside className="sidebar">
          <div className="brand">
            <a href="#/" aria-label="37Studio home"><Mark size={26} /></a>
            <div className="wordmark">37<span>Studio</span></div>
            <button className="btn ghost sm icon side-toggle" onClick={() => setCollapsed(true)} title="Collapse sidebar (Ctrl+B)" aria-label="Collapse sidebar"><SidebarSimpleIcon size={16} /></button>
          </div>
          {nav.map(({ group, items }) => (
            <div key={group}>
              <div className="nav-group eyebrow">{group}</div>
              <nav className="nav">
                {items.map(({ path, label, icon: Icon }) => (
                  <a key={path} href={"#" + path} title={collapsed ? label : undefined} className={active === path ? "active" : ""}>
                    <Icon size={18} weight={active === path ? "duotone" : "regular"} /> <span className="nav-label">{label}</span>
                  </a>
                ))}
              </nav>
            </div>
          ))}
          <div className="side-foot nav">
            <a href="#/settings" title={collapsed ? "Settings" : undefined} className={active === "/settings" ? "active" : ""}>
              <GearSixIcon size={18} weight={active === "/settings" ? "duotone" : "regular"} /> <span className="nav-label">Settings</span>
            </a>
            <a role="button" tabIndex={0} title={`Theme: ${theme}. Switch to ${nextTheme}`} onClick={() => setTheme(nextTheme)} onKeyDown={(e) => e.key === "Enter" && setTheme(nextTheme)}>
              <ThemeIcon size={18} /> <span className="nav-label">{theme[0].toUpperCase() + theme.slice(1)} theme</span>
            </a>
            <a role="button" tabIndex={0} className="expand" title="Expand sidebar (Ctrl+B)" onClick={() => setCollapsed(false)} onKeyDown={(e) => e.key === "Enter" && setCollapsed(false)}>
              <SidebarSimpleIcon size={18} /> <span className="nav-label">Expand</span>
            </a>
            {upd?.newer && <a className="who" href="#/settings" style={{ color: "var(--primary)" }}>Version {upd.latest} available</a>}
            <a className="who" href="#/settings">{lic?.valid ? <><span className="dot" />{lic.name}</> : `Free trial · ${lic?.trial_left ?? 0} of ${lic?.trial ?? 2} videos left`}</a>
          </div>
        </aside>
        <main className="main">{page}</main>
        {toast && <div className={`toast ${toast.err ? "err" : ""}`} role="status">{toast.msg}</div>}
        {needLicense && <LicenseDialog message={needLicense} onClose={() => setNeedLicense(undefined)} onActivated={() => { setNeedLicense(undefined); reloadLic(); }} />}
      </div>
    </IconContext.Provider>
  );
}

// Display currency must be known before the first money value renders.
api<Record<string, string>>("/settings").then(setDisplay, () => {}).finally(() => createRoot(document.getElementById("root")!).render(<App />));
