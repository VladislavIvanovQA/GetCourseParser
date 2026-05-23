package parser

import "testing"

func TestFilenameWithURLExt(t *testing.T) {
	url := "https://school.example.getcourse.ru/pl/fileservice/user/file/download/h/abc123def456.pdf"
	got := FilenameWithURLExt("Конспект. Анализ парковки", url)
	want := "Конспект. Анализ парковки.pdf"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	got = FilenameWithURLExt("Чек-лист парковка.pdf", url)
	if got != "Чек-лист парковка.pdf" {
		t.Fatalf("double ext: %q", got)
	}

	got = FilenameWithURLExt("doc.pdf", "https://x.com/f/file.zip")
	if got != "doc.zip" {
		t.Fatalf("url ext wins: got %q", got)
	}

	got = FilenameWithURLExt("report", "https://x.com/a/b/report.PDF")
	if got != "report.pdf" {
		t.Fatalf("got %q want report.pdf", got)
	}
}
