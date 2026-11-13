import { lazy as lazyPage, Suspense, useCallback, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  ArrowCircleUpIcon, BookOpenIcon, ChartLineUpIcon, ChartPieSliceIcon, ChatCircleDotsIcon, ChatsCircleIcon, FilmSlateIcon, FilmStripIcon, GearSixIcon, HeartIcon, IconContext, MegaphoneIcon, MonitorIcon, MoonIcon, SidebarSimpleIcon,
  SquaresFourIcon, SunIcon, TagIcon,
} from "@phosphor-icons/react";
import "./styles.css";
import { api, desktop, donateURL, go, makerURL, type Meta, openExternal, pref, repoURL, saveFile, setClaudePlan, setDesktop, setReveal, setDisplay, setHelp, setNudge, setToast, snoozeNudge, toast as notify, useApi } from "./api";
import { Mark, Modal } from "./ui";
// Each page loads when first opened, so the first screen doesn't wait for the project editor and its video player.
// After an update the old page's files are gone from the server, so a tab left open reloads once to get the new ones.
const reloaded = (v?: string) => { try { if (v === undefined) return sessionStorage.getItem("37studio:reloaded"); v ? sessionStorage.setItem("37studio:reloaded", v) : sessionStorage.removeItem("37studio:reloaded"); } catch {} return "1"; };
const pages: (() => Promise<unknown>)[] = [];
const lazy = (load: () => Promise<{ default: React.ComponentType<any> }>) => (pages.push(load), lazyPage(() => load().then((m) => { reloaded(""); return m; }, (e) => {
  if (reloaded()) throw e; // reloading already didn't help: show the error instead of looping
  reloaded("1");
  location.reload();
  return new Promise<never>(() => {});
})));
// Once the first screen is up, the other pages' code loads while the browser is idle, so opening one doesn't wait for it.
const whenIdle = (f: () => void) => ("requestIdleCallback" in window ? requestIdleCallback(f, { timeout: 4000 }) : setTimeout(f, 1500));
const prefetchPages = () => whenIdle(() => pages.forEach((load) => load().catch(() => {})));

/** A thin bar at the top while a page's code loads. It waits a moment first, so a quick load doesn't flicker. */
const PageLoading = () => <div className="page-loading" role="progressbar" aria-label="Loading"><span /></div>;
const Overview = lazy(() => import("./pages/Overview"));
const Projects = lazy(() => import("./pages/Projects"));
const ProjectPage = lazy(() => import("./pages/Project"));
const Clients = lazy(() => import("./pages/Clients"));
const ClientPage = lazy(() => import("./pages/Client"));
const Costs = lazy(() => import("./pages/Costs"));
const Campaigns = lazy(() => import("./pages/Campaigns"));
const CampaignPage = lazy(() => import("./pages/Campaigns").then((m) => ({ default: m.CampaignPage })));
const Settings = lazy(() => import("./pages/Settings"));
const Inbox = lazy(() => import("./pages/Inbox"));
const Performance = lazy(() => import("./pages/Performance"));
const Library = lazy(() => import("./pages/Library"));
const Assistant = lazy(() => import("./pages/Assistant"));
const Guide = lazy(() => import("./pages/Guide"));
const HelpPanel = lazy(() => import("./pages/HelpPanel"));

const nav = [
  { group: "Studio", items: [
    { path: "/", label: "Overview", icon: SquaresFourIcon },
    { path: "/assistant", label: "Assistant", icon: ChatCircleDotsIcon },
    { path: "/projects", label: "Projects", icon: FilmSlateIcon },
    { path: "/library", label: "Library", icon: FilmStripIcon },
    { path: "/campaigns", label: "Campaigns", icon: MegaphoneIcon },
    { path: "/clients", label: "Brands", icon: TagIcon },
    { path: "/costs", label: "Costs", icon: ChartPieSliceIcon },
  ] },
  { group: "Social", items: [
    { path: "/inbox", label: "Inbox", icon: ChatsCircleIcon },
    { path: "/performance", label: "Performance", icon: ChartLineUpIcon },
  ] },
  { group: "Help", items: [
    { path: "/guide", label: "Guide", icon: BookOpenIcon },
  ] },
];
const pageTitles = Object.fromEntries([...nav.flatMap((g) => g.items.map((i) => [i.path, i.label])), ["/settings", "Settings"]]);

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
  const [why, setWhy] = useState<string>();
  const [helpAt, setHelpAt] = useState<string>();
  useEffect(() => {
    let timer: number;
    setToast((msg, err) => { setT({ msg, err }); clearTimeout(timer); timer = window.setTimeout(() => setT(undefined), err ? 6000 : 3000); });
    setNudge(setWhy);
    setHelp(setHelpAt);
  }, []);
  useEffect(() => { if (section === "guide") setHelpAt(undefined); }, [section]); // the Guide page already shows it
  const closeHelp = useCallback(() => setHelpAt(undefined), []);
  const closeNudge = () => setWhy(undefined);

  const page = (() => {
    const n = Number(id);
    switch (section) {
      case "projects": return id ? <ProjectPage key={n} id={n} /> : <Projects />;
      case "clients": return id ? <ClientPage key={n} id={n} tab={sub} /> : <Clients />;
      case "campaigns": return id ? <CampaignPage key={n} id={n} /> : <Campaigns />;
      case "library": return <Library />;
      case "costs": return <Costs />;
      case "inbox": return <Inbox />;
      case "performance": return <Performance />;
      case "settings": return <Settings tab={id} />;
      case "assistant": return <Assistant />;
      case "guide": return <Guide section={id} />;
      default: return <Overview />;
    }
  })();
  const active = "/" + (section ?? "");
  useEffect(() => { document.title = pageTitles[active] ? `${pageTitles[active]} · 37Studio` : "37Studio"; }, [active]);

  const [collapsed, setCollapsed] = useState(pref("side") === "1");
  const [theme, setTheme] = useState<Theme>((pref("theme") as Theme) || "system");
  useEffect(() => { pref("side", collapsed ? "1" : "0"); }, [collapsed]);
  useEffect(() => { applyTheme(theme); pref("theme", theme); }, [theme]);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.target as HTMLElement).closest("input, textarea, select, [contenteditable]")) return;
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "b") {
        e.preventDefault();
        setCollapsed((c) => !c);
      } else if (e.key === "?") {
        // An open modal comes later in the page, so the last button is the one for what's on top.
        const button = [...document.querySelectorAll<HTMLElement>(".modal .help-btn, .page-head .help-btn, .editor-head .help-btn")].pop();
        if (button) button.click(); else go("/guide");
      }
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, []);
  const { data: upd, reload: recheck } = useApi<{ current?: string; newer?: boolean; latest?: string }>("/update");
  // The desktop app stays open for days, so look again now and then (the server asks GitHub at most every 6 hours).
  useEffect(() => {
    const t = setInterval(() => { if (!document.hidden) recheck(); }, 3 * 3600_000);
    return () => clearInterval(t);
  }, [recheck]);
  // Say it once per version when it's first seen, so nobody has to notice the sidebar line.
  useEffect(() => {
    if (!upd?.newer || !upd.latest || pref("update-seen") === upd.latest) return;
    pref("update-seen", upd.latest);
    notify(`37Studio ${upd.latest} is out. Settings → About says how to update.`);
  }, [upd?.newer, upd?.latest]);
  const ThemeIcon = themeIcon[theme];
  const nextTheme = themes[(themes.indexOf(theme) + 1) % themes.length];

  return (
    <IconContext.Provider value={{ weight: "regular" }}>
      <div className={`app ${collapsed ? "collapsed" : ""}`}>
        <aside className="sidebar">
          <div className="brand">
            <a href="#/" aria-label="37Studio home"><Mark size={26} /></a>
            <div className="wordmark">37<span>Studio</span></div>
            <button className="btn ghost sm icon side-toggle" onClick={() => setCollapsed(!collapsed)} title={`${collapsed ? "Expand" : "Collapse"} sidebar (Ctrl+B)`}
              aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"} aria-expanded={!collapsed}><SidebarSimpleIcon size={16} /></button>
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
            <a href={donateURL} target="_blank" rel="noreferrer" title={collapsed ? "Donate" : "37Studio is free; donations keep it going"}>
              <HeartIcon size={18} /> <span className="nav-label">Donate</span>
            </a>
            {upd?.newer && <a className="update-link" href="#/settings/about" title={`Version ${upd.latest} is available: see how to update`}>
              <ArrowCircleUpIcon size={18} weight="duotone" /> <span className="nav-label">Update to {upd.latest}</span>
            </a>}
            <a className="who" href={repoURL} target="_blank" rel="noreferrer">37Studio {upd?.current} · open source</a>
            <a className="who" href={makerURL} target="_blank" rel="noreferrer">Made by Shaitrish</a>
          </div>
        </aside>
        <main className="main"><Suspense fallback={<PageLoading />}>{page}</Suspense></main>
        {helpAt && <Suspense fallback={null}><HelpPanel section={helpAt} onSection={setHelpAt} onClose={closeHelp} /></Suspense>}
        {toast && <div className={`toast ${toast.err ? "err" : ""}`} role="status">{toast.msg}</div>}
        {why && (
          <Modal title="Nice work" onClose={closeNudge} footer={<>
            <button className="btn ghost" onClick={() => { snoozeNudge(365); closeNudge(); }}>I've already donated</button>
            <button className="btn" onClick={closeNudge}>Maybe later</button>
            <a className="btn primary" href={donateURL} target="_blank" rel="noreferrer" onClick={closeNudge}><HeartIcon size={16} /> Support 37Studio</a>
          </>}>
            <p>{why}</p>
            <p className="muted">37Studio is free and open source, with no ads, accounts or paid tiers. Donations are what keep us working on it: fixes, new features and more platforms. If it saved you time today, your support goes a long way.</p>
          </Modal>
        )}
      </div>
    </IconContext.Provider>
  );
}

// In the desktop app, new-tab links and downloads would do nothing in the window: hand them to the app instead.
// Capture phase, so it runs whatever the link's own handlers do.
document.addEventListener("click", (e) => {
  const a = desktop && !e.defaultPrevented ? (e.target as Element).closest?.("a[href]") as HTMLAnchorElement | null : null;
  if (!a) return;
  if (a.hasAttribute("download")) { e.preventDefault(); saveFile(a.href, a.download || undefined); }
  else if (a.target === "_blank") { e.preventDefault(); openExternal(a.href); }
}, true);

// Display currency must be known before the first money value renders; whether this is the desktop app, before the first click.
Promise.all([
  api<Record<string, string>>("/settings").then(setDisplay, () => {}),
  api<Meta>("/meta").then((m) => { setDesktop(!!m.env.desktop); setReveal(!!m.env.reveal); setClaudePlan(!!m.env.claude_plan); }, () => {}),
]).finally(() => { createRoot(document.getElementById("root")!).render(<App />); prefetchPages(); });
