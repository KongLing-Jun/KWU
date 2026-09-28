package builtin

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeOfficeZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
}

func TestReadFileOfficeDocxExtractsText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brief.docx")
	writeOfficeZip(t, path, map[string]string{
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:r><w:t>Header text</w:t></w:r></w:p></w:hdr>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:r><w:t>Project brief</w:t></w:r></w:p>
<w:tbl><w:tr><w:tc><w:p><w:r><w:t>Name</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Reasonix</w:t></w:r></w:p></w:tc></w:tr></w:tbl>
</w:body></w:document>`,
		"word/footer1.xml": `<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:r><w:t>Footer text</w:t></w:r></w:p></w:ftr>`,
	})

	out := runTool(t, readFile{}, map[string]any{"path": path})
	for _, want := range []string{
		"[office extract: docx]",
		"# Document",
		"Project brief",
		"Name\tReasonix",
		"Header text",
		"Footer text",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "1→") {
		t.Fatalf("Office output should be line-numbered by read_file:\n%s", out)
	}
}

func TestReadFileOfficeXLSXExtractsSheetsCellsAndFormulaText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "budget.xlsx")
	writeOfficeZip(t, path, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Summary" sheetId="1" r:id="rId1"/><sheet name="Data" sheetId="2" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="worksheet" Target="worksheets/sheet2.xml"/></Relationships>`,
		"xl/sharedStrings.xml":       `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>Label</t></si><si><t>Total</t></si><si><t>North</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>
<row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2"><v>42</v></c></row>
</sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>
<row r="1"><c r="A1"><f>SUM(Summary!B2:B2)</f></c></row>
</sheetData></worksheet>`,
	})

	out := runTool(t, readFile{}, map[string]any{"path": path})
	for _, want := range []string{
		"[office extract: xlsx]",
		"# Sheet: Summary",
		"Label\tTotal",
		"North\t42",
		"# Sheet: Data",
		"[formula not calculated: SUM(Summary!B2:B2)]",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestReadFileOfficePPTXExtractsSlides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.pptx")
	writeOfficeZip(t, path, map[string]string{
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rId1"/><p:sldId id="257" r:id="rId2"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="slide" Target="slides/slide1.xml"/><Relationship Id="rId2" Type="slide" Target="slides/slide2.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Intro</a:t></a:r></a:p><a:p><a:r><a:t>First point</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/slide2.xml":           `<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Decision</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})

	out := runTool(t, readFile{}, map[string]any{"path": path})
	for _, want := range []string{
		"[office extract: pptx]",
		"# Slide 1",
		"Intro",
		"First point",
		"# Slide 2",
		"Decision",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestReadFileOfficeOutputSupportsOffsetLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paged.docx")
	writeOfficeZip(t, path, map[string]string{
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:r><w:t>One</w:t></w:r></w:p>
<w:p><w:r><w:t>Two</w:t></w:r></w:p>
<w:p><w:r><w:t>Three</w:t></w:r></w:p>
<w:p><w:r><w:t>Four</w:t></w:r></w:p>
</w:body></w:document>`,
	})

	out := runTool(t, readFile{}, map[string]any{"path": path, "offset": 4, "limit": 2})
	if !strings.Contains(out, "5→One") || !strings.Contains(out, "6→Two") {
		t.Fatalf("expected paged document content with read_file line numbers:\n%s", out)
	}
	if strings.Contains(out, "Three") || !strings.Contains(out, "offset=6") {
		t.Fatalf("expected read_file pagination semantics for Office output:\n%s", out)
	}
}

func TestReadFileLegacyOfficeFormatsRejectedBeforeBinaryCheck(t *testing.T) {
	for _, name := range []string{"old.doc", "old.xls", "old.ppt"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte{0xd0, 0xcf, 0x11, 0xe0, 0, 0}, 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := readFile{}.Execute(context.Background(), argsJSON(t, map[string]any{"path": path}))
			if err == nil {
				t.Fatal("expected legacy Office error")
			}
			msg := err.Error()
			if !strings.Contains(msg, "legacy Office binary format") || !strings.Contains(msg, ".docx/.xlsx/.pptx") {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Contains(msg, "NUL byte") {
				t.Fatalf("legacy Office should be rejected before binary NUL check: %v", err)
			}
		})
	}
}

func TestReadFileOfficeCorruptZipReturnsActionableError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.docx")
	if err := os.WriteFile(path, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readFile{}.Execute(context.Background(), argsJSON(t, map[string]any{"path": path}))
	if err == nil {
		t.Fatal("expected corrupt Office ZIP error")
	}
	if !strings.Contains(err.Error(), "corrupt Office ZIP container") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadFileOfficeMissingRequiredXMLReturnsActionableError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.docx")
	writeOfficeZip(t, path, map[string]string{
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:r><w:t>Header only</w:t></w:r></w:p></w:hdr>`,
	})
	_, err := readFile{}.Execute(context.Background(), argsJSON(t, map[string]any{"path": path}))
	if err == nil {
		t.Fatal("expected missing XML error")
	}
	if !strings.Contains(err.Error(), "missing required XML entry word/document.xml") {
		t.Fatalf("unexpected error: %v", err)
	}
}
