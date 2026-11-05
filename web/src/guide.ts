import raw from "../../docs/guide.md?raw";

// docs/guide.md is the one copy of this text, so GitHub and the app can't disagree. Each ## is a section, and its id is
// the anchor GitHub gives that heading, so the file's own [links](#section) work in both places.
export const sections = raw.split(/^## /m).slice(1).map((part) => {
  const title = part.slice(0, part.indexOf("\n")).trim();
  const body = part.slice(part.indexOf("\n")).trim()
    .replace(/^### /gm, "## ") // Md starts at h2; the section title already is one
    .replace(/\]\(#([\w-]+)\)/g, "](#/guide/$1)");
  return { title, body, id: title.toLowerCase().replace(/[^\w\s-]/g, "").replace(/\s+/g, "-"), text: `${title}\n${body}`.toLowerCase() };
});
