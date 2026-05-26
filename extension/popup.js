const DEFAULT_PORT = 18765;

const portEl = document.getElementById("port");
const tokenEl = document.getElementById("token");
const savePdfEl = document.getElementById("savePdf");
const saveTextEl = document.getElementById("saveText");
const maxParallelEl = document.getElementById("maxParallel");
const queueEl = document.getElementById("queue");
const ffmpegBanner = document.getElementById("ffmpegBanner");
const ffmpegTitle = document.getElementById("ffmpegTitle");
const ffmpegPath = document.getElementById("ffmpegPath");
const ffmpegSteps = document.getElementById("ffmpegSteps");
const ffmpegDownloadBtn = document.getElementById("ffmpegDownload");
const ffmpegRecheckBtn = document.getElementById("ffmpegRecheck");

let lastInstallHint = null;

async function loadSettings() {
  const data = await chrome.storage.local.get([
    "gcPort",
    "gcToken",
    "gcSavePdf",
    "gcSaveText",
    "gcMaxParallel",
  ]);
  if (data.gcPort) portEl.value = data.gcPort;
  if (data.gcToken) tokenEl.value = data.gcToken;
  if (data.gcSavePdf !== undefined) savePdfEl.checked = data.gcSavePdf;
  if (data.gcSaveText !== undefined) saveTextEl.checked = data.gcSaveText;
  if (data.gcMaxParallel) maxParallelEl.value = data.gcMaxParallel;
}

async function saveSettings() {
  await chrome.storage.local.set({
    gcPort: Number(portEl.value) || DEFAULT_PORT,
    gcToken: tokenEl.value.trim(),
    gcSavePdf: savePdfEl.checked,
    gcSaveText: saveTextEl.checked,
    gcMaxParallel: Math.max(1, Math.min(8, Number(maxParallelEl.value) || 2)),
  });
}

async function apiGet(path) {
  const port = Number(portEl.value) || DEFAULT_PORT;
  const r = await fetch(`http://127.0.0.1:${port}${path}`);
  if (!r.ok) throw new Error(`HTTP ${r.status}`);
  return r.json();
}

function sendMsg(msg) {
  return new Promise((resolve) => {
    chrome.runtime.sendMessage(msg, (resp) => {
      if (chrome.runtime.lastError) {
        resolve({ ok: false, error: chrome.runtime.lastError.message });
        return;
      }
      resolve(resp || {});
    });
  });
}

function renderQueue(queue) {
  if (!queue?.length) {
    queueEl.innerHTML = '<p class="hint">Очередь пуста</p>';
    return;
  }
  queueEl.innerHTML = queue
    .map((j) => {
      const pct = j.progress || 0;
      let extra = "";
      if (j.status === "done" && j.result) {
        extra = `<div class="job-msg">Файлов: ${j.result.files_saved}, видео: ${j.result.videos_saved}${j.result.pdf_saved ? ", PDF" : ""}</div>`;
      }
      if (j.status === "error") {
        extra = `<div class="job-msg" style="color:#a00">${escapeHtml(j.error || "")}</div>`;
      }
      return `<div class="job ${j.status}">
        <div class="job-title">${escapeHtml(j.title || j.pageUrl || "Урок")}</div>
        <div class="job-msg">${escapeHtml(j.message || j.status)}</div>
        <div class="bar"><i style="width:${pct}%"></i></div>
        ${extra}
      </div>`;
    })
    .join("");
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

async function refreshQueue() {
  const resp = await sendMsg({ type: "getQueueState" });
  renderQueue(resp.queue || []);
}

function renderFfmpegBanner(health, platformInfo) {
  const ff = health?.ffmpeg;
  if (!ff) {
    ffmpegBanner.classList.remove("visible", "ok");
    return;
  }
  if (ff.found) {
    ffmpegBanner.classList.remove("visible", "ok");
    lastInstallHint = null;
    return;
  }

  const hint = gcFfmpegCheck.pickInstallHint(health, platformInfo);
  lastInstallHint = hint;
  ffmpegBanner.classList.add("visible");
  ffmpegBanner.classList.remove("ok");
  ffmpegTitle.textContent = hint?.title || "Нужен ffmpeg для видео";
  ffmpegPath.innerHTML = `Положите <code>${escapeHtml(ff.expected_binary || "ffmpeg")}</code> в:<br><code>${escapeHtml(ff.app_dir || "")}</code>`;
  ffmpegSteps.replaceChildren();
  for (const step of hint?.steps || []) {
    const li = document.createElement("li");
    li.textContent = step;
    ffmpegSteps.appendChild(li);
  }
  ffmpegDownloadBtn.style.display = hint?.download_url ? "" : "none";
  ffmpegDownloadBtn.textContent = health.platform === "windows" ? "Скачать ZIP (Windows)" : "Открыть ссылку";
}

async function checkFfmpegStatus() {
  const port = Number(portEl.value) || DEFAULT_PORT;
  try {
    const health = await gcFfmpegCheck.fetchHealth(port);
    const platformInfo = await new Promise((resolve) => {
      chrome.runtime.getPlatformInfo(resolve);
    });
    renderFfmpegBanner(health, platformInfo);
    return health;
  } catch {
    ffmpegBanner.classList.remove("visible", "ok");
    return null;
  }
}

document.getElementById("connect").addEventListener("click", async () => {
  try {
    await saveSettings();
    const health = await apiGet("/health");
    const pair = await apiGet("/api/pair");
    tokenEl.value = pair.token || "";
    await saveSettings();
    const platformInfo = await new Promise((resolve) => {
      chrome.runtime.getPlatformInfo(resolve);
    });
    renderFfmpegBanner(health, platformInfo);
    const ff = health.ffmpeg;
    const ffNote = ff?.found ? "\nffmpeg: OK" : "\n⚠ ffmpeg не найден — видео не скачаются (см. блок ниже)";
    alert(`Подключено v${health.version || "?"}${ffNote}`);
  } catch (e) {
    alert("Не удалось подключиться.\nЗапущен ли GetCourseDownloader.exe?\n" + e.message);
  }
});

async function enqueueCurrent() {
  await saveSettings();
  const health = await checkFfmpegStatus();
  if (health?.ffmpeg && !health.ffmpeg.found) {
    const go = confirm(
      "ffmpeg не найден — видео с урока не сохранятся (файлы и PDF могут скачаться).\n\nВсё равно добавить в очередь?"
    );
    if (!go) return;
  }
  const resp = await sendMsg({
    type: "enqueueCurrentTab",
    savePdf: savePdfEl.checked,
    saveText: saveTextEl.checked,
  });
  if (!resp.ok) {
    alert(resp.error || "Не удалось добавить");
    return;
  }
  await refreshQueue();
}

document.getElementById("enqueue").addEventListener("click", enqueueCurrent);
document.getElementById("enqueueNow").addEventListener("click", enqueueCurrent);

document.getElementById("clearDone").addEventListener("click", async () => {
  await sendMsg({ type: "clearFinishedJobs" });
  await refreshQueue();
});

chrome.storage.onChanged.addListener((changes, area) => {
  if (area === "local" && changes.gcQueue) {
    renderQueue(changes.gcQueue.newValue || []);
  }
});

ffmpegDownloadBtn.addEventListener("click", () => {
  const url = lastInstallHint?.download_url || lastInstallHint?.doc_url;
  if (url) chrome.tabs.create({ url });
  else alert("Следуйте шагам в списке выше.");
});

ffmpegRecheckBtn.addEventListener("click", async () => {
  const health = await checkFfmpegStatus();
  if (health?.ffmpeg?.found) alert("ffmpeg найден — можно скачивать видео.");
  else alert("ffmpeg всё ещё не найден. Положите бинарник в папку exe и перезапустите программу.");
});

loadSettings().then(() => {
  refreshQueue();
  checkFfmpegStatus();
});
setInterval(refreshQueue, 1500);
setInterval(checkFfmpegStatus, 5000);
