(function () {
  const URL_RE =
    /https?:\/\/[^\s"'<>]+?\/api\/playlist\/(?:master|media)\/[^\s"'<>]+/gi;

  let pageKey = location.href;

  function notifyLessonChange() {
    if (location.href === pageKey) return;
    pageKey = location.href;
    chrome.runtime.sendMessage({ type: "lessonChanged", url: pageKey }).catch(() => {});
  }

  function collectUrls() {
    const urls = new Set();
    const add = (u) => {
      if (typeof globalThis.gcM3U8?.shouldCaptureVideoRequest === "function") {
        if (globalThis.gcM3U8.shouldCaptureVideoRequest(u)) urls.add(u);
        return;
      }
      if (
        /\/api\/playlist\/(?:master|media)\//i.test(u) ||
        (/gceuproxy\.com/i.test(u) && /playlist/i.test(u))
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
      while ((m = URL_RE.exec(html)) !== null) add(m[0]);
    } catch (_) {}

    return [...urls];
  }

  function report() {
    notifyLessonChange();
    const urls = collectUrls();
    if (!urls.length) return;
    chrome.runtime.sendMessage({ type: "reportUrls", urls }).catch(() => {});
  }

  report();

  const obs = new PerformanceObserver((list) => {
    notifyLessonChange();
    for (const e of list.getEntries()) {
      if (e.name && /playlist|gceuproxy|\.m3u8/i.test(e.name)) {
        chrome.runtime.sendMessage({ type: "reportUrl", url: e.name }).catch(() => {});
      }
    }
  });
  try {
    obs.observe({ type: "resource", buffered: true });
  } catch (_) {}

  window.addEventListener("popstate", notifyLessonChange);
  setInterval(notifyLessonChange, 2000);
})();
