package reporter

import (
	"io"
	"time"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
)

// Reporter writes experiment reports in a specific format.
type Reporter interface {
	WriteReport(w io.Writer, reports []ExperimentReport) error
}

// ReportSummary holds aggregated verdict counts and timing.
type ReportSummary struct {
	Total        int           `json:"total"`
	Resilient    int           `json:"resilient"`
	Degraded     int           `json:"degraded"`
	Failed       int           `json:"failed"`
	Inconclusive int           `json:"inconclusive"`
	PassRate     float64       `json:"passRate"`
	TotalTime    time.Duration `json:"-"`
}

// ComputeSummary aggregates verdict counts from a slice of reports.
func ComputeSummary(reports []ExperimentReport) ReportSummary {
	s := ReportSummary{Total: len(reports)}
	for _, r := range reports {
		s.TotalTime += r.Evaluation.RecoveryTime
		switch r.Evaluation.Verdict {
		case v1alpha1.Resilient:
			s.Resilient++
		case v1alpha1.Degraded:
			s.Degraded++
		case v1alpha1.Failed:
			s.Failed++
		case v1alpha1.Inconclusive:
			s.Inconclusive++
		}
	}
	if s.Total > 0 {
		s.PassRate = float64(s.Resilient) / float64(s.Total)
	}
	return s
}
