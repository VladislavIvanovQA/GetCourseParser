importScripts("m3u8.js", "pdf.js", "ffmpeg-check.js", "queue-worker.js");

const capturedByTab = new Map();
const authByTab = new Map();
/** @type {Map<number, { referer?: string, origin?: string }>} */
const playerCtxByTab = new Map();
const lessonKeyByTab = new Map();

function lessonKeyFromUrl(url) {
  try {
    const u = new URL(url);
    if (u.pathname.includes("/pl/teach/control/lesson/view")) {
      return u.searchParams.get("id") || u.href;
    }
    return u.pathname + u.search;
  } catch {
    return url || "";
  }
}

function resetTabCapture(tabId) {
  capturedByTab.set(tabId, new Set());
  playerCtxByTab.set(tabId, {});
}

function onLessonChanged(tabId, pageUrl) {
  if (tabId < 0) return;
  const key = lessonKeyFromUrl(pageUrl);
  const prev = lessonKeyByTab.get(tabId);
  if (prev !== key) {
    lessonKeyByTab.set(tabId, key);
    resetTabCapture(tabId);
  }
}

function rememberUrl(tabId, url) {
  if (!shouldCaptureVideoRequest(url)) return;
  if (tabId < 0) return;

  let set = capturedByTab.get(tabId);
  if (!set) {
    set = new Set();
    capturedByTab.set(tabId, set);
  }
  set.add(url);
  trimSet(set, 40, 25);
}

function trimSet(set, max, keep) {
  if (set.size <= max) return;
  const arr = [...set];
  set.clear();
  arr.slice(-keep).forEach((u) => set.add(u));
}

function urlsForTab(tabId) {
  return [...(capturedByTab.get(tabId) || [])];
}

chrome.webNavigation.onCommitted.addListener((details) => {
  if (details.frameId !== 0) return;
  onLessonChanged(details.tabId, details.url);
});

chrome.webRequest.onBeforeRequest.addListener(
  (details) => {
    rememberUrl(details.tabId, details.url || "");
  },
  { urls: ["<all_urls>"] }
);

chrome.webRequest.onBeforeSendHeaders.addListener(
  (details) => {
    const url = details.url || "";
    const tabId = details.tabId;
    if (tabId < 0) return;

    let auth = "";
    let referer = "";
    let origin = "";

    for (const h of details.requestHeaders || []) {
      if (!h.name) continue;
      const n = h.name.toLowerCase();
      if (n === "authorization" && h.value) auth = h.value;
      if (n === "referer" && h.value) referer = h.value;
      if (n === "origin" && h.value) origin = h.value;
    }

    const isPlayerCdn =
      /gceuproxy\.com/i.test(url) ||
      /gcfiles\.net/i.test(url) ||
      /\/api\/playlist\//i.test(url);

    if (auth) authByTab.set(tabId, auth);
    if (isPlayerCdn && (referer || origin)) {
      const prev = playerCtxByTab.get(tabId) || {};
      playerCtxByTab.set(tabId, {
        referer: referer || prev.referer,
        origin: origin || prev.origin,
      });
    }
  },
  { urls: ["<all_urls>"] },
  ["requestHeaders"]
);

chrome.tabs.onRemoved.addListener((tabId) => {
  capturedByTab.delete(tabId);
  authByTab.delete(tabId);
  playerCtxByTab.delete(tabId);
  lessonKeyByTab.delete(tabId);
});

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  const tabId = sender.tab?.id;

  if (msg.type === "lessonChanged" && tabId != null) {
    onLessonChanged(tabId, msg.url || "");
    sendResponse({ ok: true });
    return false;
  }
  if (msg.type === "reportUrls" && Array.isArray(msg.urls)) {
    if (tabId != null) {
      for (const u of msg.urls) rememberUrl(tabId, u);
    }
    sendResponse({ ok: true });
    return false;
  }
  if (msg.type === "reportUrl" && msg.url) {
    if (tabId != null) rememberUrl(tabId, msg.url);
    sendResponse({ ok: true });
    return false;
  }
  if (msg.type === "getCaptured") {
    if (msg.lessonUrl) onLessonChanged(msg.tabId, msg.lessonUrl);
    sendResponse({
      urls: urlsForTab(msg.tabId),
      videoAuth: authByTab.get(msg.tabId) || "",
      playerCtx: playerCtxByTab.get(msg.tabId) || {},
    });
    return false;
  }
  if (msg.type === "printLessonPDF") {
    (async () => {
      try {
        const tabId = msg.tabId;
        await chrome.scripting.insertCSS({
          target: { tabId },
          css: GC_PRINT_CSS,
        });
        const data = await printTabToPdfBase64(tabId);
        sendResponse({ ok: true, pdfBase64: data });
      } catch (e) {
        sendResponse({ ok: false, error: String(e.message || e) });
      }
    })();
    return true;
  }
  if (msg.type === "enqueueCurrentTab") {
    (async () => {
      try {
        const tab = msg.tabId
          ? await chrome.tabs.get(msg.tabId)
          : (await chrome.tabs.query({ active: true, currentWindow: true }))[0];
        const job = await enqueueTab(tab, {
          savePdf: msg.savePdf,
          saveText: msg.saveText,
        });
        sendResponse({ ok: true, job });
      } catch (e) {
        sendResponse({ ok: false, error: String(e.message || e) });
      }
    })();
    return true;
  }
  if (msg.type === "getQueueState") {
    loadQueue().then((queue) => sendResponse({ queue }));
    return true;
  }
  if (msg.type === "clearFinishedJobs") {
    clearFinished().then(() => sendResponse({ ok: true }));
    return true;
  }
  if (msg.type === "removeQueueJob") {
    removeJob(msg.id).then(() => sendResponse({ ok: true }));
    return true;
  }
  return false;
});

loadQueue().then(updateBadge);
processQueue();
