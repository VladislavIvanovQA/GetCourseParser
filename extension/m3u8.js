/** @typedef {{ url: string, bandwidth: number, resolution: string }} Variant */

function shouldCaptureVideoRequest(url) {
  if (!url) return false;
  if (/\/api\/playlist\/master\//i.test(url)) return true;
  if (/\/api\/playlist\/media\//i.test(url)) return true;
  if (/gceuproxy\.com/i.test(url) && /playlist/i.test(url)) return true;
  if (/\.m3u8(\?|$)/i.test(url)) return true;
  if (/\.(mp4|webm|m4v)(\?|$)/i.test(url)) return true;
  return false;
}

function isMasterPlaylistUrl(url) {
  return /\/api\/playlist\/master\//i.test(url) || /master\.m3u8/i.test(url);
}

function resolvePlaylistUrl(line, baseUrl) {
  const t = line.trim();
  if (!t || t.startsWith("#")) return null;
  if (/^https?:\/\//i.test(t)) return t;
  try {
    return new URL(t, baseUrl).href;
  } catch {
    return null;
  }
}

/** Parse #EXTM3U master or media playlist — all segment / variant URLs. */
function parseM3U8Variants(text, baseUrl) {
  /** @type {Variant[]} */
  const variants = [];
  const lines = text.split(/\r?\n/);
  let pendingBw = 0;
  let pendingRes = "";

  for (const raw of lines) {
    const line = raw.trim();
    if (line.startsWith("#EXT-X-STREAM-INF")) {
      const bw = line.match(/BANDWIDTH=(\d+)/i);
      const res = line.match(/RESOLUTION=(\d+x\d+)/i);
      pendingBw = bw ? parseInt(bw[1], 10) : 0;
      pendingRes = res ? res[1] : "";
      continue;
    }
    const url = resolvePlaylistUrl(line, baseUrl);
    if (!url) continue;
    variants.push({
      url,
      bandwidth: pendingBw,
      resolution: pendingRes,
    });
    pendingBw = 0;
    pendingRes = "";
  }
  return variants;
}

/** One logical stream (same media ids, different resolution). */
function streamGroupKey(url) {
  try {
    const u = new URL(url);
    const m = u.pathname.match(/\/media\/([^/]+)\/([^/]+)\//i);
    if (m) return `${u.host}|${m[1]}|${m[2]}`;
    const master = u.pathname.match(/\/master\/([^/]+)\/([^/]+)/i);
    if (master) return `${u.host}|master|${master[1]}|${master[2]}`;
    return `${u.host}|${u.pathname.split("?")[0]}`;
  } catch {
    return url;
  }
}

function resolutionFromMediaPath(url) {
  const m = String(url).match(/\/(\d{3,4})(?:\?|$|\/)/);
  return m ? parseInt(m[1], 10) : 0;
}

/** One master or one media stream per video on the lesson page. */
function selectVideoInputs(urls) {
  const masters = [];
  const seenMaster = new Set();
  for (const u of urls) {
    if (!isMasterPlaylistUrl(u)) continue;
    const k = streamGroupKey(u);
    if (seenMaster.has(k)) continue;
    seenMaster.add(k);
    masters.push(u);
  }
  if (masters.length) return masters;

  const mediaBest = new Map();
  for (const u of urls) {
    if (!/\/api\/playlist\/media\//i.test(u)) continue;
    const k = streamGroupKey(u);
    const score = resolutionFromMediaPath(u);
    const prev = mediaBest.get(k);
    if (!prev || score > prev.score) mediaBest.set(k, { url: u, score });
  }
  return [...mediaBest.values()].map((x) => x.url);
}

/** Keep highest BANDWIDTH per stream; if tie, prefer larger resolution. */
function pickBestVariants(variants) {
  const best = new Map();
  for (const v of variants) {
    const key = streamGroupKey(v.url);
    const prev = best.get(key);
    if (!prev || variantScore(v) > variantScore(prev)) {
      best.set(key, v);
    }
  }
  return [...best.values()];
}

function variantScore(v) {
  let score = v.bandwidth || 0;
  const m = (v.resolution || "").match(/(\d+)x(\d+)/);
  if (m) score += parseInt(m[1], 10) * parseInt(m[2], 10);
  return score;
}

async function expandMasterPlaylists(urls, fetchHeaders) {
  const inputs = selectVideoInputs(urls);
  const all = [];
  const seen = new Set();

  const addVariant = (v) => {
    if (!v?.url || seen.has(v.url)) return;
    seen.add(v.url);
    all.push(v);
  };

  for (const url of inputs) {
    if (!isMasterPlaylistUrl(url)) {
      addVariant({ url, bandwidth: 0, resolution: "" });
      continue;
    }
    try {
      const r = await fetch(url, {
        credentials: "omit",
        headers: fetchHeaders,
      });
      if (!r.ok) {
        addVariant({ url, bandwidth: 0, resolution: "" });
        continue;
      }
      const text = await r.text();
      if (!text.includes("#EXTM3U")) {
        addVariant({ url, bandwidth: 0, resolution: "" });
        continue;
      }
      for (const v of parseM3U8Variants(text, url)) {
        addVariant(v);
      }
    } catch {
      addVariant({ url, bandwidth: 0, resolution: "" });
    }
  }

  const picked = pickBestVariants(all);
  if (!picked.length) {
    for (const url of inputs) {
      addVariant({ url, bandwidth: 0, resolution: "" });
    }
  }
  return picked.length ? picked : all;
}

globalThis.gcM3U8 = {
  shouldCaptureVideoRequest,
  isMasterPlaylistUrl,
  parseM3U8Variants,
  selectVideoInputs,
  expandMasterPlaylists,
  pickBestVariants,
};
