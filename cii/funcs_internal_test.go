package cii

import (
	"testing"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/schema"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// importVATBreakdownLines only fills in an invoice that has no lines.
func TestImportVATBreakdownLines(t *testing.T) {
	pct := num.MakePercentage(200, 3)
	inv := &bill.Invoice{
		Tags:  tax.WithTags(tax.TagBypass),
		Lines: []*bill.Line{{Quantity: num.MakeAmount(1, 0), Item: &org.Item{Name: "Item"}}},
		Totals: &bill.Totals{Taxes: &tax.Total{Categories: []*tax.CategoryTotal{{
			Code:  tax.CategoryVAT,
			Rates: []*tax.RateTotal{{Base: num.MakeAmount(1000, 2), Percent: &pct}},
		}}}},
	}
	env := gobl.NewEnvelope()
	var err error
	env.Document, err = schema.NewObject(inv)
	require.NoError(t, err)

	require.NoError(t, importVATBreakdownLines(nil, nil, env))
	inv = env.Extract().(*bill.Invoice)
	assert.Len(t, inv.Lines, 1)
	assert.Equal(t, "Item", inv.Lines[0].Item.Name)
	assert.True(t, inv.HasTags(tax.TagBypass))
}
