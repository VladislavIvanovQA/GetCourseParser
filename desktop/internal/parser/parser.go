package parser

import (
	"html"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	reLessonTitle = regexp.MustCompile(`(?is)<h2[^>]*class=["'][^"']*lesson-title-value[^"']*["'][^>]*>(.*?)</h2>`)
	reH1          = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	reTitleTag    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reStripTags   = regexp.MustCompile(`(?is)<[^>]+>`)
	reLessonID    = regexp.MustCompile(`(?is)\bdata-lesson-id\s*=\s*["'](\d+)["']`)
	reAnchor      = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
	reHref        = regexp.MustCompile(`(?is)href\s*=\s*["']([^"']+)["']`)
	reFileservice = regexp.MustCompile(`(?is)(?:https?:)?//[^\s"'<>]+?/fileservice/file/download/[^\s"'<>]+`)
	reJPlayer     = regexp.MustCompile(`(?is)new\s+jPlayerPlaylist\s*\(`)
	reMediaKey    = regexp.MustCompile(`(?is)\b(mp3|m4a|oga|wav|webm|mp4|ogg)\s*:\s*["']([^"']+)["']`)
	reTitleJS     = regexp.MustCompile(`(?is)\btitle\s*:\s*["']([^"']*)["']`)
	reLessonView  = regexp.MustCompile(`(?is)(https?://[^/\s"'<>]+)(/pl/teach/control/lesson/view)(?:\?[^"'>\s]*)?`)
	reLessonHref  = regexp.MustCompile(`(?is)href\s*=\s*["']([^"']*/pl/teach/control/lesson/view(?:\?[^"']*)?)["']`)
	reScriptBlock = regexp.MustCompile(`(?is)<script[^>]*>[\s\S]*?</script>`)
	reStyleBlock  = regexp.MustCompile(`(?is)<style[^>]*>[\s\S]*?</style>`)
	reNoscriptBlk = regexp.MustCompile(`(?is)<noscript[^>]*>[\s\S]*?</noscript>`)
	reBr          = regexp.MustCompile(`(?is)<br\s*/?>`)
	reBlockEnd    = regexp.MustCompile(`(?is)</(p|div|section|article|li|h[1-6]|tr|td)\s*>`)
	reSpaces      = regexp.MustCompile(`[ \t\r\f\v]+`)
	reNLSpaces    = regexp.MustCompile(`\n[ \t]+`)
	reManyNL      = regexp.MustCompile(`\n{3,}`)
	reBadName     = regexp.MustCompile(`[\\/:*?"<>|]+`)
	reMultiSpace  = regexp.MustCompile(`\s+`)
	reHexHash     = regexp.MustCompile(`(?i)^[0-9a-f]+$`)
)

var knownMediaExt = map[string]bool{
	"mp3": true, "m4a": true, "wav": true, "ogg": true, "oga": true,
	"webm": true, "mp4": true, "opus": true, "aac": true,
}

var cutMarkers = []string{
	"Отправить ответ",
	"Сохранить черновик",
	"Скрывать ответ от других учеников (будет виден только преподавателю)",
	"Ответы и комментарии",
	"старые ответы",
	"новые ответы",
}

type FileEntry struct {
	URL  string
	Name string
}

type Result struct {
	Title            string
	FolderName       string
	LessonID         string
	SuggestedReferer string
	Text             string
	AllLinks         []string
	Files            []FileEntry
}

func ParseHTML(raw, baseURL string) *Result {
	r := &Result{}
	title := pickFirst(raw, reLessonTitle, reH1, reTitleTag)
	titlePlain := strings.TrimSpace(reMultiSpace.ReplaceAllString(reStripTags.ReplaceAllString(title, " "), " "))
	r.Title = titlePlain
	r.FolderName = sanitizeName(titlePlain)

	urlToName := make(map[string]string)
	seenAll := make(map[string]bool)
	var allLinks []string

	addLink := func(u string) {
		if u == "" || seenAll[u] {
			return
		}
		seenAll[u] = true
		allLinks = append(allLinks, u)
	}

	remember := func(absURL, display string) {
		if absURL == "" || !isLessonAttachmentURL(absURL) {
			return
		}
		prev := urlToName[absURL]
		if prev == "" {
			if display == "" {
				display = basenameLabel(absURL)
			}
			urlToName[absURL] = strings.TrimSpace(display)
			return
		}
		if display != "" && wantsBetterName(prev, display) {
			urlToName[absURL] = strings.TrimSpace(display)
		}
	}

	addMedia := func(u, label string) {
		u = strings.TrimSpace(u)
		if u == "" || strings.HasPrefix(strings.ToLower(u), "javascript:") || strings.HasPrefix(strings.ToLower(u), "mailto:") {
			return
		}
		abs := normalizeMediaURL(u, baseURL)
		remember(abs, label)
	}

	for _, m := range reAnchor.FindAllStringSubmatch(raw, -1) {
		attrs, inner := m[1], m[2]
		hm := reHref.FindStringSubmatch(attrs)
		if hm == nil {
			continue
		}
		href := strings.TrimSpace(hm[1])
		if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") || strings.HasPrefix(strings.ToLower(href), "mailto:") {
			continue
		}
		abs := joinURL(baseURL, href)
		addLink(abs)
		if !isLessonAttachmentURL(abs) {
			continue
		}
		label := anchorLabel(inner)
		remember(abs, label)
	}

	parseJPlayer(raw, baseURL, addMedia)

	// fallback small JS objects
	for _, obj := range findBraceObjects(raw) {
		tm := reTitleJS.FindStringSubmatch(obj)
		titleJS := ""
		if tm != nil {
			titleJS = html.UnescapeString(strings.TrimSpace(tm[1]))
		}
		for _, km := range reMediaKey.FindAllStringSubmatch(obj, -1) {
			key, val := km[1], km[2]
			label := displayNameFromMediaURL(val, baseURL, titleJS)
			if label == "" {
				label = "media." + key
			}
			addMedia(val, label)
		}
	}

	for _, fm := range reFileservice.FindAllString(raw, -1) {
		remember(normalizeMediaURL(fm, baseURL), "")
	}

	clean := reNoscriptBlk.ReplaceAllString(raw, " ")
	clean = reStyleBlock.ReplaceAllString(clean, " ")
	clean = reScriptBlock.ReplaceAllString(clean, " ")
	clean = reBr.ReplaceAllString(clean, "\n")
	clean = reBlockEnd.ReplaceAllString(clean, "\n")
	clean = reStripTags.ReplaceAllString(clean, " ")
	clean = html.UnescapeString(clean)
	clean = reSpaces.ReplaceAllString(clean, " ")
	clean = reNLSpaces.ReplaceAllString(clean, "\n")
	clean = strings.TrimSpace(reManyNL.ReplaceAllString(clean, "\n\n"))
	for _, marker := range cutMarkers {
		if p := strings.Index(clean, marker); p >= 0 {
			if len(clean) > p {
				clean = strings.TrimRight(clean[:p], " \n\t")
			}
		}
	}
	r.Text = clean
	r.AllLinks = allLinks

	lessonIDAttr := ""
	if m := reLessonID.FindStringSubmatch(raw); m != nil {
		lessonIDAttr = strings.TrimSpace(m[1])
	}
	var lessonHosts []string
	lessonIDs := make(map[string]bool)

	addHostID := func(host, lid string) {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" && !contains(lessonHosts, host) {
			lessonHosts = append(lessonHosts, host)
		}
		if lid != "" {
			lessonIDs[lid] = true
		}
	}

	for _, vm := range reLessonView.FindAllStringSubmatch(raw, -1) {
		u, _ := url.Parse(vm[0])
		if u != nil {
			addHostID(u.Hostname(), u.Query().Get("id"))
		}
	}
	for _, hm := range reLessonHref.FindAllStringSubmatch(raw, -1) {
		joinU := joinURL(baseURL, strings.TrimSpace(hm[1]))
		u, _ := url.Parse(joinU)
		if u != nil && strings.Contains(strings.ToLower(joinU), "/pl/teach/control/lesson/view") {
			addHostID(u.Hostname(), u.Query().Get("id"))
		}
	}
	for _, abs := range allLinks {
		if !strings.Contains(strings.ToLower(abs), "/pl/teach/control/lesson/view") {
			continue
		}
		u, _ := url.Parse(abs)
		if u != nil {
			addHostID(u.Hostname(), u.Query().Get("id"))
		}
	}

	lessonIDEff := lessonIDAttr
	if lessonIDEff == "" && len(lessonIDs) > 0 {
		ids := make([]int, 0, len(lessonIDs))
		for id := range lessonIDs {
			if n, err := strconv.Atoi(id); err == nil {
				ids = append(ids, n)
			}
		}
		if len(ids) > 0 {
			sort.Ints(ids)
			lessonIDEff = strconv.Itoa(ids[0])
		}
	}
	r.LessonID = lessonIDEff

	portalHost := ""
	if baseURL != "" {
		if u, err := url.Parse(baseURL); err == nil {
			portalHost = strings.ToLower(u.Hostname())
		}
	}
	if portalHost == "" && len(lessonHosts) > 0 {
		portalHost = lessonHosts[0]
	}
	if portalHost == "" {
		for u := range urlToName {
			if strings.Contains(strings.ToLower(u), "/pl/fileservice/") {
				if pu, err := url.Parse(u); err == nil && pu.Hostname() != "" {
					portalHost = strings.ToLower(pu.Hostname())
					break
				}
			}
		}
	}

	scheme := "https"
	if baseURL != "" {
		if u, err := url.Parse(baseURL); err == nil && u.Scheme != "" {
			scheme = u.Scheme
		}
	}
	if portalHost != "" && lessonIDEff != "" {
		r.SuggestedReferer = scheme + "://" + portalHost + "/pl/teach/control/lesson/view?id=" + lessonIDEff
	}

	seenWrite := make(map[string]bool)
	for u, name := range urlToName {
		if seenWrite[u] {
			continue
		}
		seenWrite[u] = true
		dn := FilenameWithURLExt(name, u)
		if dn == "" {
			dn = FilenameWithURLExt(basenameLabel(u), u)
		}
		r.Files = append(r.Files, FileEntry{URL: u, Name: dn})
	}
	sort.Slice(r.Files, func(i, j int) bool { return r.Files[i].URL < r.Files[j].URL })
	return r
}

func ParseHTMLFile(path, baseURL string) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseHTML(string(data), baseURL), nil
}

func pickFirst(raw string, patterns ...*regexp.Regexp) string {
	for _, p := range patterns {
		if m := p.FindStringSubmatch(raw); m != nil {
			return html.UnescapeString(strings.TrimSpace(m[1]))
		}
	}
	return ""
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		s = "lesson"
	}
	s = reBadName.ReplaceAllString(s, "_")
	s = strings.Trim(reMultiSpace.ReplaceAllString(s, " "), " .")
	if len(s) > 140 {
		s = s[:140]
	}
	if s == "" {
		return "lesson"
	}
	return s
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\u00a0", " ")
	if s == "" {
		s = "download"
	}
	base := path.Base(strings.ReplaceAll(s, "\\", "/"))
	stem, ext := splitExt(base)
	stem = strings.Trim(strings.Trim(stem, "."), " ")
	if stem == "" {
		stem = "file"
	}
	stem = reBadName.ReplaceAllString(stem, "_")
	stem = strings.Trim(reMultiSpace.ReplaceAllString(stem, " "), ".")
	if ext != "" && len(ext) <= 13 && ext[0] == '.' {
		out := stem + ext
		if len(out) > 220 {
			out = stem[:max(1, 220-len(ext))] + ext
		}
		return out
	}
	stem = reBadName.ReplaceAllString(base, "_")
	stem = strings.Trim(reMultiSpace.ReplaceAllString(stem, " "), ".")
	if stem == "" {
		stem = "download"
	}
	if len(stem) > 180 {
		stem = stem[:180]
	}
	return stem
}

func splitExt(base string) (stem, ext string) {
	i := strings.LastIndex(base, ".")
	if i <= 0 {
		return base, ""
	}
	return base[:i], base[i:]
}

func basenameLabel(u string) string {
	u = strings.SplitN(u, "?", 2)[0]
	return path.Base(u)
}

func normalizeMediaURL(rawURL, baseURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, "//") && baseURL != "" {
		if u, err := url.Parse(baseURL); err == nil && u.Scheme != "" {
			rawURL = u.Scheme + ":" + rawURL
		}
	}
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	if baseURL == "" {
		return rawURL
	}
	bu, err := url.Parse(baseURL)
	if err != nil {
		return rawURL
	}
	ru, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return bu.ResolveReference(ru).String()
}

func joinURL(base, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	if base == "" {
		return ref
	}
	bu, err := url.Parse(base)
	if err != nil {
		return ref
	}
	ru, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return bu.ResolveReference(ru).String()
}

func isLessonAttachmentURL(u string) bool {
	low := strings.ToLower(u)
	if strings.Contains(low, "/fileservice/user/file/download/") {
		return true
	}
	if strings.Contains(low, "/fileservice/file/download/") && !strings.Contains(low, "/thumbnail/") {
		return true
	}
	if strings.Contains(low, "/pl/fileservice/user/file/download/") {
		return true
	}
	return strings.Contains(low, "/file/download/") && strings.Contains(low, "fileservice")
}

func anchorLabel(inner string) string {
	t := reStripTags.ReplaceAllString(inner, " ")
	return strings.TrimSpace(html.UnescapeString(t))
}

func displayNameFromMediaURL(rawURL, baseURL, fallback string) string {
	abs := normalizeMediaURL(rawURL, baseURL)
	bn := basenameLabel(abs)
	stem, ext := splitExt(bn)
	if ext != "" && knownMediaExt[strings.TrimPrefix(strings.ToLower(ext), ".")] && stem != "" {
		return bn
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return bn
}

func looksLikeHashedName(name string) bool {
	base := basenameLabel(name)
	stem, ext := splitExt(base)
	if ext == "" || len(stem) < 16 {
		return false
	}
	e := strings.TrimPrefix(strings.ToLower(ext), ".")
	if e != "mp3" && e != "m4a" && e != "wav" && e != "ogg" && e != "oga" && e != "webm" && e != "mp4" && e != "pdf" && e != "zip" {
		return false
	}
	core := strings.Split(strings.Split(stem, "_")[0], "-")[0]
	if len(core) < 16 {
		return false
	}
	return reHexHash.MatchString(core)
}

func wantsBetterName(current, candidate string) bool {
	if candidate == "" {
		return false
	}
	if current == "" {
		return true
	}
	candIsRaw := basenameLabel(candidate) == strings.TrimSpace(candidate)
	curHash := looksLikeHashedName(current)
	if candIsRaw {
		return curHash
	}
	if strings.EqualFold(basenameLabel(current), basenameLabel(candidate)) {
		return false
	}
	return true
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func parseJPlayer(raw, baseURL string, addMedia func(string, string)) {
	for _, loc := range reJPlayer.FindAllStringIndex(raw, -1) {
		p0 := loc[1]
		body, ok := extractParenBody(raw, p0-1)
		if !ok {
			continue
		}
		arrBody := extractSecondArrayArg(body)
		if arrBody == "" {
			continue
		}
		for _, track := range extractBalanced(arrBody, '{', '}') {
			title := ""
			if tm := reTitleJS.FindStringSubmatch(track); tm != nil {
				title = html.UnescapeString(strings.TrimSpace(tm[1]))
			}
			if mp3 := extractJSProp(track, "mp3"); mp3 != "" {
				addMedia(mp3, displayNameFromMediaURL(mp3, baseURL, title))
				continue
			}
			for _, key := range []string{"m4a", "oga", "wav", "webm", "mp4", "ogg"} {
				if v := extractJSProp(track, key); v != "" {
					lbl := title
					if lbl == "" {
						lbl = "media." + key
					}
					addMedia(v, displayNameFromMediaURL(v, baseURL, lbl))
					break
				}
			}
		}
	}
}

func extractJSProp(s, key string) string {
	re := regexp.MustCompile(`(?is)\b` + regexp.QuoteMeta(key) + `\s*:\s*["']([^"']+)["']`)
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func extractParenBody(s string, openIdx int) (string, bool) {
	if openIdx < 0 || openIdx >= len(s) || s[openIdx] != '(' {
		return "", false
	}
	depth := 0
	for i := openIdx; i < len(s); i++ {
		switch s[i] {
		case '"', '\'':
			i = skipString(s, i)
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[openIdx+1 : i], true
			}
		}
	}
	return "", false
}

func skipString(s string, i int) int {
	q := s[i]
	i++
	for i < len(s) {
		if s[i] == '\\' {
			i += 2
			continue
		}
		if s[i] == q {
			return i
		}
		i++
	}
	return len(s) - 1
}

func extractSecondArrayArg(body string) string {
	depth := 0
	commas := []int{}
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '"' || c == '\'' {
			i = skipString(body, i)
			continue
		}
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 1 {
				commas = append(commas, i)
			}
		}
	}
	if len(commas) < 2 {
		return ""
	}
	arrStart := commas[1] + 1
	rest := strings.TrimSpace(body[arrStart:])
	if !strings.HasPrefix(rest, "[") {
		return ""
	}
	inner, ok := extractBracketInner(rest, 0)
	if !ok {
		return ""
	}
	return inner
}

func extractBracketInner(s string, start int) (string, bool) {
	if start >= len(s) || s[start] != '[' {
		return "", false
	}
	depth := 0
	bodyStart := 0
	for i := start; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\'' {
			i = skipString(s, i)
			continue
		}
		if c == '[' {
			if depth == 0 {
				bodyStart = i + 1
			}
			depth++
		} else if c == ']' {
			depth--
			if depth == 0 {
				return s[bodyStart:i], true
			}
		}
	}
	return "", false
}

func extractBalanced(s string, open, close byte) []string {
	var out []string
	depth := 0
	start := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\'' {
			i = skipString(s, i)
			continue
		}
		if c == open {
			if depth == 0 {
				start = i + 1
			}
			depth++
		} else if c == close {
			depth--
			if depth == 0 && start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		}
	}
	return out
}

func findBraceObjects(raw string) []string {
	// simple shallow scan — same idea as python fallback
	var objs []string
	re := regexp.MustCompile(`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
	for _, m := range re.FindAllString(raw, -1) {
		if strings.Contains(m, "mp3") || strings.Contains(m, "m4a") || strings.Contains(m, "title") {
			objs = append(objs, m)
		}
	}
	return objs
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
