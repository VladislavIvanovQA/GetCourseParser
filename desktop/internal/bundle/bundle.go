package bundle

// LessonPayload is sent by the browser extension.
type LessonPayload struct {
	Referer      string   `json:"referer"`
	Origin       string   `json:"origin"`
	BaseURL      string   `json:"base_url"`
	VideoReferer string   `json:"video_referer"`
	VideoOrigin  string   `json:"video_origin"`
	Cookie       string   `json:"cookie"`
	VideoAuth    string   `json:"video_auth"`
	HTML           string `json:"html"`
	VideoURLs      []string `json:"video_urls"`
	PagePDFBase64  string   `json:"page_pdf_base64,omitempty"`
	SaveLessonText bool     `json:"save_lesson_text"`
	Async          bool     `json:"async,omitempty"`
}

type ProcessResult struct {
	OK          bool     `json:"ok"`
	Folder      string   `json:"folder"`
	Title       string   `json:"title"`
	FilesSaved  int      `json:"files_saved"`
	VideosSaved int      `json:"videos_saved"`
	PDFSaved    bool     `json:"pdf_saved"`
	Errors      []string `json:"errors,omitempty"`
}
