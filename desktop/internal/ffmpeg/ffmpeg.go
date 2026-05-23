package ffmpeg

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/getcourse-downloader/getcourse-downloader/internal/config"
)

// InstallHint tells the extension how to obtain ffmpeg on the desktop OS.
type InstallHint struct {
	Title       string   `json:"title"`
	DownloadURL string   `json:"download_url,omitempty"`
	DocURL      string   `json:"doc_url,omitempty"`
	Steps       []string `json:"steps"`
}

// Status is returned in GET /health for the extension UI.
type Status struct {
	Found          bool   `json:"found"`
	Path           string `json:"path,omitempty"`
	AppDir         string `json:"app_dir"`
	ExpectedBinary string `json:"expected_binary"`
	Local          bool   `json:"local"`
}

// Find returns ffmpeg path and whether it lives next to the app binary.
func Find() (path string, local bool) {
	dir := config.AppDir()
	names := []string{"ffmpeg.exe", "ffmpeg"}
	if runtime.GOOS != "windows" {
		names = []string{"ffmpeg", "ffmpeg.exe"}
	}
	for _, name := range names {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, false
	}
	return "", false
}

// StatusForHealth builds ffmpeg status and install hint for /health.
func StatusForHealth() (Status, string, *InstallHint) {
	appDir := config.AppDir()
	expected := "ffmpeg"
	if runtime.GOOS == "windows" {
		expected = "ffmpeg.exe"
	}
	path, local := Find()
	st := Status{
		Found:          path != "",
		Path:           path,
		AppDir:         appDir,
		ExpectedBinary: expected,
		Local:          local,
	}
	if path != "" {
		return st, runtime.GOOS, nil
	}
	return st, runtime.GOOS, installHint(runtime.GOOS, appDir, expected)
}

func installHint(goos, appDir, expected string) *InstallHint {
	place := "положите `" + expected + "` в папку:\n" + appDir
	switch goos {
	case "windows":
		return &InstallHint{
			Title:       "Скачайте ffmpeg для Windows",
			DownloadURL: "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip",
			Steps: []string{
				"Скачайте ZIP (кнопка ниже) и распакуйте.",
				"Из папки `bin` скопируйте `ffmpeg.exe` рядом с `GetCourseDownloader.exe`.",
				place,
				"Перезапустите GetCourseDownloader.exe.",
			},
		}
	case "darwin":
		return &InstallHint{
			Title:       "Установите ffmpeg на macOS",
			DownloadURL: "https://evermeet.cx/ffmpeg/",
			Steps: []string{
				"Вариант A: `brew install ffmpeg` (если ffmpeg в PATH — достаточно).",
				"Вариант B: скачайте бинарник с evermeet.cx и " + place,
				"Перезапустите GetCourseDownloader.",
			},
		}
	default:
		return &InstallHint{
			Title: "Установите ffmpeg",
			Steps: []string{
				"Установите пакет ffmpeg в системе (apt/dnf/pacman) или " + place,
				"Перезапустите GetCourseDownloader.",
			},
		}
	}
}
