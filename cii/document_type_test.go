package cii_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl.fr.ctc/addon/flow2"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportDocumentType(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(getParsePath(), "peppol-france-extended", "invoice-standard.xml"))
	require.NoError(t, err)

	tests := []struct {
		code string
		typ  cbc.Key
		tags []cbc.Key
	}{
		{"386", bill.InvoiceTypeStandard, []cbc.Key{tax.TagPrepayment}},
		{"393", bill.InvoiceTypeStandard, []cbc.Key{tax.TagFactoring}},
		{"500", bill.InvoiceTypeStandard, []cbc.Key{tax.TagSelfBilled, tax.TagPrepayment}},
		{"501", bill.InvoiceTypeStandard, []cbc.Key{tax.TagSelfBilled, tax.TagFactoring}},
		{"471", bill.InvoiceTypeCorrective, []cbc.Key{tax.TagSelfBilled}},
		{"472", bill.InvoiceTypeCorrective, []cbc.Key{tax.TagFactoring}},
		{"473", bill.InvoiceTypeCorrective, []cbc.Key{tax.TagSelfBilled, tax.TagFactoring}},
		{"261", bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagSelfBilled}},
		{"396", bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagFactoring}},
		{"502", bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagSelfBilled, tax.TagFactoring}},
		{"503", bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagPrepayment}},
		{"262", bill.InvoiceTypeCreditNote, []cbc.Key{flow2.TagGlobal}},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			data := strings.Replace(string(raw),
				"<ram:TypeCode>380</ram:TypeCode>",
				"<ram:TypeCode>"+tt.code+"</ram:TypeCode>", 1)
			imported, err := convert.Import([]byte(data))
			require.NoError(t, err)
			inv, ok := imported.Envelope.Extract().(*bill.Invoice)
			require.True(t, ok)

			assert.Equal(t, tt.typ, inv.Type)
			assert.Equal(t, tt.tags, inv.Tags.List)
			assert.Equal(t, cbc.Code(tt.code), inv.Tax.Ext.Get(untdid.ExtKeyDocumentType),
				"the scenario must regenerate the stated code")
		})
	}
}
