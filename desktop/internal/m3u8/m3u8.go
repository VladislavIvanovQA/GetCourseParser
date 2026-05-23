package m3u8

import (
	"bufio"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	reBandwidth  = regexp.MustCompile(`(?i)BANDWIDTH=(\d+)`)
	reResolution = regexp.MustCompile(`(?i)RESOLUTION=(\d+x\d+)`)
	reMaster     = regexp.MustCompile(`(?i)/api/playlist/master/`)
)

type Variant struct {
	URL        string
	Bandwidth  int
	Resolution string
}

func IsMasterPlaylistURL(u string) bool {
	return reMaster.MatchString(u)
}

func ParseVariants(text, baseURL string) []Variant {
	var out []Variant
	sc := bufio.NewScanner(strings.NewReader(text))
	var bw int
	var res string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
			if m := reBandwidth.FindStringSubmatch(line); m != nil {
				bw, _ = strconv.Atoi(m[1])
			}
			if m := reResolution.FindStringSubmatch(line); m != nil {
				res = m[1]
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		abs := resolveURL(line, baseURL)
		if abs == "" {
			continue
		}
		out = append(out, Variant{URL: abs, Bandwidth: bw, Resolution: res})
		bw = 0
		res = ""
	}
	return out
}

func resolveURL(ref, base string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "#") {
		return ""
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	bu, err := url.Parse(base)
	if err != nil {
		return ref
	}
	ru, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return bu.ResolveReference(ru).String()
}

func streamKey(u string) string {
	pu, err := url.Parse(u)
	if err != nil {
		return u
	}
	path := pu.Path
	if i := strings.Index(path, "/media/"); i >= 0 {
		rest := path[i+len("/media/"):]
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			return pu.Host + "|" + parts[0] + "|" + parts[1]
		}
	}
	if i := strings.Index(path, "/master/"); i >= 0 {
		rest := path[i+len("/master/"):]
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			return pu.Host + "|master|" + parts[0] + "|" + parts[1]
		}
	}
	return pu.Host + "|" + path
}

func variantScore(v Variant) int {
	score := v.Bandwidth
	if v.Resolution != "" {
		parts := strings.Split(v.Resolution, "x")
		if len(parts) == 2 {
			w, _ := strconv.Atoi(parts[0])
			h, _ := strconv.Atoi(parts[1])
			score += w * h
		}
	}
	return score
}

func PickBest(variants []Variant) []Variant {
	best := make(map[string]Variant)
	for _, v := range variants {
		k := streamKey(v.URL)
		if prev, ok := best[k]; !ok || variantScore(v) > variantScore(prev) {
			best[k] = v
		}
	}
	out := make([]Variant, 0, len(best))
	for _, v := range best {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}

func resolutionFromMediaPath(u string) int {
	re := regexp.MustCompile(`/(\d{3,4})(?:\?|$|/)`)
	m := re.FindStringSubmatch(u)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// SelectVideoInputs keeps one master (or best media) per video stream.
func SelectVideoInputs(urls []string) []string {
	seenMaster := make(map[string]bool)
	var masters []string
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" || !IsMasterPlaylistURL(u) {
			continue
		}
		k := streamKey(u)
		if seenMaster[k] {
			continue
		}
		seenMaster[k] = true
		masters = append(masters, u)
	}
	if len(masters) > 0 {
		return masters
	}
	bestMedia := make(map[string]struct {
		url   string
		score int
	})
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if !strings.Contains(strings.ToLower(u), "/api/playlist/media/") {
			continue
		}
		k := streamKey(u)
		sc := resolutionFromMediaPath(u)
		if prev, ok := bestMedia[k]; !ok || sc > prev.score {
			bestMedia[k] = struct {
				url   string
				score int
			}{u, sc}
		}
	}
	out := make([]string, 0, len(bestMedia))
	for _, v := range bestMedia {
		out = append(out, v.url)
	}
	sort.Strings(out)
	return out
}

// ExpandMasters fetches master playlists and returns best media URL per stream.
func ExpandMasters(client *http.Client, headers http.Header, urls []string) []string {
	urls = SelectVideoInputs(urls)
	var all []Variant
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !IsMasterPlaylistURL(u) {
			all = append(all, Variant{URL: u})
			continue
		}
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			all = append(all, Variant{URL: u})
			continue
		}
		for k, vv := range headers {
			for _, v := range vv {
				req.Header.Add(k, v)
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			all = append(all, Variant{URL: u})
			continue
		}
		body := resp.Body
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body.Close()
			all = append(all, Variant{URL: u})
			continue
		}
		var b strings.Builder
		sc := bufio.NewScanner(body)
		for sc.Scan() {
			b.WriteString(sc.Text())
			b.WriteByte('\n')
		}
		body.Close()
		text := b.String()
		if !strings.Contains(text, "#EXTM3U") {
			all = append(all, Variant{URL: u})
			continue
		}
		all = append(all, ParseVariants(text, u)...)
	}
	picked := PickBest(all)
	out := make([]string, len(picked))
	for i, v := range picked {
		out[i] = v.URL
	}
	return out
}
