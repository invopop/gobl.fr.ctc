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

// importNoLines keeps the totals of a document without lines, even when they
// already matched the calculated ones and the base import set no bypass.
func TestImportNoLines(t *testing.T) {
	envelope := func(t *testing.T, inv *bill.Invoice) *gobl.Envelope {
		t.Helper()
		env := gobl.NewEnvelope()
		var err error
		env.Document, err = schema.NewObject(inv)
		require.NoError(t, err)
		return env
	}

	t.Run("an invoice without lines gets no-lines and bypass", func(t *testing.T) {
		inv := new(bill.Invoice)
		env := envelope(t, inv)
		require.NoError(t, importNoLines(nil, nil, env))
		inv = env.Extract().(*bill.Invoice)
		assert.True(t, inv.HasTags(tax.TagNoLines, tax.TagBypass), "tags %v", inv.GetTags())
	})

	t.Run("an invoice with lines is left alone", func(t *testing.T) {
		inv := &bill.Invoice{
			Lines: []*bill.Line{{Quantity: num.MakeAmount(1, 0), Item: &org.Item{Name: "Item"}}},
		}
		env := envelope(t, inv)
		require.NoError(t, importNoLines(nil, nil, env))
		inv = env.Extract().(*bill.Invoice)
		assert.Empty(t, inv.GetTags())
	})
}
