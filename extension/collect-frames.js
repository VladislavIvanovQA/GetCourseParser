// Injected with executeScript(allFrames: true) — no extension APIs here.
function gcCollectVideoUrlsFromFrame() {
  const urls = new Set();
  const re =
    /https?:\/\/[^\s"'<>]+?\/api\/playlist\/(?:master|media)\/[^\s"'<>]+/gi;

  const add = (u) => {
    if (
      /\/api\/playlist\/(?:master|media)\//i.test(u) ||
      (/gceuproxy\.com/i.test(u) && /playlist/i.test(u)) ||
      /\.m3u8(\?|$)/i.test(u)
    ) {
      urls.add(u);
    }
  };

  try {
    for (const e of performance.getEntriesByType("resource")) add(e.name);
  } catch (_) {}

  try {
    const html = document.documentElement.innerHTML;
    let m;
    while ((m = re.exec(html)) !== null) add(m[0]);
  } catch (_) {}

  for (const el of document.querySelectorAll("iframe[src], video[src], source[src]")) {
    const src = el.getAttribute("src");
    if (!src) continue;
    try {
      add(new URL(src, location.href).href);
    } catch (_) {}
  }

  return [...urls];
}
