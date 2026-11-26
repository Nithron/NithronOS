package alerts

import "testing"

func TestAlertTemplate(t *testing.T) {
	rule := &AlertRule{Name: "CPU", Metric: "cpu", Threshold: 90, Template: "{{.Name}} is {{.State}}: {{.Value}} > {{.Threshold}}"}
	e := &Engine{}
	if got := e.formatMessage(rule, 95.5, "firing"); got != "CPU is firing: 95.50 > 90.00" {
		t.Fatalf("got %q", got)
	}
	// A broken template falls back to the raw text instead of failing.
	rule.Template = "{{.Nope}"
	if got := e.formatMessage(rule, 1, "firing"); got != "{{.Nope}" {
		t.Fatalf("broken template: %q", got)
	}
}
