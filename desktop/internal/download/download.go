package download

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/getcourse-downloader/getcourse-downloader/internal/bundle"
	"github.com/getcourse-downloader/getcourse-downloader/internal/config"
	"github.com/getcourse-downloader/getcourse-downloader/internal/ffmpeg"
	m3u8parse "github.com/getcourse-downloader/getcourse-downloader/internal/m3u8"
	"github.com/getcourse-downloader/getcourse-downloader/internal/parser"
)

const defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

type Auth struct {
	Referer   string
	Origin    string
	Cookie    string
	VideoAuth string
	UserAgent string
}

func ProcessLesson(cfg *config.Config, payload *bundle.LessonPayload) (*bundle.ProcessResult, error) {
	return processLesson(cfg, payload, nil)
}

type ProgressFunc func(percent int, message string)

func ProcessLessonWithProgress(cfg *config.Config, payload *bundle.LessonPayload, onProgress ProgressFunc) (*bundle.ProcessResult, error) {
	return processLesson(cfg, payload, onProgress)
}

func processLesson(cfg *config.Config, payload *bundle.LessonPayload, report ProgressFunc) (*bundle.ProcessResult, error) {
	if report == nil {
		report = func(int, string) {}
	}
	if strings.TrimSpace(payload.HTML) == "" {
		return nil, fmt.Errorf("empty html")
	}

	baseURL := strings.TrimSpace(payload.BaseURL)
	if baseURL == "" && payload.Referer != "" {
		if u, err := url.Parse(payload.Referer); err == nil {
			baseURL = u.Scheme + "://" + u.Host
		}
	}

	report(5, "Разбор страницы…")
	parsed := parser.ParseHTML(payload.HTML, baseURL)

	auth := Auth{
		Referer:   strings.TrimSpace(payload.Referer),
		Origin:    strings.TrimSpace(payload.Origin),
		Cookie:    strings.TrimSpace(payload.Cookie),
		VideoAuth: strings.TrimSpace(payload.VideoAuth),
		UserAgent: defaultUA,
	}
	if auth.Referer == "" {
		auth.Referer = parsed.SuggestedReferer
	}
	if auth.Origin == "" && auth.Referer != "" {
		if u, err := url.Parse(auth.Referer); err == nil {
			auth.Origin = u.Scheme + "://" + u.Host
		}
	}

	parent := cfg.DownloadsPath()
	targetDir := filepath.Join(parent, parsed.FolderName)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, err
	}

	res := &bundle.ProcessResult{
		OK:     true,
		Folder: targetDir,
		Title:  parsed.Title,
	}

	if b64 := strings.TrimSpace(payload.PagePDFBase64); b64 != "" {
		report(15, "Сохранение PDF…")
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			res.Errors = append(res.Errors, "lesson.pdf decode: "+err.Error())
		} else if err := os.WriteFile(filepath.Join(targetDir, "lesson.pdf"), data, 0o644); err != nil {
			res.Errors = append(res.Errors, "lesson.pdf: "+err.Error())
		} else {
			res.PDFSaved = true
		}
	}

	lessonTxt := filepath.Join(targetDir, "lesson.txt")
	linksTxt := filepath.Join(targetDir, "links.txt")
	if payload.SaveLessonText {
		var sb strings.Builder
		fmt.Fprintf(&sb, "Lesson: %s\n\n=== TEXT ===\n\n%s\n\n=== LINKS ===\n", parsed.Title, parsed.Text)
		if len(parsed.AllLinks) == 0 {
			sb.WriteString("(no links found)\n")
		} else {
			for _, l := range parsed.AllLinks {
				sb.WriteString(l)
				sb.WriteByte('\n')
			}
		}
		if err := os.WriteFile(lessonTxt, []byte(sb.String()), 0o644); err != nil {
			res.Errors = append(res.Errors, "lesson.txt: "+err.Error())
		}
	}

	var linksContent strings.Builder
	for _, l := range parsed.AllLinks {
		linksContent.WriteString(l)
		linksContent.WriteByte('\n')
	}
	_ = os.WriteFile(linksTxt, []byte(linksContent.String()), 0o644)

	fileTotal := len(parsed.Files)
	for i, fe := range parsed.Files {
		if fileTotal > 0 {
			report(20+(i*30)/max(fileTotal, 1), fmt.Sprintf("Файл %d/%d…", i+1, fileTotal))
		}
		filename := parser.FilenameWithURLExt(fe.Name, fe.URL)
		target := filepath.Join(targetDir, filename)
		if _, err := os.Stat(target); err == nil {
			target = filepath.Join(targetDir, fmt.Sprintf("%d_%s", i+1, filename))
		}
		if err := downloadFile(auth, fe.URL, target); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("file %s: %v", filename, err))
		} else {
			res.FilesSaved++
		}
	}

	vAuth := auth
	if strings.TrimSpace(payload.VideoReferer) != "" {
		vAuth.Referer = strings.TrimSpace(payload.VideoReferer)
	}
	if strings.TrimSpace(payload.VideoOrigin) != "" {
		vAuth.Origin = strings.TrimSpace(payload.VideoOrigin)
	}

	videos := uniqueStrings(payload.VideoURLs)
	if len(videos) > 0 {
		h := http.Header{}
		h.Set("User-Agent", vAuth.UserAgent)
		h.Set("Accept", "*/*")
		if vAuth.Referer != "" {
			h.Set("Referer", vAuth.Referer)
		}
		if vAuth.Origin != "" {
			h.Set("Origin", vAuth.Origin)
		}
		if vAuth.Cookie != "" {
			h.Set("Cookie", vAuth.Cookie)
		}
		if vAuth.VideoAuth != "" {
			auth := vAuth.VideoAuth
			if !strings.HasPrefix(auth, "Bearer ") && !strings.HasPrefix(auth, "Basic ") {
				auth = "Bearer " + auth
			}
			h.Set("Authorization", auth)
		}
		client := &http.Client{Timeout: 2 * time.Minute}
		videos = m3u8parse.ExpandMasters(client, h, videos)
	}

	vn := 0
	videoTotal := len(videos)
	for _, vurl := range videos {
		vurl = strings.TrimSpace(vurl)
		if vurl == "" {
			continue
		}
		vn++
		report(50+(vn*45)/max(videoTotal, 1), fmt.Sprintf("Видео %d/%d…", vn, videoTotal))
		out := filepath.Join(targetDir, fmt.Sprintf("video_%02d.mp4", vn))
		if err := downloadVideo(vAuth, vurl, out); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("video %s: %v", vurl, err))
			_ = os.Remove(out)
		} else {
			res.VideosSaved++
		}
	}

	if len(videos) == 0 && strings.Contains(strings.ToLower(payload.HTML), "m3u8") {
		res.Errors = append(res.Errors, "no video URLs from extension — play the video on the page and try again")
	}

	report(100, "Готово")
	return res, nil
}

func downloadFile(auth Auth, fileURL, target string) error {
	req, err := http.NewRequest(http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}
	applyHeaders(req, auth, fileURL, false)
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func downloadVideo(auth Auth, streamURL, output string) error {
	ffPath, _ := ffmpeg.Find()
	if ffPath == "" {
		return fmt.Errorf("ffmpeg not found next to %s (or in PATH)", config.AppDir())
	}

	headers := buildFFmpegHeaders(auth)
	tmp, err := os.CreateTemp("", "gc_plist_*.m3u8")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	if err := prefetchPlaylist(auth, streamURL, tmpPath); err == nil {
		if isM3U8File(tmpPath) {
			if err := runFFmpeg(ffPath, tmpPath, output, headers, auth.UserAgent, true); err == nil {
				return nil
			}
		}
	}

	return runFFmpeg(ffPath, streamURL, output, headers, auth.UserAgent, needsHLSDemuxer(streamURL))
}

// Kinescope / gceuproxy: playlists without .m3u8 and segments as .bin
func needsHLSDemuxer(streamURL string) bool {
	low := strings.ToLower(streamURL)
	if strings.Contains(low, ".m3u8") {
		return true
	}
	return strings.Contains(low, "/api/playlist/") ||
		strings.Contains(low, "gceuproxy.com") ||
		strings.Contains(low, "kinescopecdn.net")
}

func prefetchPlaylist(auth Auth, streamURL, dest string) error {
	req, err := http.NewRequest(http.MethodGet, streamURL, nil)
	if err != nil {
		return err
	}
	applyHeaders(req, auth, streamURL, true)
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func isM3U8File(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := string(data)
	return strings.Contains(s, "#EXTM3U") || strings.Contains(s, "#EXT-X-STREAM-INF")
}

func runFFmpeg(ffmpeg, input, output, headers, ua string, forceHLS bool) error {
	args := []string{
		"-loglevel", "warning", "-stats",
		"-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5",
		"-user_agent", ua,
	}
	if headers != "" {
		args = append(args, "-headers", headers)
	}
	if forceHLS {
		// gceuproxy/Kinescope: media playlists and .bin segments
		args = append(args,
			"-extension_picky", "0",
			"-allowed_extensions", "ALL",
			"-allowed_segment_extensions", "ALL",
			"-f", "hls",
		)
	}
	args = append(args,
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
		"-i", input,
		"-c", "copy",
		"-movflags", "+faststart",
		output,
	)
	cmd := exec.Command(ffmpeg, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildFFmpegHeaders(auth Auth) string {
	var b strings.Builder
	b.WriteString("Accept: */*\r\n")
	b.WriteString("Accept-Language: ru-RU,ru;q=0.9,en-US;q=0.8\r\n")
	b.WriteString("Cache-Control: no-cache\r\n")
	b.WriteString("Pragma: no-cache\r\n")
	if auth.Origin != "" {
		fmt.Fprintf(&b, "Origin: %s\r\n", auth.Origin)
	}
	if auth.Referer != "" {
		fmt.Fprintf(&b, "Referer: %s\r\n", auth.Referer)
	}
	if auth.VideoAuth != "" {
		v := auth.VideoAuth
		if !strings.HasPrefix(v, "Bearer ") && !strings.HasPrefix(v, "Basic ") {
			v = "Bearer " + v
		}
		fmt.Fprintf(&b, "Authorization: %s\r\n", v)
	}
	if auth.Cookie != "" {
		fmt.Fprintf(&b, "Cookie: %s\r\n", auth.Cookie)
	}
	return b.String()
}

func applyHeaders(req *http.Request, auth Auth, requestURL string, video bool) {
	if auth.UserAgent == "" {
		auth.UserAgent = defaultUA
	}
	req.Header.Set("User-Agent", auth.UserAgent)
	if video || strings.Contains(strings.ToLower(requestURL), "/fileservice/") || strings.Contains(strings.ToLower(requestURL), "/file/download/") {
		req.Header.Set("Accept", "*/*")
	} else {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	}
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("DNT", "1")
	if auth.Referer != "" {
		req.Header.Set("Referer", auth.Referer)
	}
	if auth.Origin != "" {
		req.Header.Set("Origin", auth.Origin)
	} else if auth.Referer != "" {
		if u, err := url.Parse(auth.Referer); err == nil {
			req.Header.Set("Origin", u.Scheme+"://"+u.Host)
		}
	}
	if auth.Cookie != "" {
		req.Header.Set("Cookie", auth.Cookie)
	}
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
