package reporter

import (
	_ "embed"
	"fmt"
	htmltemplate "html/template"
	"io"
	"strings"
	"time"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
)

//go:embed templates/report.html
var htmlTemplateContent string

//go:embed templates/pico.min.css
var picoCSS string

// HTMLReporter writes experiment reports as a self-contained HTML file
// with embedded Pico CSS for styling.
type HTMLReporter struct {
	version string
}

// NewHTMLReporter creates an HTMLReporter with the given version string.
func NewHTMLReporter(version string) *HTMLReporter {
	return &HTMLReporter{version: version}
}

type htmlReportData struct {
	CSS             htmltemplate.CSS
	Generated       string
	Summary         ReportSummary
	PassRatePercent float64
	Experiments     []ExperimentReport
	Version         string
}

// WriteReport renders a collection of experiment reports as a self-contained HTML page.
func (h *HTMLReporter) WriteReport(w io.Writer, reports []ExperimentReport) error {
	if reports == nil {
		reports = []ExperimentReport{}
	}

	summary := ComputeSummary(reports)

	funcMap := htmltemplate.FuncMap{
		"verdictClass": verdictClass,
		"formatRecovery": func(d time.Duration, cycles int) string {
			return formatRecovery(d, cycles)
		},
	}

	tmpl, err := htmltemplate.New("report").Funcs(funcMap).Parse(htmlTemplateContent)
	if err != nil {
		return fmt.Errorf("parsing HTML template: %w", err)
	}

	data := htmlReportData{
		CSS:             htmltemplate.CSS(picoCSS),
		Generated:       time.Now().UTC().Format(time.RFC3339),
		Summary:         summary,
		PassRatePercent: summary.PassRate * 100,
		Experiments:     reports,
		Version:         h.version,
	}

	return tmpl.Execute(w, data)
}

func verdictClass(v v1alpha1.Verdict) string {
	switch v {
	case v1alpha1.Resilient:
		return "resilient"
	case v1alpha1.Degraded:
		return "degraded"
	case v1alpha1.Failed:
		return "failed"
	case v1alpha1.Inconclusive:
		return "inconclusive"
	default:
		return strings.ToLower(string(v))
	}
}

// Compile-time check that HTMLReporter implements Reporter.
var _ Reporter = (*HTMLReporter)(nil)
