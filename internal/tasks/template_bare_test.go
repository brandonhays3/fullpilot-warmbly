package tasks

import "testing"

func TestRewriteBareFieldRef(t *testing.T) {
	cases := map[string]string{
		"{{firstName}}":             "{{.FirstName}}",
		"{{ FirstName }}":           "{{.FirstName}}",
		"{{first_name}}":            "{{.FirstName}}",
		"{{company}}":               "{{.Company}}",
		"{{jobTitle}}":              "{{.jobTitle}}",
		"{{end}}":                   "{{end}}",
		"{{.FirstName}}":            "{{.FirstName}}",
		`{{or .FirstName "there"}}`: `{{or .FirstName "there"}}`,
	}
	for in, want := range cases {
		if got := rewriteBareFieldRef(in); got != want {
			t.Errorf("%s: got %s want %s", in, got, want)
		}
	}
}
