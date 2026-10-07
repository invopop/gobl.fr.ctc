package cii_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/phorm"

	goblcii "github.com/invopop/gobl.cii"
	frcii "github.com/invopop/gobl.fr.ctc/cii"
)

// franceInvoiceProbe pins one Flow 2 invoice fixture to the CII context it
// must validate against. Treat warnings as errors — the French CTC
// schematrons emit warnings that future releases promote to errors, so a
// clean run must have zero of both (mirrors cdar_probe_test.go).
type franceInvoiceProbe struct {
	name    string
	dir     string
	file    string
	context goblcii.Format
}

// franceInvoiceProbes drives TestProbeFranceInvoices over the same fixtures
// that feed TestConvertToInvoice, but through the warnings-as-errors gate.
var franceInvoiceProbes = []franceInvoiceProbe{
	{"CIUS/380", "peppol-france-cius", "invoice-standard.json", frcii.FormatPeppolFranceCIUS},
	{"CIUS/381", "peppol-france-cius", "credit-note.json", frcii.FormatPeppolFranceCIUS},
	{"FacturX/380", "peppol-france-facturx", "invoice-standard.json", frcii.FormatPeppolFranceFacturX},
	{"FacturX/381", "peppol-france-facturx", "credit-note.json", frcii.FormatPeppolFranceFacturX},
	{"Extended/380", "peppol-france-extended", "invoice-standard.json", frcii.FormatPeppolFranceExtended},
	{"Extended/381", "peppol-france-extended", "credit-note.json", frcii.FormatPeppolFranceExtended},
}

// TestProbeFranceInvoices converts each French CTC invoice fixture and
// pushes the generated CII XML through phive, failing on any error OR
// warning against the dedicated French schematron pinned on the context.
func TestProbeFranceInvoices(t *testing.T) {
	pc := phormClient(t)

	for _, p := range franceInvoiceProbes {
		t.Run(p.name, func(t *testing.T) {
			env := loadEnvelope(t, filepath.Join(p.dir, p.file))
			out, err := goblcii.ExportInvoice(env, goblcii.WithFormat(p.context))
			if err != nil {
				t.Fatalf("ConvertInvoice: %v", err)
			}
			data, err := goblcii.Encode(out)
			if err != nil {
				t.Fatalf("Bytes: %v", err)
			}
			resp, err := pc.ValidateXml(context.Background(), &phorm.ValidateXmlRequest{
				Vesid:      p.context.VESID,
				XmlContent: data,
			})
			if err != nil {
				t.Fatalf("phorm: %v", err)
			}
			var problems []string
			for _, r := range resp.Results {
				for _, e := range r.Errors {
					problems = append(problems, "ERROR: "+e.Message)
				}
				for _, w := range r.Warnings {
					problems = append(problems, "WARN:  "+w.Message)
				}
			}
			if len(problems) > 0 {
				t.Errorf("[%s] %s: %d problem(s) (warnings are treated as errors):\n%s",
					p.name, p.context.VESID, len(problems), strings.Join(problems, "\n\n"))
			}
		})
	}
}
