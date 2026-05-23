package parser

import (
	"path"
	"regexp"
	"strings"
)

var reValidExt = regexp.MustCompile(`^\.[a-z0-9]{1,12}$`)

// Extensions we trust from download URLs and display names.
var knownDownloadExt = map[string]bool{
	"pdf": true, "zip": true, "rar": true, "7z": true,
	"doc": true, "docx": true, "xls": true, "xlsx": true,
	"ppt": true, "pptx": true, "odt": true, "ods": true, "odp": true,
	"txt": true, "rtf": true, "csv": true,
	"png": true, "jpg": true, "jpeg": true, "gif": true, "webp": true, "svg": true,
	"mp3": true, "m4a": true, "wav": true, "ogg": true, "oga": true, "webm": true, "mp4": true,
	"opus": true, "aac": true, "epub": true, "djvu": true,
}

func extFromURL(fileURL string) string {
	if fileURL == "" {
		return ""
	}
	base := path.Base(strings.SplitN(fileURL, "?", 2)[0])
	_, ext := splitValidExt(base)
	return ext
}

func splitValidExt(base string) (stem, ext string) {
	base = strings.TrimSpace(base)
	i := strings.LastIndex(base, ".")
	if i <= 0 || i >= len(base)-1 {
		return strings.TrimRight(base, "."), ""
	}
	ext = strings.ToLower(base[i:])
	if !reValidExt.MatchString(ext) {
		return strings.TrimRight(base, "."), ""
	}
	e := strings.TrimPrefix(ext, ".")
	if !knownDownloadExt[e] {
		return strings.TrimRight(base, "."), ""
	}
	stem = strings.TrimSpace(base[:i])
	stem = strings.TrimRight(stem, ".")
	if stem == "" {
		return "file", ext
	}
	return stem, ext
}

func isKnownExt(ext string) bool {
	if ext == "" {
		return false
	}
	e := strings.TrimPrefix(strings.ToLower(ext), ".")
	return knownDownloadExt[e]
}

// FilenameWithURLExt builds a safe filename: human-readable name + extension from URL when needed.
func FilenameWithURLExt(displayName, downloadURL string) string {
	displayName = sanitizeFilename(displayName)
	if displayName == "" {
		displayName = sanitizeFilename(basenameLabel(downloadURL))
	}

	urlExt := extFromURL(downloadURL)
	stem, curExt := splitValidExt(path.Base(displayName))

	if urlExt == "" {
		return finalizeFilename(stem, curExt)
	}

	if curExt == "" {
		return finalizeFilename(stem, urlExt)
	}

	curKey := strings.TrimPrefix(strings.ToLower(curExt), ".")
	urlKey := strings.TrimPrefix(strings.ToLower(urlExt), ".")

	if curKey == urlKey {
		return finalizeFilename(stem, curExt)
	}

	// Display name has a different extension than the URL — trust the URL.
	return finalizeFilename(stem, urlExt)
}

func finalizeFilename(stem, ext string) string {
	stem = reBadName.ReplaceAllString(stem, "_")
	stem = strings.Trim(reMultiSpace.ReplaceAllString(stem, " "), ".")
	if stem == "" {
		stem = "file"
	}
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if ext != "" && !isKnownExt(ext) {
		ext = ""
	}
	out := stem + ext
	if len(out) > 220 && ext != "" {
		out = stem[:max(1, 220-len(ext))] + ext
	}
	return out
}
