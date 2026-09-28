package automation

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"kwu/internal/event"
	"kwu/internal/fileutil"
)

const businessDiagnosticsArtifactName = "business-diagnostics.json"

type BusinessDataKind string

const (
	BusinessDataSales     BusinessDataKind = "sales"
	BusinessDataInventory BusinessDataKind = "inventory"
	BusinessDataPurchase  BusinessDataKind = "purchase"
	BusinessDataFinance   BusinessDataKind = "finance"
	BusinessDataReturns   BusinessDataKind = "returns"
)

type BusinessSourceType string

const (
	BusinessSourceCSV  BusinessSourceType = "csv"
	BusinessSourceXLSX BusinessSourceType = "xlsx"
	BusinessSourceHTTP BusinessSourceType = "http"
	BusinessSourceMCP  BusinessSourceType = "mcp"
)

type BusinessDataSource struct {
	ID     string             `json:"id"`
	Kind   BusinessDataKind   `json:"kind"`
	Type   BusinessSourceType `json:"type"`
	Path   string             `json:"path,omitempty"`
	URL    string             `json:"url,omitempty"`
	Format string             `json:"format,omitempty"`
	Rows   []map[string]any   `json:"rows,omitempty"`
}

type BusinessDiagnosticsConfig struct {
	Sources     []BusinessDataSource `json:"sources"`
	TargetStart time.Time            `json:"targetStart"`
	TargetEnd   time.Time            `json:"targetEnd"`
	Region      string               `json:"region,omitempty"`
	Recipients  []string             `json:"recipients,omitempty"`
}

type WeeklyBusinessDiagnosticsParams struct {
	ID          string
	Workspace   string
	RunTime     string
	Timezone    string
	Region      string
	Recipients  []string
	TargetStart time.Time
	TargetEnd   time.Time
	Sources     []BusinessDataSource
}

type BusinessInputSummary struct {
	SourceID string             `json:"sourceId"`
	Kind     BusinessDataKind   `json:"kind"`
	Type     BusinessSourceType `json:"type"`
	Location string             `json:"location,omitempty"`
	Rows     int                `json:"rows"`
}

type BusinessNormalizedDataset struct {
	Records []BusinessRecord `json:"records"`
}

type BusinessRecord struct {
	SourceID    string           `json:"sourceId"`
	Kind        BusinessDataKind `json:"kind"`
	RowNumber   int              `json:"rowNumber"`
	Date        time.Time        `json:"date"`
	Region      string           `json:"region,omitempty"`
	RecordID    string           `json:"recordId,omitempty"`
	OrderID     string           `json:"orderId,omitempty"`
	SKU         string           `json:"sku,omitempty"`
	Quantity    float64          `json:"quantity,omitempty"`
	Amount      float64          `json:"amount,omitempty"`
	OnHandQty   float64          `json:"onHandQty,omitempty"`
	ReservedQty float64          `json:"reservedQty,omitempty"`
	Warehouse   string           `json:"warehouse,omitempty"`
	Supplier    string           `json:"supplier,omitempty"`
	Account     string           `json:"account,omitempty"`
	Category    string           `json:"category,omitempty"`
	Reason      string           `json:"reason,omitempty"`
}

type BusinessQualityIssue struct {
	Severity       string           `json:"severity"`
	Code           string           `json:"code"`
	Message        string           `json:"message"`
	SourceID       string           `json:"sourceId,omitempty"`
	Kind           BusinessDataKind `json:"kind,omitempty"`
	RowNumber      int              `json:"rowNumber,omitempty"`
	Field          string           `json:"field,omitempty"`
	BlocksAnalysis bool             `json:"blocksAnalysis,omitempty"`
}

type BusinessMetric struct {
	Name  string                `json:"name"`
	Label string                `json:"label"`
	Value float64               `json:"value"`
	Unit  string                `json:"unit"`
	Trace []BusinessMetricTrace `json:"trace"`
}

type BusinessMetricTrace struct {
	SourceID string           `json:"sourceId"`
	Kind     BusinessDataKind `json:"kind"`
	Rows     []int            `json:"rows"`
}

type BusinessDiagnosticsReport struct {
	GeneratedAt    time.Time                 `json:"generatedAt"`
	TargetStart    time.Time                 `json:"targetStart"`
	TargetEnd      time.Time                 `json:"targetEnd"`
	Region         string                    `json:"region,omitempty"`
	RawInputs      []BusinessInputSummary    `json:"rawInputs"`
	Dataset        BusinessNormalizedDataset `json:"dataset"`
	QualityIssues  []BusinessQualityIssue    `json:"qualityIssues,omitempty"`
	Metrics        []BusinessMetric          `json:"metrics,omitempty"`
	Blocked        bool                      `json:"blocked"`
	BlockingReason string                    `json:"blockingReason,omitempty"`
}

type BusinessDiagnosticsRunner struct {
	Next   Runner
	Client *http.Client
	now    func() time.Time
}

func NewBusinessDiagnosticsRunner(next Runner) *BusinessDiagnosticsRunner {
	return &BusinessDiagnosticsRunner{Next: next, Client: &http.Client{Timeout: 30 * time.Second}, now: time.Now}
}

func (r *BusinessDiagnosticsRunner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	if req.Task.BusinessDiagnostics == nil {
		if r.Next != nil {
			return r.Next.Run(ctx, req)
		}
		err := errors.New("automation: business diagnostics config is required")
		return runnerFailedRunResult(err), err
	}
	if strings.TrimSpace(req.RunDir) == "" {
		err := errors.New("automation: run dir is required")
		return failedRunResult(err), err
	}
	report, err := RunBusinessDiagnostics(ctx, *req.Task.BusinessDiagnostics, r.Client)
	if err != nil {
		return runnerFailedRunResult(err), err
	}
	if r.now != nil {
		report.GeneratedAt = r.now().UTC()
	}
	artifactPath := filepath.Join(req.RunDir, businessDiagnosticsArtifactName)
	if err := writeBusinessArtifact(artifactPath, report); err != nil {
		result := RunResult{Status: RunStatusFailed, ErrorCode: RunErrorArchiveFailed, ErrorMessage: err.Error(), Error: err.Error()}
		return result, err
	}
	markdown := businessDiagnosticsMarkdown(report)
	resultPath := filepath.Join(req.RunDir, resultFileName)
	if err := WriteResult(resultPath, Result{Markdown: markdown}); err != nil {
		result := RunResult{Status: RunStatusFailed, ErrorCode: RunErrorArchiveFailed, ErrorMessage: err.Error(), Error: err.Error()}
		return result, err
	}
	if req.Sink != nil {
		req.Sink.Emit(event.Event{Kind: event.Message, Text: markdown})
	}
	result := RunResult{
		Status:     RunStatusSucceeded,
		Summary:    businessReportSummary(report),
		ResultPath: resultPath,
	}
	if report.Blocked {
		result.Status = RunStatusBlocked
		result.ErrorCode = RunErrorDataQualityBlocked
		result.ErrorMessage = report.BlockingReason
		result.Error = report.BlockingReason
	}
	return result, nil
}

func WeeklyBusinessDiagnosticsTask(params WeeklyBusinessDiagnosticsParams) (Task, error) {
	if _, _, ok := parseScheduleClock(params.RunTime); !ok {
		return Task{}, fmt.Errorf("%w: invalid run time %q", ErrInvalidSchedule, params.RunTime)
	}
	cfg, err := NormalizeBusinessDiagnostics(BusinessDiagnosticsConfig{
		Sources:     params.Sources,
		TargetStart: params.TargetStart,
		TargetEnd:   params.TargetEnd,
		Region:      params.Region,
		Recipients:  params.Recipients,
	})
	if err != nil {
		return Task{}, err
	}
	timezone := strings.TrimSpace(params.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	name := "周度经营诊断"
	if cfg.Region != "" {
		name += " - " + cfg.Region
	}
	task := Task{
		ID:            strings.TrimSpace(params.ID),
		Name:          name,
		Enabled:       true,
		Scope:         "business_operations",
		WorkspaceRoot: strings.TrimSpace(params.Workspace),
		Prompt: PromptConfig{
			Role:   "business operations analyst",
			Prompt: weeklyBusinessDiagnosticsPrompt(cfg),
		},
		Schedule: Schedule{
			Kind:     ScheduleKindCalendar,
			Timezone: timezone,
			Calendar: CalendarSchedule{
				Frequency: "weekly",
				TimeOfDay: params.RunTime,
				Weekdays:  []string{"monday"},
			},
		},
		Execution: ExecutionPolicy{
			ApprovalMode:    ApprovalModeAuto,
			ApprovalTimeout: DefaultApprovalTimeoutString,
		},
		Archive: ArchivePolicy{
			Visible:    true,
			KeepEvents: true,
		},
		BusinessDiagnostics: &cfg,
	}
	return NormalizeTask(task)
}

func NormalizeBusinessDiagnostics(cfg BusinessDiagnosticsConfig) (BusinessDiagnosticsConfig, error) {
	cfg.Region = strings.TrimSpace(cfg.Region)
	for i := range cfg.Recipients {
		cfg.Recipients[i] = strings.TrimSpace(cfg.Recipients[i])
	}
	recipients := cfg.Recipients[:0]
	for _, recipient := range cfg.Recipients {
		if recipient != "" {
			recipients = append(recipients, recipient)
		}
	}
	cfg.Recipients = recipients
	for i := range cfg.Sources {
		cfg.Sources[i].ID = strings.TrimSpace(cfg.Sources[i].ID)
		cfg.Sources[i].Kind = BusinessDataKind(strings.ToLower(strings.TrimSpace(string(cfg.Sources[i].Kind))))
		cfg.Sources[i].Type = BusinessSourceType(strings.ToLower(strings.TrimSpace(string(cfg.Sources[i].Type))))
		cfg.Sources[i].Path = strings.TrimSpace(cfg.Sources[i].Path)
		cfg.Sources[i].URL = strings.TrimSpace(cfg.Sources[i].URL)
		cfg.Sources[i].Format = strings.ToLower(strings.TrimSpace(cfg.Sources[i].Format))
	}
	if err := ValidateBusinessDiagnostics(cfg); err != nil {
		return BusinessDiagnosticsConfig{}, err
	}
	return cfg, nil
}

func ValidateBusinessDiagnostics(cfg BusinessDiagnosticsConfig) error {
	if !cfg.TargetStart.Before(cfg.TargetEnd) {
		return fmt.Errorf("business diagnostics target range is required")
	}
	if len(cfg.Sources) == 0 {
		return fmt.Errorf("business diagnostics sources are required")
	}
	seen := map[string]bool{}
	for _, source := range cfg.Sources {
		if source.ID == "" {
			return fmt.Errorf("business diagnostics source id is required")
		}
		if seen[source.ID] {
			return fmt.Errorf("duplicate business diagnostics source id %q", source.ID)
		}
		seen[source.ID] = true
		if !validBusinessDataKind(source.Kind) {
			return fmt.Errorf("invalid business data kind %q", source.Kind)
		}
		if !validBusinessSourceType(source.Type) {
			return fmt.Errorf("invalid business source type %q", source.Type)
		}
		switch source.Type {
		case BusinessSourceCSV, BusinessSourceXLSX:
			if source.Path == "" {
				return fmt.Errorf("business source %q path is required", source.ID)
			}
		case BusinessSourceHTTP:
			if source.URL == "" {
				return fmt.Errorf("business source %q url is required", source.ID)
			}
		case BusinessSourceMCP:
			if source.Rows == nil {
				return fmt.Errorf("business source %q rows are required", source.ID)
			}
		}
	}
	return nil
}

func RunBusinessDiagnostics(ctx context.Context, cfg BusinessDiagnosticsConfig, client *http.Client) (BusinessDiagnosticsReport, error) {
	cfg, err := NormalizeBusinessDiagnostics(cfg)
	if err != nil {
		return BusinessDiagnosticsReport{}, err
	}
	report := BusinessDiagnosticsReport{
		GeneratedAt: time.Now().UTC(),
		TargetStart: cfg.TargetStart.UTC(),
		TargetEnd:   cfg.TargetEnd.UTC(),
		Region:      cfg.Region,
	}
	for _, source := range cfg.Sources {
		rows, summary, issues := readBusinessSource(ctx, source, client)
		report.RawInputs = append(report.RawInputs, summary)
		report.QualityIssues = append(report.QualityIssues, issues...)
		if hasBlockingIssue(issues) {
			continue
		}
		records, issues := normalizeBusinessRecords(source, rows, cfg)
		report.Dataset.Records = append(report.Dataset.Records, records...)
		report.QualityIssues = append(report.QualityIssues, issues...)
	}
	report.QualityIssues = append(report.QualityIssues, businessDatasetQuality(report.Dataset, cfg)...)
	if hasBlockingIssue(report.QualityIssues) {
		report.Blocked = true
		report.BlockingReason = firstBlockingReason(report.QualityIssues)
		return report, nil
	}
	report.Metrics = businessMetrics(report.Dataset, cfg)
	return report, nil
}

func readBusinessSource(ctx context.Context, source BusinessDataSource, client *http.Client) ([]map[string]string, BusinessInputSummary, []BusinessQualityIssue) {
	summary := BusinessInputSummary{
		SourceID: source.ID,
		Kind:     source.Kind,
		Type:     source.Type,
		Location: source.Path,
	}
	var (
		rows []map[string]string
		err  error
	)
	switch source.Type {
	case BusinessSourceCSV:
		rows, err = readBusinessCSVFile(source.Path)
	case BusinessSourceXLSX:
		rows, err = readBusinessXLSXFile(source.Path)
	case BusinessSourceHTTP:
		summary.Location = source.URL
		rows, err = readBusinessHTTP(ctx, source, client)
	case BusinessSourceMCP:
		rows = rowsFromBusinessMaps(source.Rows)
	default:
		err = fmt.Errorf("unsupported source type %q", source.Type)
	}
	summary.Rows = len(rows)
	if err != nil {
		return nil, summary, []BusinessQualityIssue{blockingBusinessIssue(source, 0, "", "source_read_failed", err.Error())}
	}
	if len(rows) == 0 {
		return nil, summary, []BusinessQualityIssue{blockingBusinessIssue(source, 0, "", "source_empty", "数据源没有记录")}
	}
	return rows, summary, nil
}

func readBusinessCSVFile(path string) ([]map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return readBusinessCSV(b)
}

func readBusinessCSV(b []byte) ([]map[string]string, error) {
	reader := csv.NewReader(bytes.NewReader(b))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	return businessRowsFromTable(records)
}

func readBusinessXLSXFile(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return nil, err
	}
	return readBusinessXLSX(zr)
}

func readBusinessXLSX(zr *zip.Reader) ([]map[string]string, error) {
	shared, err := readXLSXSharedStrings(zr)
	if err != nil {
		return nil, err
	}
	sheet, err := readZipFile(zr, "xl/worksheets/sheet1.xml")
	if err != nil {
		return nil, err
	}
	var worksheet xlsxWorksheet
	if err := xml.Unmarshal(sheet, &worksheet); err != nil {
		return nil, err
	}
	table := make([][]string, 0, len(worksheet.Rows))
	for _, row := range worksheet.Rows {
		values := []string{}
		for _, cell := range row.Cells {
			idx := xlsxColumnIndex(cell.Ref)
			if idx < 0 {
				idx = len(values)
			}
			for len(values) <= idx {
				values = append(values, "")
			}
			values[idx] = cell.Value(shared)
		}
		table = append(table, values)
	}
	return businessRowsFromTable(table)
}

func readBusinessHTTP(ctx context.Context, source BusinessDataSource, client *http.Client) ([]map[string]string, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("http source returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	format := inferBusinessFormat(source.Format, source.URL, resp.Header.Get("Content-Type"))
	switch format {
	case "csv":
		return readBusinessCSV(body)
	case "json":
		var rows []map[string]any
		if err := json.Unmarshal(body, &rows); err != nil {
			return nil, err
		}
		return rowsFromBusinessMaps(rows), nil
	case "xlsx":
		zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			return nil, err
		}
		return readBusinessXLSX(zr)
	default:
		return nil, fmt.Errorf("unsupported http format %q", format)
	}
}

func rowsFromBusinessMaps(rows []map[string]any) []map[string]string {
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		next := map[string]string{}
		for key, value := range row {
			next[canonicalBusinessField(key)] = strings.TrimSpace(fmt.Sprint(value))
		}
		out = append(out, next)
	}
	return out
}

func businessRowsFromTable(table [][]string) ([]map[string]string, error) {
	if len(table) == 0 {
		return nil, nil
	}
	headers := make([]string, len(table[0]))
	for i, header := range table[0] {
		headers[i] = canonicalBusinessField(header)
	}
	rows := make([]map[string]string, 0, len(table)-1)
	for _, raw := range table[1:] {
		row := map[string]string{}
		empty := true
		for i, value := range raw {
			if i >= len(headers) || headers[i] == "" {
				continue
			}
			value = strings.TrimSpace(value)
			if value != "" {
				empty = false
			}
			row[headers[i]] = value
		}
		if !empty {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func normalizeBusinessRecords(source BusinessDataSource, rows []map[string]string, cfg BusinessDiagnosticsConfig) ([]BusinessRecord, []BusinessQualityIssue) {
	var issues []BusinessQualityIssue
	for _, field := range requiredBusinessFields(source.Kind) {
		if !businessRowsHaveField(rows, field) {
			issues = append(issues, blockingBusinessIssue(source, 0, field, "missing_column", "缺少必填列 "+field))
		}
	}
	if hasBlockingIssue(issues) {
		return nil, issues
	}
	records := make([]BusinessRecord, 0, len(rows))
	for i, row := range rows {
		record, rowIssues := businessRecordFromRow(source, i+2, row)
		issues = append(issues, rowIssues...)
		if len(rowIssues) == 0 {
			if cfg.Region == "" || strings.EqualFold(record.Region, cfg.Region) {
				records = append(records, record)
			}
		}
	}
	return records, issues
}

func businessRecordFromRow(source BusinessDataSource, rowNumber int, row map[string]string) (BusinessRecord, []BusinessQualityIssue) {
	record := BusinessRecord{
		SourceID:  source.ID,
		Kind:      source.Kind,
		RowNumber: rowNumber,
		Region:    businessValue(row, "region"),
		SKU:       businessValue(row, "sku"),
	}
	var issues []BusinessQualityIssue
	for _, field := range requiredBusinessFields(source.Kind) {
		if strings.TrimSpace(businessValue(row, field)) == "" {
			issues = append(issues, blockingBusinessIssue(source, rowNumber, field, "empty_value", "必填字段为空"))
		}
	}
	if len(issues) > 0 {
		return record, issues
	}
	date, err := parseBusinessDate(businessValue(row, "date"))
	if err != nil {
		issues = append(issues, blockingBusinessIssue(source, rowNumber, "date", "invalid_date", "日期格式无效："+businessValue(row, "date")))
		return record, issues
	}
	record.Date = date.UTC()
	switch source.Kind {
	case BusinessDataSales:
		record.RecordID = businessValue(row, "order_id")
		record.OrderID = record.RecordID
		record.Quantity, issues = businessFloatField(source, rowNumber, row, "quantity", issues)
		record.Amount, issues = businessFloatField(source, rowNumber, row, "amount", issues)
	case BusinessDataInventory:
		record.RecordID = record.SKU
		record.OnHandQty, issues = businessFloatField(source, rowNumber, row, "on_hand_qty", issues)
		record.ReservedQty, issues = businessFloatField(source, rowNumber, row, "reserved_qty", issues)
		record.Warehouse = businessValue(row, "warehouse")
	case BusinessDataPurchase:
		record.RecordID = businessValue(row, "purchase_id")
		record.Quantity, issues = businessFloatField(source, rowNumber, row, "quantity", issues)
		record.Amount, issues = businessFloatField(source, rowNumber, row, "amount", issues)
		record.Supplier = businessValue(row, "supplier")
	case BusinessDataFinance:
		record.RecordID = businessValue(row, "account")
		record.Account = record.RecordID
		record.Amount, issues = businessFloatField(source, rowNumber, row, "amount", issues)
		record.Category = businessValue(row, "category")
	case BusinessDataReturns:
		record.RecordID = businessValue(row, "return_id")
		record.OrderID = businessValue(row, "order_id")
		record.Quantity, issues = businessFloatField(source, rowNumber, row, "quantity", issues)
		record.Amount, issues = businessFloatField(source, rowNumber, row, "amount", issues)
		record.Reason = businessValue(row, "reason")
	}
	return record, issues
}

func businessFloatField(source BusinessDataSource, rowNumber int, row map[string]string, field string, issues []BusinessQualityIssue) (float64, []BusinessQualityIssue) {
	value, err := parseBusinessFloat(businessValue(row, field))
	if err != nil {
		return 0, append(issues, blockingBusinessIssue(source, rowNumber, field, "invalid_number", "数值格式无效："+businessValue(row, field)))
	}
	if value < 0 {
		return value, append(issues, blockingBusinessIssue(source, rowNumber, field, "negative_number", "金额和数量字段不能为负数"))
	}
	if value > 1_000_000_000_000 {
		return value, append(issues, warningBusinessIssue(source, rowNumber, field, "number_outlier", "数值异常偏大"))
	}
	return value, issues
}

func businessDatasetQuality(dataset BusinessNormalizedDataset, cfg BusinessDiagnosticsConfig) []BusinessQualityIssue {
	var issues []BusinessQualityIssue
	requiredKinds := []BusinessDataKind{BusinessDataSales, BusinessDataInventory, BusinessDataPurchase}
	for _, kind := range requiredKinds {
		if len(businessRecords(dataset, kind, cfg.TargetStart, cfg.TargetEnd)) == 0 {
			issues = append(issues, BusinessQualityIssue{
				Severity:       "error",
				Code:           "insufficient_current_data",
				Kind:           kind,
				Message:        fmt.Sprintf("缺少当前周期 %s 记录，无法分析", kind),
				BlocksAnalysis: true,
			})
		}
	}
	duration := cfg.TargetEnd.Sub(cfg.TargetStart)
	prevStart := cfg.TargetStart.Add(-duration)
	if len(businessRecords(dataset, BusinessDataSales, prevStart, cfg.TargetStart)) == 0 {
		issues = append(issues, BusinessQualityIssue{Severity: "error", Code: "missing_week_over_week_sales", Kind: BusinessDataSales, Message: "缺少环比所需的上一周期销售记录", BlocksAnalysis: true})
	}
	if len(businessRecords(dataset, BusinessDataSales, cfg.TargetStart.AddDate(-1, 0, 0), cfg.TargetEnd.AddDate(-1, 0, 0))) == 0 {
		issues = append(issues, BusinessQualityIssue{Severity: "error", Code: "missing_year_over_year_sales", Kind: BusinessDataSales, Message: "缺少同比所需的去年同期销售记录", BlocksAnalysis: true})
	}
	seen := map[string]BusinessRecord{}
	for _, record := range dataset.Records {
		if !record.Date.IsZero() && outsideBusinessObservationWindow(record.Date, cfg) {
			issues = append(issues, BusinessQualityIssue{Severity: "warning", Code: "outside_time_range", SourceID: record.SourceID, Kind: record.Kind, RowNumber: record.RowNumber, Message: "记录不在目标周期、上一周期或去年同期比较范围内"})
		}
		key := businessDuplicateKey(record)
		if prev, ok := seen[key]; ok {
			issues = append(issues, BusinessQualityIssue{Severity: "error", Code: "duplicate_record", SourceID: record.SourceID, Kind: record.Kind, RowNumber: record.RowNumber, Message: fmt.Sprintf("与第 %d 行重复", prev.RowNumber), BlocksAnalysis: true})
		}
		seen[key] = record
	}
	return issues
}

func businessMetrics(dataset BusinessNormalizedDataset, cfg BusinessDiagnosticsConfig) []BusinessMetric {
	currentSales := businessRecords(dataset, BusinessDataSales, cfg.TargetStart, cfg.TargetEnd)
	currentInventory := latestInventoryRecords(businessRecords(dataset, BusinessDataInventory, cfg.TargetStart, cfg.TargetEnd))
	currentPurchase := businessRecords(dataset, BusinessDataPurchase, cfg.TargetStart, cfg.TargetEnd)
	durationDays := math.Max(1, cfg.TargetEnd.Sub(cfg.TargetStart).Hours()/24)
	salesQty := sumBusiness(currentSales, func(r BusinessRecord) float64 { return r.Quantity })
	purchaseQty := sumBusiness(currentPurchase, func(r BusinessRecord) float64 { return r.Quantity })
	remaining := sumBusiness(currentInventory, func(r BusinessRecord) float64 { return r.OnHandQty - r.ReservedQty })
	prevStart := cfg.TargetStart.Add(-cfg.TargetEnd.Sub(cfg.TargetStart))
	prevSales := businessRecords(dataset, BusinessDataSales, prevStart, cfg.TargetStart)
	yoySales := businessRecords(dataset, BusinessDataSales, cfg.TargetStart.AddDate(-1, 0, 0), cfg.TargetEnd.AddDate(-1, 0, 0))
	currentAmount := sumBusiness(currentSales, func(r BusinessRecord) float64 { return r.Amount })
	prevAmount := sumBusiness(prevSales, func(r BusinessRecord) float64 { return r.Amount })
	yoyAmount := sumBusiness(yoySales, func(r BusinessRecord) float64 { return r.Amount })
	return []BusinessMetric{
		{
			Name:  "order_turnover_rate",
			Label: "订单周转率",
			Value: float64(len(distinctBusinessOrders(currentSales))) / durationDays,
			Unit:  "orders/day",
			Trace: businessTrace(currentSales),
		},
		{
			Name:  "inventory_remaining_qty",
			Label: "库存余量",
			Value: remaining,
			Unit:  "units",
			Trace: businessTrace(currentInventory),
		},
		{
			Name:  "overstock_risk_days",
			Label: "爆仓风险覆盖天数",
			Value: remaining / math.Max(salesQty/durationDays, 1),
			Unit:  "days",
			Trace: append(businessTrace(currentInventory), businessTrace(currentSales)...),
		},
		{
			Name:  "purchase_consumption_ratio",
			Label: "采购消耗速度",
			Value: salesQty / math.Max(purchaseQty, 1),
			Unit:  "ratio",
			Trace: append(businessTrace(currentSales), businessTrace(currentPurchase)...),
		},
		{
			Name:  "week_over_week_sales_amount_change",
			Label: "销售额环比变化",
			Value: businessRatioChange(currentAmount, prevAmount),
			Unit:  "ratio",
			Trace: append(businessTrace(currentSales), businessTrace(prevSales)...),
		},
		{
			Name:  "year_over_year_sales_amount_change",
			Label: "销售额同比变化",
			Value: businessRatioChange(currentAmount, yoyAmount),
			Unit:  "ratio",
			Trace: append(businessTrace(currentSales), businessTrace(yoySales)...),
		},
	}
}

func businessRatioChange(current, baseline float64) float64 {
	if baseline == 0 {
		return 0
	}
	return (current - baseline) / math.Abs(baseline)
}

func businessDiagnosticsMarkdown(report BusinessDiagnosticsReport) string {
	var b strings.Builder
	writeLine(&b, "# 周度经营诊断")
	writeLine(&b, "")
	if report.Blocked {
		writeLine(&b, "## 状态")
		writeLine(&b, "已阻塞："+report.BlockingReason)
		writeLine(&b, "")
	}
	writeLine(&b, "## 原始输入摘要")
	for _, input := range report.RawInputs {
		writeLine(&b, fmt.Sprintf("- %s (%s/%s)：%d 行", input.SourceID, input.Kind, input.Type, input.Rows))
	}
	writeLine(&b, "")
	writeLine(&b, "## 清洗结果")
	writeLine(&b, fmt.Sprintf("- 标准化记录数：%d", len(report.Dataset.Records)))
	writeLine(&b, "")
	writeLine(&b, "## 数据质量")
	if len(report.QualityIssues) == 0 {
		writeLine(&b, "- 未发现阻塞或警告问题。")
	} else {
		for _, issue := range report.QualityIssues {
			prefix := "- " + issue.Severity + " " + issue.Code
			if issue.SourceID != "" {
				prefix += " [" + issue.SourceID + "]"
			}
			if issue.RowNumber > 0 {
				prefix += fmt.Sprintf(" 第 %d 行", issue.RowNumber)
			}
			writeLine(&b, prefix+": "+issue.Message)
		}
	}
	if report.Blocked {
		return strings.TrimSpace(b.String()) + "\n"
	}
	writeLine(&b, "")
	writeLine(&b, "## 指标结果")
	for _, metric := range report.Metrics {
		label := metric.Label
		if label == "" {
			label = metric.Name
		}
		writeLine(&b, fmt.Sprintf("- %s（%s）：%.4f %s", label, metric.Name, metric.Value, metric.Unit))
	}
	writeLine(&b, "")
	writeLine(&b, "## 建议方案")
	writeLine(&b, recommendationLine(report.Metrics))
	return strings.TrimSpace(b.String()) + "\n"
}

func recommendationLine(metrics []BusinessMetric) string {
	for _, metric := range metrics {
		if metric.Name == "overstock_risk_days" {
			switch {
			case metric.Value >= 56:
				return "- 下调近期采购量，并在下次补货前复核滞销 SKU。"
			case metric.Value >= 28:
				return "- 保持保守补货，并在下次运行时继续观察库存覆盖。"
			default:
				return "- 当前库存覆盖处于第一版简单阈值范围内。"
			}
		}
	}
	return "- 缺少风险指标，未生成建议方案。"
}

func writeBusinessArtifact(path string, report BusinessDiagnosticsReport) error {
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(path, b, 0o600)
}

func businessReportSummary(report BusinessDiagnosticsReport) string {
	if report.Blocked {
		return "已阻塞：" + report.BlockingReason
	}
	return fmt.Sprintf("经营诊断完成，生成 %d 个指标", len(report.Metrics))
}

func validBusinessDataKind(kind BusinessDataKind) bool {
	switch kind {
	case BusinessDataSales, BusinessDataInventory, BusinessDataPurchase, BusinessDataFinance, BusinessDataReturns:
		return true
	default:
		return false
	}
}

func validBusinessSourceType(sourceType BusinessSourceType) bool {
	switch sourceType {
	case BusinessSourceCSV, BusinessSourceXLSX, BusinessSourceHTTP, BusinessSourceMCP:
		return true
	default:
		return false
	}
}

func requiredBusinessFields(kind BusinessDataKind) []string {
	switch kind {
	case BusinessDataSales:
		return []string{"date", "order_id", "sku", "quantity", "amount", "region"}
	case BusinessDataInventory:
		return []string{"date", "sku", "on_hand_qty", "reserved_qty", "warehouse", "region"}
	case BusinessDataPurchase:
		return []string{"date", "purchase_id", "sku", "quantity", "amount", "supplier", "region"}
	case BusinessDataFinance:
		return []string{"date", "account", "amount", "category", "region"}
	case BusinessDataReturns:
		return []string{"date", "return_id", "order_id", "sku", "quantity", "amount", "reason", "region"}
	default:
		return nil
	}
}

func businessValue(row map[string]string, field string) string {
	for _, alias := range businessFieldAliases(field) {
		if value, ok := row[alias]; ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func businessFieldAliases(field string) []string {
	switch field {
	case "date":
		return []string{"date", "occurred_at", "order_date", "snapshot_date"}
	case "order_id":
		return []string{"order_id", "orderid", "order_no", "order"}
	case "purchase_id":
		return []string{"purchase_id", "purchaseid", "purchase_no"}
	case "return_id":
		return []string{"return_id", "returnid", "return_no"}
	case "sku":
		return []string{"sku", "product_sku", "item_sku"}
	case "quantity":
		return []string{"quantity", "qty", "units"}
	case "amount":
		return []string{"amount", "total_amount", "sales_amount", "value"}
	case "on_hand_qty":
		return []string{"on_hand_qty", "on_hand", "stock_qty", "inventory_qty"}
	case "reserved_qty":
		return []string{"reserved_qty", "reserved", "allocated_qty"}
	default:
		return []string{field}
	}
}

func businessRowsHaveField(rows []map[string]string, field string) bool {
	for _, row := range rows {
		for _, alias := range businessFieldAliases(field) {
			if _, ok := row[alias]; ok {
				return true
			}
		}
	}
	return false
}

func canonicalBusinessField(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(value)), "\ufeff")
	value = strings.NewReplacer(" ", "_", "-", "_").Replace(value)
	return value
}

func parseBusinessDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006/01/02"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q", value)
}

func parseBusinessFloat(value string) (float64, error) {
	value = strings.TrimSpace(strings.NewReplacer(",", "", "$", "", "¥", "").Replace(value))
	return strconv.ParseFloat(value, 64)
}

func blockingBusinessIssue(source BusinessDataSource, row int, field, code, message string) BusinessQualityIssue {
	return BusinessQualityIssue{Severity: "error", Code: code, Message: message, SourceID: source.ID, Kind: source.Kind, RowNumber: row, Field: field, BlocksAnalysis: true}
}

func warningBusinessIssue(source BusinessDataSource, row int, field, code, message string) BusinessQualityIssue {
	return BusinessQualityIssue{Severity: "warning", Code: code, Message: message, SourceID: source.ID, Kind: source.Kind, RowNumber: row, Field: field}
}

func hasBlockingIssue(issues []BusinessQualityIssue) bool {
	for _, issue := range issues {
		if issue.BlocksAnalysis {
			return true
		}
	}
	return false
}

func firstBlockingReason(issues []BusinessQualityIssue) string {
	for _, issue := range issues {
		if issue.BlocksAnalysis {
			return issue.Message
		}
	}
	return ""
}

func businessRecords(dataset BusinessNormalizedDataset, kind BusinessDataKind, start, end time.Time) []BusinessRecord {
	var out []BusinessRecord
	for _, record := range dataset.Records {
		if record.Kind != kind {
			continue
		}
		if !record.Date.Before(start) && record.Date.Before(end) {
			out = append(out, record)
		}
	}
	return out
}

func latestInventoryRecords(records []BusinessRecord) []BusinessRecord {
	byKey := map[string]BusinessRecord{}
	for _, record := range records {
		key := record.SKU + "\x00" + record.Warehouse
		if prev, ok := byKey[key]; !ok || record.Date.After(prev.Date) {
			byKey[key] = record
		}
	}
	out := make([]BusinessRecord, 0, len(byKey))
	for _, record := range byKey {
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SourceID == out[j].SourceID {
			return out[i].RowNumber < out[j].RowNumber
		}
		return out[i].SourceID < out[j].SourceID
	})
	return out
}

func sumBusiness(records []BusinessRecord, value func(BusinessRecord) float64) float64 {
	var sum float64
	for _, record := range records {
		sum += value(record)
	}
	return sum
}

func distinctBusinessOrders(records []BusinessRecord) map[string]bool {
	out := map[string]bool{}
	for _, record := range records {
		if record.OrderID != "" {
			out[record.OrderID] = true
		}
	}
	return out
}

func businessTrace(records []BusinessRecord) []BusinessMetricTrace {
	grouped := map[string]*BusinessMetricTrace{}
	for _, record := range records {
		key := string(record.Kind) + "\x00" + record.SourceID
		trace := grouped[key]
		if trace == nil {
			trace = &BusinessMetricTrace{SourceID: record.SourceID, Kind: record.Kind}
			grouped[key] = trace
		}
		trace.Rows = append(trace.Rows, record.RowNumber)
	}
	out := make([]BusinessMetricTrace, 0, len(grouped))
	for _, trace := range grouped {
		sort.Ints(trace.Rows)
		out = append(out, *trace)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].SourceID < out[j].SourceID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func outsideBusinessObservationWindow(date time.Time, cfg BusinessDiagnosticsConfig) bool {
	duration := cfg.TargetEnd.Sub(cfg.TargetStart)
	ranges := [][2]time.Time{
		{cfg.TargetStart, cfg.TargetEnd},
		{cfg.TargetStart.Add(-duration), cfg.TargetStart},
		{cfg.TargetStart.AddDate(-1, 0, 0), cfg.TargetEnd.AddDate(-1, 0, 0)},
	}
	for _, r := range ranges {
		if !date.Before(r[0]) && date.Before(r[1]) {
			return false
		}
	}
	return true
}

func businessDuplicateKey(record BusinessRecord) string {
	return strings.Join([]string{string(record.Kind), record.SourceID, record.Date.Format("2006-01-02"), record.RecordID, record.SKU}, "\x00")
}

func inferBusinessFormat(format, sourceURL, contentType string) string {
	if format != "" {
		return strings.ToLower(format)
	}
	lowerURL := strings.ToLower(sourceURL)
	lowerType := strings.ToLower(contentType)
	switch {
	case strings.Contains(lowerType, "json") || strings.HasSuffix(lowerURL, ".json"):
		return "json"
	case strings.Contains(lowerType, "csv") || strings.HasSuffix(lowerURL, ".csv"):
		return "csv"
	default:
		return "csv"
	}
}

func weeklyBusinessDiagnosticsPrompt(cfg BusinessDiagnosticsConfig) string {
	var b strings.Builder
	writeLine(&b, "周度经营诊断固定流程：采集销售 -> 采集库存 -> 对比历史 -> 识别风险 -> 生成方案。")
	writeLine(&b, "只使用 businessDiagnostics 配置中的真实数据源和归一化结果。")
	writeLine(&b, "数据质量阻塞时必须停止分析，不输出伪造结论。")
	if cfg.Region != "" {
		writeLine(&b, "目标业务区域："+cfg.Region)
	}
	if len(cfg.Recipients) > 0 {
		writeLine(&b, "报告收件人："+strings.Join(cfg.Recipients, ", "))
	}
	return strings.TrimSpace(b.String())
}

type xlsxWorksheet struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Ref    string     `xml:"r,attr"`
	Type   string     `xml:"t,attr"`
	V      string     `xml:"v"`
	Inline xlsxInline `xml:"is"`
}

type xlsxInline struct {
	Text string `xml:"t"`
}

func (c xlsxCell) Value(shared []string) string {
	switch c.Type {
	case "inlineStr":
		return strings.TrimSpace(c.Inline.Text)
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(c.V))
		if err == nil && idx >= 0 && idx < len(shared) {
			return shared[idx]
		}
	}
	return strings.TrimSpace(c.V)
}

type xlsxSharedStrings struct {
	Items []xlsxSharedString `xml:"si"`
}

type xlsxSharedString struct {
	Text string `xml:"t"`
}

func readXLSXSharedStrings(zr *zip.Reader) ([]string, error) {
	b, err := readZipFile(zr, "xl/sharedStrings.xml")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var shared xlsxSharedStrings
	if err := xml.Unmarshal(b, &shared); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(shared.Items))
	for _, item := range shared.Items {
		out = append(out, strings.TrimSpace(item.Text))
	}
	return out, nil
}
