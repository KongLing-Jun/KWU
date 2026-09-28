package automation

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestExportRunResultSupportsUserFormats(t *testing.T) {
	store := NewStore(t.TempDir())
	task := validStoreTask()
	task.Name = "Daily Result"
	manifest, _, err := store.CreateRun(task, string(TriggerManual), "desktop:123")
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	result := "# Result\n\n## Section\n\n- **Answer**: ok & <safe>\n\n> quoted\n\nplain [link](https://example.test)"
	if err := store.WriteResult(task.ID, manifest.RunID, result); err != nil {
		t.Fatalf("WriteResult returned error: %v", err)
	}
	svc := newService(store, nil)

	tests := []struct {
		format string
		ext    string
		entry  string
		main   string
		want   string
	}{
		{format: "md", ext: ".md", want: result},
		{format: "txt", ext: ".txt", want: "Answer: ok & <safe>"},
		{format: "json", ext: ".json", want: `"blocks"`},
		{format: "docx", ext: ".docx", entry: "word/document.xml", main: "word/document.xml", want: `<w:pStyle w:val="Heading1"`},
		{format: "xlsx", ext: ".xlsx", entry: "xl/worksheets/sheet1.xml", main: "xl/workbook.xml", want: "<t xml:space=\"preserve\">type</t>"},
		{format: "pptx", ext: ".pptx", entry: "ppt/slides/slide1.xml", main: "ppt/presentation.xml", want: "<a:t>Result</a:t>"},
		{format: "ppt", ext: ".pptx", entry: "ppt/slides/slide1.xml", main: "ppt/presentation.xml", want: "<a:t>Result</a:t>"},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got, err := svc.ExportRunResult(context.Background(), RunResultExportRequest{
				TaskID: task.ID,
				RunID:  manifest.RunID,
				Format: tt.format,
			})
			if err != nil {
				t.Fatalf("ExportRunResult returned error: %v", err)
			}
			if !strings.HasSuffix(got.Filename, tt.ext) || got.MimeType == "" || !got.Base64Encoded {
				t.Fatalf("export metadata = %+v", got)
			}
			payload, err := base64.StdEncoding.DecodeString(got.Payload)
			if err != nil {
				t.Fatalf("payload base64 decode: %v", err)
			}
			body := string(payload)
			if tt.entry != "" {
				assertValidOfficePackage(t, payload, tt.main)
				body = readExportZipEntry(t, payload, tt.entry)
			}
			if !strings.Contains(body, tt.want) {
				t.Fatalf("payload body = %q, want to contain %q", body, tt.want)
			}
			if tt.format != "md" && strings.Contains(body, "**Answer**") {
				t.Fatalf("%s export leaked markdown emphasis: %q", tt.format, body)
			}
		})
	}
}

func TestExportRunResultRejectsUnknownFormat(t *testing.T) {
	store := NewStore(t.TempDir())
	task := validStoreTask()
	manifest, _, err := store.CreateRun(task, string(TriggerManual), "desktop:123")
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := store.WriteResult(task.ID, manifest.RunID, "ok"); err != nil {
		t.Fatalf("WriteResult returned error: %v", err)
	}
	svc := newService(store, nil)

	if _, err := svc.ExportRunResult(context.Background(), RunResultExportRequest{
		TaskID: task.ID,
		RunID:  manifest.RunID,
		Format: "exe",
	}); err == nil {
		t.Fatal("ExportRunResult returned nil error for unknown format")
	}
}

func assertValidOfficePackage(t *testing.T, payload []byte, mainPart string) {
	t.Helper()
	entries := readExportZipEntries(t, payload)
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", mainPart} {
		if _, ok := entries[name]; !ok {
			t.Fatalf("office package missing %s", name)
		}
	}
	if !strings.Contains(entries["[Content_Types].xml"], `PartName="/`+mainPart+`"`) {
		t.Fatalf("[Content_Types].xml missing override for %s", mainPart)
	}
	if !strings.Contains(entries["_rels/.rels"], `Target="`+mainPart+`"`) {
		t.Fatalf("_rels/.rels missing officeDocument target %s", mainPart)
	}
	for name, body := range entries {
		if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels") {
			assertWellFormedXML(t, name, body)
		}
	}
}

func readExportZipEntry(t *testing.T, payload []byte, name string) string {
	t.Helper()
	entries := readExportZipEntries(t, payload)
	if body, ok := entries[name]; ok {
		return body
	}
	t.Fatalf("zip entry %s not found", name)
	return ""
}

func readExportZipEntries(t *testing.T, payload []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	entries := make(map[string]string, len(zr.File))
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", file.Name, err)
		}
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(rc); err != nil {
			_ = rc.Close()
			t.Fatalf("read zip entry %s: %v", file.Name, err)
		}
		if err := rc.Close(); err != nil {
			t.Fatalf("close zip entry %s: %v", file.Name, err)
		}
		entries[file.Name] = buf.String()
	}
	return entries
}

func assertWellFormedXML(t *testing.T, name, body string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(body))
	for {
		if _, err := dec.Token(); err == io.EOF {
			return
		} else if err != nil {
			t.Fatalf("%s is not well-formed XML: %v", name, err)
		}
	}
}
