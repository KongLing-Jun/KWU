package office

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type workbookSheet struct {
	name string
	rid  string
}

type sheetText struct {
	name  string
	lines []string
}

func extractXLSX(ctx context.Context, a *archive, source string) (string, error) {
	sheets, rels, shared, err := readWorkbookParts(ctx, a, source)
	if err != nil {
		return "", err
	}
	out, err := readWorkbookSheets(ctx, a, source, sheets, rels, shared)
	if err != nil {
		return "", err
	}
	return renderXLSX(source, out), nil
}

func readWorkbookParts(ctx context.Context, a *archive, source string) ([]workbookSheet, map[string]string, []string, error) {
	sheets, err := readWorkbookSheetList(ctx, a, source)
	if err != nil {
		return nil, nil, nil, err
	}
	rels, err := readWorkbookRels(ctx, a, source)
	if err != nil {
		return nil, nil, nil, err
	}
	shared, err := readSharedStrings(ctx, a, source)
	return sheets, rels, shared, err
}

func readWorkbookSheetList(ctx context.Context, a *archive, source string) ([]workbookSheet, error) {
	workbookXML, err := a.readXML(ctx, FormatXLSX, source, "xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	sheets, err := parseWorkbookSheets(workbookXML)
	if err != nil {
		return nil, xmlError(FormatXLSX, source, "xl/workbook.xml", err)
	}
	if len(sheets) > a.limits.MaxSheets {
		return nil, officeError(ErrTooLarge, FormatXLSX, source, nil, fmt.Sprintf("sheet count %d > %d", len(sheets), a.limits.MaxSheets))
	}
	return sheets, nil
}

func readWorkbookRels(ctx context.Context, a *archive, source string) (map[string]string, error) {
	relsXML, err := a.readXML(ctx, FormatXLSX, source, "xl/_rels/workbook.xml.rels")
	if err != nil {
		return nil, err
	}
	rels, err := parseRelationships(relsXML)
	if err != nil {
		return nil, xmlError(FormatXLSX, source, "xl/_rels/workbook.xml.rels", err)
	}
	return rels, nil
}

func readSharedStrings(ctx context.Context, a *archive, source string) ([]string, error) {
	if data, ok, err := a.readXMLIfExists(ctx, FormatXLSX, source, "xl/sharedStrings.xml"); err != nil {
		return nil, err
	} else if ok {
		shared, err := parseSharedStrings(data)
		if err != nil {
			return nil, xmlError(FormatXLSX, source, "xl/sharedStrings.xml", err)
		}
		return shared, nil
	}
	return nil, nil
}

func readWorkbookSheets(ctx context.Context, a *archive, source string, sheets []workbookSheet, rels map[string]string, shared []string) ([]sheetText, error) {
	out := make([]sheetText, 0, len(sheets))
	for i, sheet := range sheets {
		target := relTarget("xl/workbook.xml", rels[sheet.rid])
		if target == "" {
			return nil, missingEntry(FormatXLSX, source, "relationship "+sheet.rid)
		}
		lines, err := readSheetLines(ctx, a, source, target, shared)
		if err != nil {
			return nil, err
		}
		out = append(out, sheetText{name: sheetName(sheet.name, i), lines: lines})
	}
	return out, nil
}

func readSheetLines(ctx context.Context, a *archive, source, target string, shared []string) ([]string, error) {
	data, err := a.readXML(ctx, FormatXLSX, source, target)
	if err != nil {
		return nil, err
	}
	lines, err := parseWorksheet(ctx, data, shared, a.limits)
	if err != nil {
		return nil, xmlError(FormatXLSX, source, target, err)
	}
	return lines, nil
}

func sheetName(name string, index int) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return fmt.Sprintf("Sheet%d", index+1)
}

func parseWorkbookSheets(data []byte) ([]workbookSheet, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	var out []workbookSheet
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "sheet" {
			continue
		}
		out = append(out, workbookSheet{name: attr(se, "name"), rid: relID(se)})
	}
	return out, nil
}

func parseSharedStrings(data []byte) ([]string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	state := &sharedStringState{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			state.start(t.Name.Local)
		case xml.CharData:
			state.charData(string(t))
		case xml.EndElement:
			state.end(t.Name.Local)
		}
	}
	return state.values, nil
}

type sharedStringState struct {
	values  []string
	current strings.Builder
	inSI    bool
	inText  bool
}

func (s *sharedStringState) start(local string) {
	switch local {
	case "si":
		s.inSI = true
		s.current.Reset()
	case "t":
		if s.inSI {
			s.inText = true
		}
	}
}

func (s *sharedStringState) charData(text string) {
	if s.inSI && s.inText {
		s.current.WriteString(text)
	}
}

func (s *sharedStringState) end(local string) {
	switch local {
	case "t":
		s.inText = false
	case "si":
		s.values = append(s.values, normalizeText(s.current.String()))
		s.current.Reset()
		s.inSI = false
	}
}

func parseWorksheet(ctx context.Context, data []byte, shared []string, limits Limits) ([]string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	state := newWorksheetState(shared, limits)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := state.start(t); err != nil {
				return nil, err
			}
		case xml.CharData:
			state.charData(string(t))
		case xml.EndElement:
			state.end(t.Name.Local)
		}
	}
	return state.lines, nil
}

type worksheetState struct {
	limits       Limits
	shared       []string
	lines        []string
	value        strings.Builder
	formula      strings.Builder
	inline       strings.Builder
	rowCells     map[int]string
	inCell       bool
	inValue      bool
	inFormula    bool
	inInlineText bool
	cellType     string
	cellCol      int
	minCol       int
	maxCol       int
	rowsSeen     int
	cellsSeen    int
}

func newWorksheetState(shared []string, limits Limits) *worksheetState {
	return &worksheetState{shared: shared, limits: limits, rowCells: map[int]string{}}
}

func (s *worksheetState) start(se xml.StartElement) error {
	switch se.Name.Local {
	case "row":
		return s.startRow()
	case "c":
		return s.startCell(se)
	case "v":
		s.inValue = s.inCell
	case "f":
		s.inFormula = s.inCell
	case "t":
		s.inInlineText = s.inCell
	}
	return nil
}

func (s *worksheetState) startRow() error {
	s.rowsSeen++
	if s.rowsSeen > s.limits.MaxRowsPerSheet {
		return fmt.Errorf("row count exceeds limit")
	}
	s.rowCells = map[int]string{}
	s.minCol, s.maxCol = 0, 0
	return nil
}

func (s *worksheetState) startCell(se xml.StartElement) error {
	s.cellsSeen++
	if s.cellsSeen > s.limits.MaxCellsPerSheet {
		return fmt.Errorf("cell count exceeds limit")
	}
	s.inCell = true
	s.cellType = attr(se, "t")
	s.cellCol = colIndex(attr(se, "r"))
	if s.cellCol == 0 {
		s.cellCol = s.maxCol + 1
	}
	return nil
}

func (s *worksheetState) charData(text string) {
	switch {
	case s.inValue:
		s.value.WriteString(text)
	case s.inFormula:
		s.formula.WriteString(text)
	case s.inInlineText:
		s.inline.WriteString(text)
	}
}

func (s *worksheetState) end(local string) {
	switch local {
	case "v":
		s.inValue = false
	case "f":
		s.inFormula = false
	case "t":
		s.inInlineText = false
	case "c":
		s.endCell()
	case "row":
		s.endRow()
	}
}

func (s *worksheetState) endCell() {
	text := xlsxCellText(s.cellType, s.value.String(), s.formula.String(), s.inline.String(), s.shared)
	if strings.TrimSpace(text) != "" {
		s.recordCell(text)
	}
	s.value.Reset()
	s.formula.Reset()
	s.inline.Reset()
	s.cellType = ""
	s.cellCol = 0
	s.inCell = false
}

func (s *worksheetState) recordCell(text string) {
	s.rowCells[s.cellCol] = text
	if s.minCol == 0 || s.cellCol < s.minCol {
		s.minCol = s.cellCol
	}
	if s.cellCol > s.maxCol {
		s.maxCol = s.cellCol
	}
}

func (s *worksheetState) endRow() {
	if len(s.rowCells) == 0 || s.minCol == 0 || s.maxCol == 0 {
		return
	}
	fields := s.rowFields()
	if rowHasText(fields) {
		s.lines = append(s.lines, strings.Join(fields, "\t"))
	}
}

func (s *worksheetState) rowFields() []string {
	fields := make([]string, 0, s.maxCol-s.minCol+1)
	for col := s.minCol; col <= s.maxCol; col++ {
		fields = append(fields, s.rowCells[col])
	}
	return fields
}

func xlsxCellText(cellType, value, formula, inline string, shared []string) string {
	value = strings.TrimSpace(value)
	formula = strings.TrimSpace(formula)
	inline = normalizeText(inline)
	if value != "" {
		return xlsxValueText(cellType, value, inline, shared)
	}
	if inline != "" {
		return inline
	}
	return xlsxFormulaText(formula)
}

func xlsxValueText(cellType, value, inline string, shared []string) string {
	switch cellType {
	case "s":
		return xlsxSharedText(value, shared)
	case "b":
		return xlsxBoolText(value)
	case "inlineStr":
		return firstNonEmpty(inline, value)
	default:
		return value
	}
}

func xlsxSharedText(value string, shared []string) string {
	idx, ok := parseInt(value)
	if ok && idx >= 0 && idx < len(shared) {
		return shared[idx]
	}
	return value
}

func xlsxBoolText(value string) string {
	switch value {
	case "1":
		return "TRUE"
	case "0":
		return "FALSE"
	default:
		return value
	}
}

func xlsxFormulaText(formula string) string {
	if formula == "" {
		return ""
	}
	return "[formula not calculated: " + formula + "]"
}

func firstNonEmpty(vals ...string) string {
	for _, val := range vals {
		if val != "" {
			return val
		}
	}
	return ""
}
