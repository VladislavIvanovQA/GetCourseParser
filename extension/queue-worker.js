const STORAGE_QUEUE = "gcQueue";
const STORAGE_SETTINGS = "gcSettings";
const DEFAULT_MAX_PARALLEL = 2;

let queueProcessing = false;
let activeJobs = 0;

async function loadSettings() {
  const { gcPort, gcToken, gcSavePdf, gcSaveText, gcMaxParallel } =
    await chrome.storage.local.get([
      "gcPort",
      "gcToken",
      "gcSavePdf",
      "gcSaveText",
      "gcMaxParallel",
    ]);
  return {
    port: gcPort || 18765,
    token: gcToken || "",
    savePdf: gcSavePdf !== false,
    saveText: gcSaveText !== false,
    maxParallel: Math.max(1, gcMaxParallel || DEFAULT_MAX_PARALLEL),
  };
}

async function loadQueue() {
  const { [STORAGE_QUEUE]: q } = await chrome.storage.local.get(STORAGE_QUEUE);
  return Array.isArray(q) ? q : [];
}

async function saveQueue(queue) {
  await chrome.storage.local.set({ [STORAGE_QUEUE]: queue });
  updateBadge(queue);
}

function updateBadge(queue) {
  const q = queue || [];
  const running = q.filter((j) => j.status === "running").length;
  const queued = q.filter((j) => j.status === "queued").length;
  if (running > 0) {
    chrome.action.setBadgeText({ text: running > 1 ? `${running}⇣` : "…" });
    chrome.action.setBadgeBackgroundColor({ color: "#e67e22" });
  } else if (queued > 0) {
    chrome.action.setBadgeText({ text: String(queued) });
    chrome.action.setBadgeBackgroundColor({ color: "#3498db" });
  } else {
    chrome.action.setBadgeText({ text: "" });
  }
}

async function patchJob(id, patch) {
  const queue = await loadQueue();
  const i = queue.findIndex((j) => j.id === id);
  if (i < 0) return;
  queue[i] = { ...queue[i], ...patch, updatedAt: Date.now() };
  await saveQueue(queue);
}

function lessonTitleFromUrl(url, tabTitle) {
  try {
    const u = new URL(url);
    const id = u.searchParams.get("id");
    if (id) return `Урок ${id}`;
  } catch (_) {}
  return (tabTitle || "Урок").slice(0, 80);
}

async function enqueueTab(tab, options = {}) {
  if (!tab?.id) throw new Error("Нет вкладки");
  const pageUrl = tab.url || "";
  if (!pageUrl.includes("/lesson/view")) {
    throw new Error("Откройте страницу урока GetCourse");
  }
  const queue = await loadQueue();
  if (queue.some((j) => j.pageUrl === pageUrl && (j.status === "queued" || j.status === "running"))) {
    throw new Error("Этот урок уже в очереди");
  }
  const job = {
    id: crypto.randomUUID(),
    tabId: tab.id,
    pageUrl,
    title: lessonTitleFromUrl(pageUrl, tab.title),
    status: "queued",
    progress: 0,
    message: "В очереди…",
    savePdf: options.savePdf !== false,
    saveText: options.saveText !== false,
    result: null,
    error: null,
    addedAt: Date.now(),
    updatedAt: Date.now(),
  };
  queue.push(job);
  await saveQueue(queue);
  processQueue();
  return job;
}

async function processQueue() {
  if (queueProcessing) return;
  queueProcessing = true;
  try {
    await launchParallelJobs();
  } finally {
    queueProcessing = false;
    updateBadge(await loadQueue());
  }
}

async function launchParallelJobs() {
  const settings = await loadSettings();
  const maxP = settings.maxParallel;
  while (true) {
    const queue = await loadQueue();
    const running = queue.filter((j) => j.status === "running").length;
    if (running >= maxP) return;
    const next = queue.find((j) => j.status === "queued");
    if (!next) return;
    activeJobs++;
    runJob(next).finally(() => {
      activeJobs--;
      processQueue();
    });
    if (running + 1 >= maxP) return;
  }
}

async function cookieHeaderForUrl(pageUrl) {
  if (!pageUrl) return "";
  let u;
  try {
    u = new URL(pageUrl);
  } catch {
    return "";
  }
  const cookies = await chrome.cookies.getAll({ url: pageUrl });
  if (!cookies.length) {
    const domainCookies = await chrome.cookies.getAll({ domain: u.hostname });
    return domainCookies.map((c) => `${c.name}=${c.value}`).join("; ");
  }
  return cookies.map((c) => `${c.name}=${c.value}`).join("; ");
}

async function collectUrlsFromAllFrames(tabId) {
  try {
    const results = await chrome.scripting.executeScript({
      target: { tabId, allFrames: true },
      func: () => {
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
        return [...urls];
      },
    });
    return (results || []).flatMap((r) => r.result || []);
  } catch {
    return [];
  }
}

async function getCapturedMedia(tabId, lessonUrl) {
  return new Promise((resolve) => {
    chrome.runtime.sendMessage(
      { type: "getCaptured", tabId, lessonUrl: lessonUrl || "" },
      (resp) => {
        resolve({
          urls: (resp && resp.urls) || [],
          videoAuth: (resp && resp.videoAuth) || "",
          playerCtx: (resp && resp.playerCtx) || {},
        });
      }
    );
  });
}

async function captureLessonPdf(tabId) {
  await chrome.scripting.insertCSS({ target: { tabId }, css: GC_PRINT_CSS });
  return printTabToPdfBase64(tabId);
}

async function apiPost(settings, path, body) {
  const r = await fetch(`http://127.0.0.1:${settings.port}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-GC-Token": settings.token,
    },
    body: JSON.stringify(body),
  });
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(data.error || `HTTP ${r.status}`);
  if (data.ok === false) throw new Error(data.error || "failed");
  return data;
}

async function apiGet(settings, path) {
  const r = await fetch(`http://127.0.0.1:${settings.port}${path}`, {
    headers: { "X-GC-Token": settings.token },
  });
  if (!r.ok) throw new Error(`HTTP ${r.status}`);
  return r.json();
}

async function pollDesktopJob(settings, jobId, extJobId) {
  for (let i = 0; i < 7200; i++) {
    await sleep(1000);
    const st = await apiGet(settings, `/api/job?id=${encodeURIComponent(jobId)}`);
    const pct = st.state === "done" ? 100 : Math.min(95, st.progress || 0);
    await patchJob(extJobId, {
      progress: pct,
      message: st.message || "Скачивание…",
    });
    if (st.state === "done") return st.result;
    if (st.state === "error") throw new Error(st.error || st.message || "Ошибка exe");
  }
  throw new Error("Таймаут скачивания");
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

async function runJob(job) {
  const settings = await loadSettings();
  if (!settings.token) {
    await patchJob(job.id, { status: "error", error: "Нет токена — нажмите Подключиться", progress: 0 });
    return;
  }

  await patchJob(job.id, { status: "running", progress: 5, message: "Сбор страницы…" });

  try {
    let tab;
    try {
      tab = await chrome.tabs.get(job.tabId);
    } catch {
      throw new Error("Вкладка закрыта — откройте урок снова и добавьте в очередь");
    }

    const [{ result: html }] = await chrome.scripting.executeScript({
      target: { tabId: job.tabId },
      func: () => document.documentElement.outerHTML,
    });

    const pageUrl = tab.url || job.pageUrl;
    const { urls: fromBg, videoAuth, playerCtx } = await getCapturedMedia(job.tabId, pageUrl);
    const fromFrames = await collectUrlsFromAllFrames(job.tabId);
    const captured = selectVideoInputs([...new Set([...fromBg, ...fromFrames])]);

    const u = new URL(pageUrl);
    const baseUrl = `${u.protocol}//${u.host}`;
    const cookie = await cookieHeaderForUrl(pageUrl);

    let videoReferer = (playerCtx.referer || "").trim() || "https://vhapi02.gcfiles.net/";
    let videoOrigin = (playerCtx.origin || "").trim();
    if (!videoOrigin) {
      try {
        videoOrigin = new URL(videoReferer).origin;
      } catch {
        videoOrigin = "https://vhapi02.gcfiles.net";
      }
    }

    let pagePdfBase64 = "";
    if (job.savePdf) {
      await patchJob(job.id, { progress: 15, message: "PDF страницы…" });
      try {
        pagePdfBase64 = await captureLessonPdf(job.tabId);
      } catch (e) {
        await patchJob(job.id, { message: "PDF пропущен: " + e.message });
      }
    }

    let videoUrls = [];
    if (captured.length) {
      try {
        const health = await gcFfmpegCheck.fetchHealth(settings.port);
        if (!health?.ffmpeg?.found) {
          const platformInfo = await new Promise((resolve) => {
            chrome.runtime.getPlatformInfo(resolve);
          });
          throw new Error(gcFfmpegCheck.ffmpegMissingMessage(health, platformInfo));
        }
      } catch (e) {
        if (String(e.message || e).includes("ffmpeg")) throw e;
        throw new Error(
          "Не удалось проверить ffmpeg — запущен ли GetCourseDownloader.exe?"
        );
      }
      await patchJob(job.id, { progress: 25, message: `Видео: ${captured.length}…` });
      const fetchHeaders = {
        Accept: "*/*",
        "Accept-Language": "ru-RU,ru;q=0.9,en-US;q=0.8",
        Origin: videoOrigin,
        Referer: videoReferer,
      };
      if ((videoAuth || "").trim()) fetchHeaders.Authorization = videoAuth.trim();
      const variants = await expandMasterPlaylists(captured, fetchHeaders);
      videoUrls = variants.map((v) => v.url).filter(Boolean);
    }

    await patchJob(job.id, { progress: 45, message: "Отправка в загрузчик…" });

    const payload = {
      referer: pageUrl,
      origin: baseUrl,
      base_url: baseUrl,
      video_referer: videoReferer,
      video_origin: videoOrigin,
      cookie,
      video_auth: (videoAuth || "").trim(),
      html: html || "",
      video_urls: videoUrls,
      page_pdf_base64: pagePdfBase64,
      save_lesson_text: job.saveText,
      async: true,
    };

    const started = await apiPost(settings, "/api/lesson", payload);
    const desktopJobId = started.job_id;
    if (!desktopJobId) throw new Error("exe не вернул job_id");

    await patchJob(job.id, { progress: 55, message: "Скачивание файлов и видео…" });
    const result = await pollDesktopJob(settings, desktopJobId, job.id);

    await patchJob(job.id, {
      status: "done",
      progress: 100,
      message: "Готово",
      result,
      title: result?.title || job.title,
    });
  } catch (e) {
    await patchJob(job.id, {
      status: "error",
      progress: 100,
      message: "Ошибка",
      error: String(e.message || e),
    });
  }
}

async function clearFinished() {
  const queue = await loadQueue();
  const kept = queue.filter((j) => j.status === "queued" || j.status === "running");
  await saveQueue(kept);
}

async function removeJob(id) {
  const queue = await loadQueue();
  await saveQueue(queue.filter((j) => j.id !== id || j.status === "running"));
}

globalThis.gcQueue = {
  enqueueTab,
  loadQueue,
  loadSettings,
  saveQueue,
  clearFinished,
  removeJob,
  processQueue,
  STORAGE_SETTINGS,
};
