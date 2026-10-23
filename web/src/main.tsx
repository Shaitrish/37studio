import { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  ChartPieSliceIcon, ChatsCircleIcon, FilmSlateIcon, FilmStripIcon, GearSixIcon, IconContext, MegaphoneIcon, MonitorIcon, MoonIcon, SidebarSimpleIcon,
  SquaresFourIcon, SunIcon, UsersThreeIcon,
} from "@phosphor-icons/react";
import "@hyperframes/player";
import "./styles.css";
import { api, repoURL, setDisplay, setToast, useApi } from "./api";
import { Mark } from "./ui";
import Overview from "./pages/Overview";
import Projects from "./pages/Projects";
import ProjectPage from "./pages/Project";
import Clients from "./pages/Clients";
import ClientPage from "./pages/Client";
import Costs from "./pages/Costs";
import Campaigns, { CampaignPage } from "./pages/Campaigns";
import Settings from "./pages/Settings";
import Inbox from "./pages/Inbox";
import Library from "./pages/Library";

const nav = [
  { group: "Studio", items: [
    { path: "/", label: "Overview", icon: SquaresFourIcon },
    { path: "/projects", label: "Projects", icon: FilmSlateIcon },
    { path: "/library", label: "Library", icon: FilmStripIcon },
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
      case "library": return <Library />;
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
  const { data: upd } = useApi<{ current?: string; newer?: boolean; latest?: string }>("/update");
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
            <a className="who" href={repoURL} target="_blank" rel="noreferrer">37Studio {upd?.current} · open source</a>
          </div>
        </aside>
        <main className="main">{page}</main>
        {toast && <div className={`toast ${toast.err ? "err" : ""}`} role="status">{toast.msg}</div>}
      </div>
    </IconContext.Provider>
  );
}

// Display currency must be known before the first money value renders.
api<Record<string, string>>("/settings").then(setDisplay, () => {}).finally(() => createRoot(document.getElementById("root")!).render(<App />));
