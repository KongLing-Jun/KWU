package modelbilling

import (
	"context"
	"testing"
)

func TestUsageReportBuilderKeepsExportedBoundaryAndReadsQuotaInput(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_reader")
	input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_01", "1", true, 1)}
	reader := &quotaReportBuilderReader{input: input}

	result, err := NewUsageReportBuilder(reader).BuildFinal(
		context.Background(), input.Task.Scope, input.Task.TaskID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if reader.reads != 1 || result.Disposition != ReportBuildDeliverable || result.Final == nil {
		t.Fatalf("reads=%d result=%+v", reader.reads, result)
	}
}

func TestUsageReportBuilderRejectsNilReaderAndMismatchedScope(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_reader_scope")
	input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_01", "1", true, 1)}

	if _, err := NewUsageReportBuilder(nil).BuildFinal(context.Background(), input.Task.Scope, input.Task.TaskID); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("nil reader error = %v", err)
	}

	wrong := input.Task.Scope
	wrong.ActorID = "other_actor"
	if _, err := NewUsageReportBuilder(&quotaReportBuilderReader{input: input}).BuildFinal(context.Background(), wrong, input.Task.TaskID); ErrorCodeOf(err) != ErrorScopeMismatch {
		t.Fatalf("scope error = %v", err)
	}
}

type quotaReportBuilderReader struct {
	input SubscriptionQuotaInput
	err   error
	reads int
}

func (r *quotaReportBuilderReader) ReadQuotaInput(_ context.Context, _ TaskScope, _ string) (SubscriptionQuotaInput, error) {
	r.reads++
	return r.input, r.err
}
