/** Shared ffmpeg status helpers (popup + background queue). */

function clientOS(platformInfo) {
  const os = (platformInfo && platformInfo.os) || "win";
  if (os === "mac") return "darwin";
  if (os === "linux" || os === "openbsd" || os === "cros") return "linux";
  return "windows";
}

function pickInstallHint(health, platformInfo) {
  if (!health?.ffmpeg_install) return null;
  const server = health.platform || "windows";
  const browser = clientOS(platformInfo);
  if (server !== browser) {
    return {
      ...health.ffmpeg_install,
      title:
        health.ffmpeg_install.title +
        ` (ПК: ${server}, браузер: ${browser} — ориентируйтесь на ОС компьютера с exe)`,
    };
  }
  return health.ffmpeg_install;
}

function ffmpegMissingMessage(health, platformInfo) {
  const hint = pickInstallHint(health, platformInfo);
  if (!hint) return "ffmpeg не найден — откройте расширение для инструкции";
  const lines = [hint.title, "", ...(hint.steps || [])];
  if (hint.download_url) lines.push("", "Ссылка: " + hint.download_url);
  return lines.join("\n");
}

async function fetchHealth(port) {
  const r = await fetch(`http://127.0.0.1:${port}/health`);
  if (!r.ok) throw new Error(`HTTP ${r.status}`);
  return r.json();
}

if (typeof globalThis !== "undefined") {
  globalThis.gcFfmpegCheck = {
    clientOS,
    pickInstallHint,
    ffmpegMissingMessage,
    fetchHealth,
  };
}
