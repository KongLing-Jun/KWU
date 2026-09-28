package automation

import (
	"archive/zip"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWeeklyBusinessDiagnosticsTemplateBuildsAutomationTask(t *testing.T) {
	start := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	task, err := WeeklyBusinessDiagnosticsTask(WeeklyBusinessDiagnosticsParams{
		ID:          "weekly-east",
		Workspace:   " D:/project/acme ",
		RunTime:     "03:30",
		Timezone:    "Asia/Shanghai",
		Region:      "east",
		Recipients:  []string{"ops@example.com"},
		TargetStart: start,
		TargetEnd:   start.AddDate(0, 0, 7),
		Sources: []BusinessDataSource{
			{ID: "sales-csv", Kind: BusinessDataSales, Type: BusinessSourceCSV, Path: "sales.csv"},
			{ID: "stock-xlsx", Kind: BusinessDataInventory, Type: BusinessSourceXLSX, Path: "stock.xlsx"},
		},
	})
	if err != nil {
		t.Fatalf("WeeklyBusinessDiagnosticsTask returned error: %v", err)
	}

	if task.ID != "weekly-east" || task.WorkspaceRoot != "D:/project/acme" || !task.Enabled {
		t.Fatalf("task basics = %+v, want enabled weekly task with trimmed workspace", task)
	}
	if task.Schedule.Kind != ScheduleKindCalendar || task.Schedule.Calendar.Frequency != "weekly" || task.Schedule.Calendar.TimeOfDay != "03:30" {
		t.Fatalf("schedule = %+v, want weekly calendar at 03:30", task.Schedule)
	}
	if task.BusinessDiagnostics == nil || task.BusinessDiagnostics.Region != "east" || len(task.BusinessDiagnostics.Sources) != 2 {
		t.Fatalf("business diagnostics config = %+v, want region and sources", task.BusinessDiagnostics)
	}
	for _, want := range []string{"采集销售", "采集库存", "对比历史", "识别风险", "生成方案"} {
		if !strings.Contains(task.Prompt.Prompt, want) {
			t.Fatalf("prompt missing fixed step %q:\n%s", want, task.Prompt.Prompt)
		}
	}
}

func TestBusinessDiagnosticsRunnerWritesArtifactsAndMetrics(t *testing.T) {
	dir := t.TempDir()
	salesPath := filepath.Join(dir, "sales.csv")
	mustWriteFile(t, salesPath, strings.Join([]string{
		"date,order_id,sku,quantity,amount,region",
		"2025-06-16,o-yoy-1,sku-1,6,60,east",
		"2026-06-08,o-prev-1,sku-1,8,80,east",
		"2026-06-16,o-cur-1,sku-1,10,100,east",
		"2026-06-17,o-cur-2,sku-2,5,75,east",
	}, "\n"))
	inventoryPath := filepath.Join(dir, "inventory.xlsx")
	writeXLSX(t, inventoryPath, [][]string{
		{"date", "sku", "on_hand_qty", "reserved_qty", "warehouse", "region"},
		{"2026-06-16", "sku-1", "100", "10", "wh-1", "east"},
		{"2026-06-17", "sku-2", "20", "5", "wh-1", "east"},
	})
	financePath := filepath.Join(dir, "finance.csv")
	mustWriteFile(t, financePath, "date,account,amount,category,region\n2026-06-16,cash,100,revenue,east\n")
	returnsPath := filepath.Join(dir, "returns.csv")
	mustWriteFile(t, returnsPath, "date,return_id,order_id,sku,quantity,amount,reason,region\n2026-06-18,r-1,o-cur-1,sku-1,1,10,damaged,east\n")
	purchasePath := filepath.Join(dir, "purchase.xlsx")
	writeXLSX(t, purchasePath, [][]string{
		{"date", "purchase_id", "sku", "quantity", "amount", "supplier", "region"},
		{"2026-06-15", "p-1", "sku-1", "30", "120", "supplier-a", "east"},
	})
	purchaseBody, err := os.ReadFile(purchasePath)
	if err != nil {
		t.Fatalf("ReadFile purchase xlsx: %v", err)
	}
	purchaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		_, _ = w.Write(purchaseBody)
	}))
	defer purchaseServer.Close()

	task, err := WeeklyBusinessDiagnosticsTask(WeeklyBusinessDiagnosticsParams{
		ID:          "weekly-east",
		Workspace:   dir,
		RunTime:     "03:30",
		Region:      "east",
		TargetStart: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		TargetEnd:   time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC),
		Sources: []BusinessDataSource{
			{ID: "sales", Kind: BusinessDataSales, Type: BusinessSourceCSV, Path: salesPath},
			{ID: "inventory", Kind: BusinessDataInventory, Type: BusinessSourceXLSX, Path: inventoryPath},
			{ID: "purchase", Kind: BusinessDataPurchase, Type: BusinessSourceHTTP, URL: purchaseServer.URL, Format: "xlsx"},
			{ID: "finance", Kind: BusinessDataFinance, Type: BusinessSourceCSV, Path: financePath},
			{ID: "returns", Kind: BusinessDataReturns, Type: BusinessSourceCSV, Path: returnsPath},
		},
	})
	if err != nil {
		t.Fatalf("WeeklyBusinessDiagnosticsTask returned error: %v", err)
	}

	store, manifest, paths := runBusinessDiagnosticsThroughEngine(t, task)
	if manifest.Status != RunStatusSucceeded {
		t.Fatalf("manifest status = %q, want succeeded: %+v", manifest.Status, manifest)
	}
	body, err := store.ReadResult(task.ID, manifest.RunID)
	if err != nil {
		t.Fatalf("ReadResult returned error: %v", err)
	}
	for _, want := range []string{"原始输入摘要", "清洗结果", "数据质量", "指标结果", "建议方案"} {
		if !strings.Contains(body, want) {
			t.Fatalf("result.md missing %q:\n%s", want, body)
		}
	}

	artifact := readBusinessArtifact(t, filepath.Join(paths.Dir, businessDiagnosticsArtifactName))
	if artifact.Blocked {
		t.Fatalf("artifact blocked = true, want false: %+v", artifact.QualityIssues)
	}
	wantMetrics := []string{
		"order_turnover_rate",
		"inventory_remaining_qty",
		"overstock_risk_days",
		"purchase_consumption_ratio",
		"week_over_week_sales_amount_change",
		"year_over_year_sales_amount_change",
	}
	for _, name := range wantMetrics {
		metric := findBusinessMetric(artifact.Metrics, name)
		if metric == nil {
			t.Fatalf("missing metric %q in %+v", name, artifact.Metrics)
		}
		if len(metric.Trace) == 0 {
			t.Fatalf("metric %q has no source trace: %+v", name, metric)
		}
	}
	if got := len(artifact.Dataset.Records); got != 9 {
		t.Fatalf("normalized records = %d, want 9", got)
	}
}

func TestBusinessDiagnosticsRunnerBlocksInsufficientData(t *testing.T) {
	dir := t.TempDir()
	salesPath := filepath.Join(dir, "sales.csv")
	mustWriteFile(t, salesPath, "date,order_id,sku,quantity,region\n2026-06-16,o-1,sku-1,10,east\n")
	task, err := WeeklyBusinessDiagnosticsTask(WeeklyBusinessDiagnosticsParams{
		ID:          "weekly-east",
		Workspace:   dir,
		RunTime:     "03:30",
		Region:      "east",
		TargetStart: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		TargetEnd:   time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC),
		Sources: []BusinessDataSource{
			{ID: "sales", Kind: BusinessDataSales, Type: BusinessSourceCSV, Path: salesPath},
		},
	})
	if err != nil {
		t.Fatalf("WeeklyBusinessDiagnosticsTask returned error: %v", err)
	}

	_, manifest, paths := runBusinessDiagnosticsThroughEngine(t, task)
	if manifest.Status != RunStatusBlocked || manifest.ErrorCode != RunErrorDataQualityBlocked {
		t.Fatalf("manifest = %+v, want blocked data quality error", manifest)
	}
	body, err := os.ReadFile(paths.Result)
	if err != nil {
		t.Fatalf("ReadFile result.md: %v", err)
	}
	if !strings.Contains(string(body), "已阻塞") || strings.Contains(string(body), "建议方案") {
		t.Fatalf("blocked result should explain block without recommendations:\n%s", body)
	}
	artifact := readBusinessArtifact(t, filepath.Join(paths.Dir, businessDiagnosticsArtifactName))
	if !artifact.Blocked || len(artifact.Metrics) != 0 {
		t.Fatalf("artifact = %+v, want blocked with no metrics", artifact)
	}
}

func TestBusinessDiagnosticsHandlesZeroHistoricalSalesBaseline(t *testing.T) {
	report, err := RunBusinessDiagnostics(context.Background(), BusinessDiagnosticsConfig{
		Region:      "east",
		TargetStart: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		TargetEnd:   time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC),
		Sources: []BusinessDataSource{
			{
				ID:   "sales-tool",
				Kind: BusinessDataSales,
				Type: BusinessSourceMCP,
				Rows: []map[string]any{
					{"date": "2025-06-16", "order_id": "o-yoy", "sku": "sku-1", "quantity": 1, "amount": 0, "region": "east"},
					{"date": "2026-06-09", "order_id": "o-prev", "sku": "sku-1", "quantity": 1, "amount": 0, "region": "east"},
					{"date": "2026-06-16", "order_id": "o-cur", "sku": "sku-1", "quantity": 5, "amount": 100, "region": "east"},
				},
			},
			{
				ID:   "inventory-tool",
				Kind: BusinessDataInventory,
				Type: BusinessSourceMCP,
				Rows: []map[string]any{
					{"date": "2026-06-16", "sku": "sku-1", "on_hand_qty": 12, "reserved_qty": 2, "warehouse": "wh", "region": "east"},
				},
			},
			{
				ID:   "purchase-tool",
				Kind: BusinessDataPurchase,
				Type: BusinessSourceMCP,
				Rows: []map[string]any{
					{"date": "2026-06-16", "purchase_id": "p-1", "sku": "sku-1", "quantity": 10, "amount": 20, "supplier": "s", "region": "east"},
				},
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("RunBusinessDiagnostics returned error: %v", err)
	}
	if report.Blocked {
		t.Fatalf("report blocked unexpectedly: %+v", report.QualityIssues)
	}
	for _, metric := range report.Metrics {
		if math.IsInf(metric.Value, 0) || math.IsNaN(metric.Value) {
			t.Fatalf("metric %q value = %v, want finite value", metric.Name, metric.Value)
		}
	}
}

func TestBusinessDiagnosticsNormalizesMCPRows(t *testing.T) {
	report, err := RunBusinessDiagnostics(context.Background(), BusinessDiagnosticsConfig{
		Region:      "east",
		TargetStart: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		TargetEnd:   time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC),
		Sources: []BusinessDataSource{
			{
				ID:   "sales-tool",
				Kind: BusinessDataSales,
				Type: BusinessSourceMCP,
				Rows: []map[string]any{
					{"date": "2025-06-16", "order_id": "o-yoy", "sku": "sku-1", "quantity": 3, "amount": 30, "region": "east"},
					{"date": "2026-06-09", "order_id": "o-prev", "sku": "sku-1", "quantity": 4, "amount": 40, "region": "east"},
					{"date": "2026-06-16", "order_id": "o-cur", "sku": "sku-1", "quantity": 5, "amount": 50, "region": "east"},
				},
			},
			{
				ID:   "inventory-tool",
				Kind: BusinessDataInventory,
				Type: BusinessSourceMCP,
				Rows: []map[string]any{
					{"date": "2026-06-16", "sku": "sku-1", "on_hand_qty": 12, "reserved_qty": 2, "warehouse": "wh", "region": "east"},
				},
			},
			{
				ID:   "purchase-tool",
				Kind: BusinessDataPurchase,
				Type: BusinessSourceMCP,
				Rows: []map[string]any{
					{"date": "2026-06-16", "purchase_id": "p-1", "sku": "sku-1", "quantity": 10, "amount": 20, "supplier": "s", "region": "east"},
				},
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("RunBusinessDiagnostics returned error: %v", err)
	}
	if report.Blocked {
		t.Fatalf("report blocked unexpectedly: %+v", report.QualityIssues)
	}
	if len(report.Dataset.Records) != 5 {
		t.Fatalf("records = %d, want 5", len(report.Dataset.Records))
	}
}

func TestBusinessDiagnosticsRunnerDelegatesNonBusinessTask(t *testing.T) {
	next := &mockRunner{result: RunResult{Status: RunStatusSucceeded, Summary: "delegated"}}
	result, err := NewBusinessDiagnosticsRunner(next).Run(context.Background(), RunRequest{RunDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if next.count() != 1 || result.Summary != "delegated" {
		t.Fatalf("delegation count=%d result=%+v, want one delegated run", next.count(), result)
	}
}

func runBusinessDiagnosticsThroughEngine(t *testing.T, task Task) (*Store, RunManifest, RunPaths) {
	t.Helper()
	now := task.BusinessDiagnostics.TargetEnd.Add(time.Hour)
	task.Schedule = Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)}
	task.CreatedAt = now.Add(-time.Hour)
	task.UpdatedAt = now.Add(-time.Hour)
	store := newEngineTestStore(t, task)
	engine := newEngineTestEngine(store, "test:business", NewBusinessDiagnosticsRunner(nil))
	engine.now = func() time.Time { return now }
	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	runs, err := store.ListRuns(task.ID, 1)
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1", len(runs))
	}
	manifest, err := store.ReadRunManifest(task.ID, runs[0].RunID)
	if err != nil {
		t.Fatalf("ReadRunManifest returned error: %v", err)
	}
	paths, err := store.RunPaths(task.ID, manifest.RunID)
	if err != nil {
		t.Fatalf("RunPaths returned error: %v", err)
	}
	return store, manifest, paths
}

func readBusinessArtifact(t *testing.T, path string) BusinessDiagnosticsReport {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile business artifact: %v", err)
	}
	var report BusinessDiagnosticsReport
	if err := json.Unmarshal(b, &report); err != nil {
		t.Fatalf("Unmarshal business artifact: %v\n%s", err, b)
	}
	return report
}

func findBusinessMetric(metrics []BusinessMetric, name string) *BusinessMetric {
	for i := range metrics {
		if metrics[i].Name == name {
			return &metrics[i]
		}
	}
	return nil
}

func mustWriteFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func writeXLSX(t *testing.T, path string, rows [][]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create xlsx: %v", err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	writeZipEntry(t, zw, "[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`)
	writeZipEntry(t, zw, "_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`)
	writeZipEntry(t, zw, "xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/></sheets></workbook>`)
	writeZipEntry(t, zw, "xl/worksheets/sheet1.xml", xlsxSheetXML(rows))
}

func writeZipEntry(t *testing.T, zw *zip.Writer, name, body string) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("Create zip entry %s: %v", name, err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatalf("Write zip entry %s: %v", name, err)
	}
}

func xlsxSheetXML(rows [][]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for i, row := range rows {
		b.WriteString(`<row r="`)
		b.WriteString(string(rune('1' + i)))
		b.WriteString(`">`)
		for j, value := range row {
			b.WriteString(`<c r="`)
			b.WriteByte(byte('A' + j))
			b.WriteString(string(rune('1' + i)))
			b.WriteString(`" t="inlineStr"><is><t>`)
			b.WriteString(value)
			b.WriteString(`</t></is></c>`)
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}
