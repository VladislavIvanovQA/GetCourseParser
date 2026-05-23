#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  page_and_video_downloader.sh interactive
  page_and_video_downloader.sh lesson <html_file> [base_url] [parent_dir]
  page_and_video_downloader.sh video  <stream_url> [output_file]

Commands:
  interactive  Ask for HTML text (paste), extract lesson data, save lesson.txt,
               download attachments, then ask for video URL(s): after each download
               optionally another URL until you press Enter (empty line) to quit.
  lesson   Parse local lesson HTML, create folder by lesson title,
           save lesson.txt and download attached files there.
  video    Download video stream with ffmpeg copy mode.

Notes:
  - base_url is used for relative links (example: https://your-school.getcourse.ru)
  - /pl/fileservice/... обычно требует сессию: Referer (URL урока) и Cookie из браузера.
    Если Referer пустой, скрипт попробует собрать его из HTML (data-lesson-id + домен).
  - For protected files/streams, you may need cookies:
      export CURL_OPTS='-b cookies.txt'

Examples:
  ./page_and_video_downloader.sh interactive
  ./page_and_video_downloader.sh lesson "./lesson.html" "https://your-school.getcourse.ru" "./downloads"
  ./page_and_video_downloader.sh video "https://host/playlist.m3u8" "video.mp4"
EOF
}

need_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Error: command not found: $cmd" >&2
    exit 1
  fi
}

LESSON_COOKIE="${LESSON_COOKIE:-}"
LESSON_REFERER="${LESSON_REFERER:-}"
LESSON_ORIGIN="${LESSON_ORIGIN:-}"
LESSON_VIDEO_AUTH="${LESSON_VIDEO_AUTH:-}"
LESSON_USER_AGENT="${LESSON_USER_AGENT:-Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36}"

# Используем один массив вместо nameref к $3: при set -u и «чужом» scope bash мог дать "unbound variable".
GC_CURL_HEADER_ARGS=()

build_curl_header_args() {
  local referer="${1-}"
  local request_url="${2-}"
  local accept_hdr="Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
  local low_rq="${request_url,,}"
  if [[ "$low_rq" == *'/fileservice/'* || "$low_rq" == *'/file/download/'* ]]; then
    accept_hdr="Accept: */*"
  fi
  GC_CURL_HEADER_ARGS=(
    --compressed
    -H "$accept_hdr"
    -H "Accept-Language: ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7"
    -H "Cache-Control: no-cache"
    -H "Pragma: no-cache"
    -H "DNT: 1"
    -H "Upgrade-Insecure-Requests: 1"
    -H "User-Agent: $LESSON_USER_AGENT"
  )
  if [[ -n "${referer// }" ]]; then
    GC_CURL_HEADER_ARGS+=(-H "Referer: $referer")
    if [[ -z "${LESSON_ORIGIN// }" ]]; then
      local o
      o="$(REF="$referer" python3 -c "from urllib.parse import urlparse; import os; u=os.environ.get('REF',''); p=urlparse(u); print(f'{p.scheme}://{p.netloc}' if p.scheme and p.netloc else '')")"
      [[ -n "${o// }" ]] && GC_CURL_HEADER_ARGS+=(-H "Origin: $o")
    fi
  fi
  if [[ -n "${LESSON_ORIGIN// }" ]]; then
    GC_CURL_HEADER_ARGS+=(-H "Origin: $LESSON_ORIGIN")
  fi
  if [[ -n "${LESSON_COOKIE// }" ]]; then
    GC_CURL_HEADER_ARGS+=(-H "Cookie: $LESSON_COOKIE")
  fi
}

ffmpeg_origin_header() {
  local ref="$1"
  if [[ -n "${LESSON_ORIGIN// }" ]]; then
    printf '%s' "$LESSON_ORIGIN"
    return 0
  fi
  [[ -z "${ref// }" ]] && return 0
  REF="$ref" python3 -c "from urllib.parse import urlparse; import os; u=os.environ.get('REF',''); p=urlparse(u); print(f'{p.scheme}://{p.netloc}' if p.scheme and p.netloc else '')"
}

download_video() {
  local stream_url="$1"
  local output_file="${2:-video.mp4}"
  need_cmd ffmpeg
  echo "Downloading video with ffmpeg..."
  local ff_headers=""
  local origin_eff
  origin_eff="$(ffmpeg_origin_header "${LESSON_REFERER:-}")"
  ff_headers+="Accept: */*"$'\r\n'
  ff_headers+="Accept-Language: ru-RU,ru;q=0.9,en;q=0.8"$'\r\n'
  ff_headers+="Cache-Control: no-cache"$'\r\n'
  ff_headers+="Pragma: no-cache"$'\r\n'
  [[ -n "${origin_eff// }" ]] && ff_headers+="Origin: ${origin_eff}"$'\r\n'
  [[ -n "${LESSON_REFERER// }" ]] && ff_headers+="Referer: ${LESSON_REFERER}"$'\r\n'
  if [[ -n "${LESSON_VIDEO_AUTH// }" ]]; then
    local auth_val="$LESSON_VIDEO_AUTH"
    if [[ "$auth_val" != Bearer\ * && "$auth_val" != Basic\ * ]]; then
      auth_val="Bearer ${auth_val}"
    fi
    ff_headers+="Authorization: ${auth_val}"$'\r\n'
  fi
  [[ -n "${LESSON_COOKIE// }" ]] && ff_headers+="Cookie: ${LESSON_COOKIE}"$'\r\n'
  # HLS может тянуть вложенные плейлисты и crypto — whitelist снижает сюрпризы на старых ffmpeg.
  local tmp_plist
  tmp_plist="$(mktemp "${TMPDIR:-/tmp}/gc_plist_XXXXXX.m3u8")"
  build_curl_header_args "${LESSON_REFERER:-}" "$stream_url"
  local ff_hls_extra=()
  if [[ "${stream_url,,}" == *"/api/playlist/"* || "${stream_url,,}" == *"gceuproxy.com"* ]]; then
    ff_hls_extra=(-extension_picky 0 -allowed_extensions ALL -allowed_segment_extensions ALL -f hls)
  fi
  local ff_ok=1
  if curl -fsSL --retry 2 --retry-delay 1 "${GC_CURL_HEADER_ARGS[@]}" ${CURL_OPTS:-} -o "$tmp_plist" "$stream_url"; then
    if grep -Eq '^#EXTM3U|^#EXT-X-STREAM-INF' "$tmp_plist" 2>/dev/null; then
      echo "Using playlist fetched with curl (then ffmpeg reads segments)."
      ffmpeg \
        -loglevel warning -stats \
        -reconnect 1 -reconnect_streamed 1 -reconnect_delay_max 5 \
        -user_agent "$LESSON_USER_AGENT" \
        ${ff_headers:+-headers "$ff_headers"} \
        "${ff_hls_extra[@]}" \
        -protocol_whitelist "file,http,https,tcp,tls,crypto" \
        -i "$tmp_plist" \
        -c copy \
        -movflags +faststart \
        "$output_file" && ff_ok=0 || ff_ok=$?
      rm -f "$tmp_plist"
      [[ "$ff_ok" -eq 0 ]] && return 0
      echo "ffmpeg failed on local playlist, retrying direct URL..." >&2
    else
      rm -f "$tmp_plist"
    fi
  else
    rm -f "$tmp_plist"
    echo "Could not prefetch playlist via curl (auth/network); trying ffmpeg on URL..." >&2
  fi

  ffmpeg \
    -loglevel warning -stats \
    -reconnect 1 -reconnect_streamed 1 -reconnect_delay_max 5 \
    -user_agent "$LESSON_USER_AGENT" \
    ${ff_headers:+-headers "$ff_headers"} \
    "${ff_hls_extra[@]}" \
    -protocol_whitelist "file,http,https,tcp,tls,crypto" \
    -i "$stream_url" \
    -c copy \
    -movflags +faststart \
    "$output_file"
}

parse_lesson_html() {
  local html_file="$1"
  local base_url="${2:-}"
  local parent_dir="${3:-downloads}"
  local referer_url="${4:-$LESSON_REFERER}"

  need_cmd python3
  need_cmd curl
  need_cmd ffmpeg

  if [[ ! -f "$html_file" ]]; then
    echo "Error: HTML file not found: $html_file" >&2
    exit 1
  fi

  mkdir -p "$parent_dir"

  local tmp_meta tmp_files tmp_text tmp_links
  tmp_meta="$(mktemp)"
  tmp_files="$(mktemp)"
  tmp_text="$(mktemp)"
  tmp_links="$(mktemp)"
  trap 'rm -f "${tmp_meta:-}" "${tmp_files:-}" "${tmp_text:-}" "${tmp_links:-}"' RETURN

  python3 - "$html_file" "$base_url" "$tmp_meta" "$tmp_files" "$tmp_text" "$tmp_links" <<'PY'
import html as html_lib
import json
import os
import re
import sys
from urllib.parse import parse_qs, urljoin, urlsplit

html_file, base_url, meta_out, files_out, text_out, links_out = sys.argv[1:7]
raw = open(html_file, "r", encoding="utf-8", errors="ignore").read()

def pick_first(*patterns):
    for p in patterns:
        m = re.search(p, raw, flags=re.I | re.S)
        if m:
            return html_lib.unescape(m.group(1)).strip()
    return ""

def sanitize_name(s):
    s = s.strip() or "lesson"
    s = re.sub(r'[\\/:*?"<>|]+', "_", s)
    s = re.sub(r"\s+", " ", s).strip(" .")
    return s[:140] or "lesson"

_KNOWN_DL_EXT = frozenset({
    "pdf", "zip", "rar", "7z", "doc", "docx", "xls", "xlsx", "ppt", "pptx",
    "odt", "ods", "odp", "txt", "rtf", "csv", "png", "jpg", "jpeg", "gif",
    "webp", "svg", "mp3", "m4a", "wav", "ogg", "oga", "webm", "mp4", "opus",
    "aac", "epub", "djvu",
})


def _split_valid_ext(base: str):
    base = (base or "").strip().rstrip(".")
    i = base.rfind(".")
    if i <= 0 or i >= len(base) - 1:
        return base, ""
    ext = base[i:].lower()
    if not re.fullmatch(r"\.[a-z0-9]{1,12}", ext):
        return base, ""
    if ext.lstrip(".") not in _KNOWN_DL_EXT:
        return base, ""
    stem = base[:i].strip().rstrip(".") or "file"
    return stem, ext


def _ext_from_url(file_url: str) -> str:
    path = basename_label(file_url)
    _, ext = _split_valid_ext(path)
    return ext


def sanitize_filename(s):
    raw_s = ((s or "").strip() or "download").replace("\u00a0", " ")
    base = os.path.basename(raw_s.replace("\\", "/"))
    stem, ext = _split_valid_ext(base)
    stem = re.sub(r'[\\/:*?"<>|]+', "_", stem)
    stem = re.sub(r"\s+", " ", stem).strip().strip(".").strip() or "file"
    if ext:
        out = stem + ext
        if len(out) > 220:
            stem = stem[: max(1, 220 - len(ext))].rstrip(" .")
            out = stem + ext
        return out
    stem = re.sub(r'[\\/:*?"<>|]+', "_", base)
    stem = re.sub(r"\s+", " ", stem).strip().strip(".").strip() or "download"
    return stem[:180] or "download"


def filename_with_url_ext(display_name: str, download_url: str) -> str:
    name = sanitize_filename(display_name) if display_name else sanitize_filename(basename_label(download_url))
    url_ext = _ext_from_url(download_url)
    stem, cur_ext = _split_valid_ext(os.path.basename(name.replace("\\", "/")))
    if not url_ext:
        return name if cur_ext else stem
    if not cur_ext:
        return stem + url_ext
    if cur_ext.lower() == url_ext.lower():
        return stem + cur_ext
    return stem + url_ext

title = pick_first(
    r'<h2[^>]*class=["\'][^"\']*lesson-title-value[^"\']*["\'][^>]*>(.*?)</h2>',
    r'<h1[^>]*>(.*?)</h1>',
    r'<title[^>]*>(.*?)</title>',
)
title_plain = re.sub(r"(?is)<[^>]+>", " ", title)
title_plain = re.sub(r"\s+", " ", title_plain).strip()
folder_name = sanitize_name(title_plain)

all_links = []
seen_all = set()
url_to_display_name = {}

def anchor_label_from_inner(inner_html: str) -> str:
    text = re.sub(r"(?is)<[^>]+>", " ", inner_html)
    text = html_lib.unescape(text)
    text = re.sub(r"\s+", " ", text).strip()
    return text

def normalize_media_url(url: str) -> str:
    url = (url or "").strip()
    if url.startswith("//") and base_url:
        parsed = urlsplit(base_url)
        if parsed.scheme:
            url = parsed.scheme + ":" + url
    return urljoin(base_url, url) if base_url else url


def basename_label(u: str) -> str:
    return os.path.basename(u.split("?", 1)[0]) if u else ""


_KNOWN_MEDIA_EXT = frozenset(
    {"mp3", "m4a", "wav", "ogg", "oga", "webm", "mp4", "opus", "aac"}
)


def display_name_from_media_url(raw_url: str, fallback_title: str) -> str:
    """jPlayer title часто без расширения (хеш) — надёжнее взять basename из URL поля mp3/m4a/...."""
    abs_u = normalize_media_url((raw_url or "").strip())
    bn = basename_label(abs_u)
    stem, dot, ext = bn.rpartition(".")
    if dot and ext.lower() in _KNOWN_MEDIA_EXT and stem:
        return bn
    fb = (fallback_title or "").strip()
    return fb or bn


def looks_like_hashed_download_name(name: str) -> bool:
    base = basename_label(name)
    stem, dot, ext = base.rpartition(".")
    if not dot or len(stem) < 16:
        return False
    if ext.lower() not in {"mp3", "m4a", "wav", "ogg", "oga", "webm", "mp4", "pdf", "zip"}:
        return False
    core = stem.split("_")[0].split("-")[0]
    if len(core) < 16:
        return False
    return bool(re.fullmatch(r"(?i)[0-9a-f]+", core))


def wants_better_name(current: str, candidate: str) -> bool:
    """Prefer playlist / anchor text over bare hashed CDN filenames."""
    if not candidate:
        return False
    if not current:
        return True
    cand_is_raw_file = basename_label(candidate) == candidate.strip()
    cur_is_hashtype = looks_like_hashed_download_name(current)
    if cand_is_raw_file:
        return cur_is_hashtype
    if basename_label(current).lower() == basename_label(candidate).lower():
        return False
    return True


def remember_download(abs_url: str, display_name_candidate: str) -> None:
    if not abs_url or not is_lesson_attachment_url(abs_url):
        return
    prev = url_to_display_name.get(abs_url, "")
    if not prev.strip():
        url_to_display_name[abs_url] = display_name_candidate.strip() if display_name_candidate else basename_label(abs_url)
        return
    if display_name_candidate and wants_better_name(prev, display_name_candidate):
        url_to_display_name[abs_url] = display_name_candidate.strip()


def add_media_download(url: str, label: str) -> None:
    url = (url or "").strip()
    if not url or url.lower().startswith(("javascript:", "mailto:")):
        return
    abs_url = normalize_media_url(url)
    remember_download(abs_url, label)

def is_lesson_attachment_url(u: str) -> bool:
    if not u:
        return False
    low = u.lower()
    if "/fileservice/user/file/download/" in low:
        return True
    if "/fileservice/file/download/" in low and "/thumbnail/" not in low:
        return True
    if "/pl/fileservice/user/file/download/" in low:
        return True
    return "/file/download/" in low and "fileservice" in low

for m in re.finditer(
    r'(?is)<a\b([^>]*)>(.*?)</a>',
    raw,
):
    attrs, inner = m.group(1), m.group(2)
    hm = re.search(r'href\s*=\s*["\']([^"\']+)["\']', attrs, flags=re.I)
    if not hm:
        continue
    href = hm.group(1).strip()
    if not href or href.lower().startswith(("javascript:", "mailto:")):
        continue
    abs_url = urljoin(base_url, href) if base_url else href
    if abs_url not in seen_all:
        seen_all.add(abs_url)
        all_links.append(abs_url)
    if not is_lesson_attachment_url(abs_url):
        continue
    label = anchor_label_from_inner(inner)
    remember_download(abs_url, label)

def extract_balanced_segments(s: str, open_ch: str, close_ch: str):
    depth = 0
    start = None
    for i, ch in enumerate(s):
        if ch == open_ch:
            if depth == 0:
                start = i + 1
            depth += 1
        elif ch == close_ch:
            depth -= 1
            if depth == 0 and start is not None:
                yield s[start:i]
                start = None

# jPlayer playlist: new jPlayerPlaylist( ... , [ {...}, {...} ], {...} );
for jpm in re.finditer(r"(?is)new\s+jPlayerPlaylist\s*\(", raw):
    p0 = jpm.end()
    depth = 1
    i = p0
    comma_positions = []
    while i < len(raw) and depth > 0:
        c = raw[i]
        if c in ("\"", "'"):
            q = c
            i += 1
            while i < len(raw):
                if raw[i] == "\\":
                    i += 2
                    continue
                if raw[i] == q:
                    break
                i += 1
        elif c == "(":
            depth += 1
        elif c == ")":
            depth -= 1
            if depth == 0:
                body = raw[p0:i]
                break
        elif c == "," and depth == 1:
            comma_positions.append(i)
        i += 1
    else:
        continue
    if len(comma_positions) < 2:
        continue
    arr_start = comma_positions[1] + 1
    arr_body = ""
    ai = arr_start
    while ai < len(body) and body[ai] in " \t\r\n":
        ai += 1
    if ai >= len(body) or body[ai] != "[":
        continue
    depth_br = 0
    aj = ai
    while aj < len(body):
        ch = body[aj]
        if ch in ("\"", "'"):
            q = ch
            aj += 1
            while aj < len(body):
                if body[aj] == "\\":
                    aj += 2
                    continue
                if body[aj] == q:
                    break
                aj += 1
        elif ch == "[":
            if depth_br == 0:
                arr_body_start = aj + 1
            depth_br += 1
        elif ch == "]":
            depth_br -= 1
            if depth_br == 0:
                arr_body = body[arr_body_start:aj]
                break
        aj += 1
    else:
        continue
    for track in extract_balanced_segments(arr_body.strip(), "{", "}"):
        title_m = re.search(r'(?is)\btitle\s*:\s*["\']([^"\']*)["\']', track)
        title = html_lib.unescape(title_m.group(1).strip()) if title_m else ""

        mp3_m = re.search(r'(?is)\bmp3\s*:\s*["\']([^"\']+)["\']', track)
        if mp3_m:
            add_media_download(
                mp3_m.group(1),
                display_name_from_media_url(mp3_m.group(1), title),
            )
            continue

        # alternate formats only if mp3 missing
        for key in ("m4a", "oga", "wav", "webm", "mp4", "ogg"):
            km = re.search(rf'(?is)\b{re.escape(key)}\s*:\s*["\']([^"\']+)["\']', track)
            if km:
                add_media_download(
                    km.group(1),
                    display_name_from_media_url(km.group(1), title or f"media.{key}"),
                )
                break

# Audio/video URLs in small JS-like objects (fallback)
for obj in re.findall(r"\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}", raw, flags=re.S):
    title_m = re.search(r'(?is)\btitle\s*:\s*["\']([^"\']+)["\']', obj)
    title = html_lib.unescape(title_m.group(1).strip()) if title_m else ""

    for key in ("mp3", "m4a", "oga", "wav", "webm", "mp4", "ogg"):
        km = re.search(rf'(?is)\b{re.escape(key)}\s*:\s*["\']([^"\']+)["\']', obj)
        if not km:
            continue
        add_media_download(
            km.group(1),
            display_name_from_media_url(km.group(1), title or f"media.{key}"),
        )

    for um in re.finditer(
        r'(?is)https?://[^\s"\'<>]+?\.(mp3|m4a|wav|ogg|oga|webm|mp4)(?:\?[^\s"\'<>]*)?',
        obj,
    ):
        add_media_download(
            um.group(0),
            display_name_from_media_url(um.group(0), title or f"media.{um.group(1)}"),
        )

# Direct fileservice download URLs embedded anywhere (CDN host, scripts, plain text).
for fm in re.finditer(
    r'(?is)(?:https?:)?//[^\s"\'<>]+?/fileservice/file/download/[^\s"\'<>]+',
    raw,
):
    remember_download(normalize_media_url(fm.group(0)), "")

# Remove script/style/noscript before text extraction
clean = re.sub(r"(?is)<(script|style|noscript)[^>]*>.*?</\1>", " ", raw)
clean = re.sub(r"(?is)<br\s*/?>", "\n", clean)
clean = re.sub(r"(?is)</(p|div|section|article|li|h[1-6]|tr|td)\s*>", "\n", clean)
clean = re.sub(r"(?is)<[^>]+>", " ", clean)
clean = html_lib.unescape(clean)
clean = re.sub(r"[ \t\r\f\v]+", " ", clean)
clean = re.sub(r"\n[ \t]+", "\n", clean)
clean = re.sub(r"\n{3,}", "\n\n", clean).strip()

# Cut off noisy tail ("send answer", comments, etc.) from lesson text.
cut_markers = [
    "Отправить ответ",
    "Сохранить черновик",
    "Скрывать ответ от других учеников (будет виден только преподавателю)",
    "Ответы и комментарии",
    "старые ответы",
    "новые ответы",
]
cut_pos = -1
for marker in cut_markers:
    p = clean.find(marker)
    if p != -1 and (cut_pos == -1 or p < cut_pos):
        cut_pos = p
if cut_pos != -1:
    clean = clean[:cut_pos].rstrip()

lesson_id_attr = ""
m_lid = re.search(r'(?is)\bdata-lesson-id\s*=\s*["\'](\d+)["\']', raw)
if m_lid:
    lesson_id_attr = m_lid.group(1).strip()

lesson_hosts = []
lesson_ids_from_url: set[str] = set()
for vm in re.finditer(
    r'(?is)(https?://[^/\s"\'<>]+)(/pl/teach/control/lesson/view)(?:\?[^"\'>\s]*)?',
    raw,
):
    host = urlsplit(vm.group(1)).hostname or ""
    host = host.lower()
    if host and host not in lesson_hosts:
        lesson_hosts.append(host)
    qs = parse_qs(urlsplit(vm.group(0)).query)
    lid = (qs.get("id") or [None])[0]
    if lid and str(lid).isdigit():
        lesson_ids_from_url.add(str(lid))

for hm in re.finditer(
    r'(?is)href\s*=\s*["\']([^"\']*/pl/teach/control/lesson/view(?:\?[^"\']*)?)["\']',
    raw,
):
    join_u = urljoin(base_url, hm.group(1).strip()) if base_url else hm.group(1).strip()
    if "/pl/teach/control/lesson/view" not in join_u.lower():
        continue
    pu = urlsplit(join_u)
    qs = parse_qs(pu.query)
    lid = (qs.get("id") or [None])[0]
    if pu.hostname:
        host = pu.hostname.lower()
        if host and host not in lesson_hosts:
            lesson_hosts.append(host)
    if lid and str(lid).isdigit():
        lesson_ids_from_url.add(str(lid))

for abs_url in all_links:
    if "/pl/teach/control/lesson/view" not in abs_url.lower():
        continue
    pu = urlsplit(abs_url)
    qs = parse_qs(pu.query)
    lid = (qs.get("id") or [None])[0]
    if pu.hostname:
        host = pu.hostname.lower()
        if host and host not in lesson_hosts:
            lesson_hosts.append(host)
    if lid and str(lid).isdigit():
        lesson_ids_from_url.add(str(lid))

lesson_id_eff = lesson_id_attr
if not lesson_id_eff and lesson_ids_from_url:
    lesson_id_eff = min(lesson_ids_from_url, key=lambda x: int(x))

portal_host = (urlsplit(base_url).hostname or "").lower().strip() if base_url else ""
if not portal_host and lesson_hosts:
    portal_host = lesson_hosts[0]
if not portal_host:
    for u in url_to_display_name.keys():
        if "/pl/fileservice/" in u.lower():
            pu = urlsplit(u)
            if pu.hostname:
                portal_host = pu.hostname.lower().strip()
                break

scheme = urlsplit(base_url).scheme if base_url else "https"

suggested_referer = ""
if portal_host and lesson_id_eff and scheme:
    suggested_referer = f"{scheme}://{portal_host}/pl/teach/control/lesson/view?id={lesson_id_eff}"

meta = {
    "title": title_plain,
    "folder_name": folder_name,
    "lesson_id": lesson_id_eff,
    "portal_host": portal_host,
    "suggested_referer": suggested_referer,
}
with open(meta_out, "w", encoding="utf-8") as f:
    json.dump(meta, f, ensure_ascii=False)

seen_write = set()
with open(files_out, "w", encoding="utf-8") as f:
    for url, raw_name in url_to_display_name.items():
        dn = filename_with_url_ext(raw_name, url) if raw_name else filename_with_url_ext(basename_label(url), url)
        dn = dn.replace("\t", " ").replace("\r", " ").replace("\n", " ")
        if url in seen_write:
            continue
        seen_write.add(url)
        f.write(f"{url}\t{dn}\n")

with open(text_out, "w", encoding="utf-8") as f:
    f.write(clean + ("\n" if clean else ""))

with open(links_out, "w", encoding="utf-8") as f:
    for link in all_links:
        f.write(link + "\n")
PY

  local lesson_title lesson_folder suggested_ref
  lesson_title="$(python3 -c "import json;print(json.load(open(r'$tmp_meta',encoding='utf-8'))['title'])")"
  lesson_folder="$(python3 -c "import json;print(json.load(open(r'$tmp_meta',encoding='utf-8'))['folder_name'])")"
  suggested_ref="$(python3 -c "import json;print(json.load(open(r'$tmp_meta',encoding='utf-8')).get('suggested_referer',''))")"

  local effective_referer="$referer_url"
  if [[ -z "${effective_referer// }" ]]; then
    effective_referer="$suggested_ref"
  fi
  if [[ -n "${effective_referer// }" ]]; then
    LESSON_REFERER="$effective_referer"
  fi
  if [[ -z "${referer_url// }" && -n "${suggested_ref// }" ]]; then
    echo "Referer was empty — using from HTML: $suggested_ref"
  fi

  local target_dir="$parent_dir/$lesson_folder"
  mkdir -p "$target_dir"

  echo "Lesson: ${lesson_title:-unknown}"
  echo "Folder: $target_dir"

  local lesson_txt="$target_dir/lesson.txt"
  local links_txt="$target_dir/links.txt"
  {
    echo "Lesson: ${lesson_title:-unknown}"
    echo
    echo "=== TEXT ==="
    echo
    cat "$tmp_text"
    echo
    echo "=== LINKS ==="
    if [[ -s "$tmp_links" ]]; then
      cat "$tmp_links"
    else
      echo "(no links found)"
    fi
    echo
  } >"$lesson_txt"
  echo "Saved text and links: $lesson_txt"

  if [[ -s "$tmp_links" ]]; then
    cp "$tmp_links" "$links_txt"
  else
    : >"$links_txt"
  fi
  echo "Saved links only: $links_txt"

  if [[ -s "$tmp_files" ]]; then
    echo
    echo "Downloading lesson files..."
    if grep -Fq "/pl/fileservice/" "$tmp_files" 2>/dev/null && [[ -z "${LESSON_COOKIE// }" ]]; then
      echo "Note: attachments under /pl/fileservice/ usually need Cookie from your logged-in browser (same as Referer)." >&2
    fi
    local count=0
    while IFS=$'\t' read -r file_url friendly_name || [[ -n "${file_url:-}" ]]; do
      [[ -z "$file_url" ]] && continue
      count=$((count + 1))
      local filename target
      if [[ -n "${friendly_name// }" ]]; then
        filename="$friendly_name"
      else
        filename="$(basename "${file_url%%\?*}")"
      fi
      [[ -z "$filename" || "$filename" == "/" ]] && filename="file_${count}"
      target="$target_dir/$filename"
      if [[ -e "$target" ]]; then
        target="$target_dir/${count}_$filename"
      fi
      echo "[$count] $file_url"
      build_curl_header_args "${effective_referer:-}" "$file_url"
      # shellcheck disable=SC2086
      curl -fL --retry 2 --retry-delay 1 "${GC_CURL_HEADER_ARGS[@]}" ${CURL_OPTS:-} -o "$target" "$file_url" || {
        echo "  Failed: $file_url" >&2
      }
    done <"$tmp_files"
  else
    echo "No lesson file links found."
  fi

  echo
  echo "Done: $target_dir"

  local video_n=0 video_out
  while true; do
    if [[ "$video_n" -eq 0 ]]; then
      printf "URL первого видео (или Enter — пропустить видео): "
    else
      printf "Ещё есть видео? Вставьте URL или Enter — завершить: "
    fi
    IFS= read -r video_url || break
    if [[ -z "${video_url// }" ]]; then
      break
    fi
    video_out="$target_dir/video_$(printf '%02d' "$((video_n + 1))").mp4"
    if download_video "$video_url" "$video_out"; then
      video_n=$((video_n + 1))
    else
      echo "Не удалось скачать (можно указать другой URL или Enter для выхода)." >&2
      rm -f "$video_out"
    fi
  done
  if [[ "$video_n" -eq 0 ]]; then
    echo "Видео не скачивалось."
  else
    echo "Готово. Сохранено видео: $video_n файл(ов) в $target_dir"
  fi
}

interactive_mode() {
  local tmp_html
  tmp_html="$(mktemp)"
  trap 'rm -f "${tmp_html:-}"' RETURN

  echo "Paste full HTML below. Then enter a single line: __END_HTML__"
  : >"$tmp_html"
  while IFS= read -r line; do
    [[ "$line" == "__END_HTML__" ]] && break
    printf '%s\n' "$line" >>"$tmp_html"
  done

  if [[ ! -s "$tmp_html" ]]; then
    echo "Error: HTML text is empty." >&2
    exit 1
  fi

  printf "Enter base URL for relative links (example https://your-school.getcourse.ru): "
  IFS= read -r base_url
  printf "Enter lesson URL for Referer (optional): "
  IFS= read -r LESSON_REFERER
  printf "Enter Origin for video requests (optional, often player page): "
  IFS= read -r LESSON_ORIGIN
  printf "Enter Cookie header value (optional, full string): "
  IFS= read -r LESSON_COOKIE
  printf "Enter Authorization header for video only (optional, value after \"Bearer \"): "
  IFS= read -r LESSON_VIDEO_AUTH
  printf "Enter parent download directory [downloads]: "
  IFS= read -r parent_dir
  parent_dir="${parent_dir:-downloads}"

  parse_lesson_html "$tmp_html" "$base_url" "$parent_dir" "$LESSON_REFERER"
}

main() {
  if [[ $# -lt 1 ]]; then
    interactive_mode
    exit 0
  fi
  local cmd="$1"
  shift

  case "$cmd" in
    interactive)
      interactive_mode
      ;;
    lesson)
      [[ $# -lt 1 ]] && { usage; exit 1; }
      parse_lesson_html "$@"
      ;;
    video)
      [[ $# -lt 1 ]] && { usage; exit 1; }
      download_video "$@"
      ;;
    -h|--help|help)
      usage
      ;;
    *)
      echo "Unknown command: $cmd" >&2
      usage
      exit 1
      ;;
  esac
}

main "$@"
