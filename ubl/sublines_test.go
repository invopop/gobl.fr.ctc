package ubl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl"
	frubl "github.com/invopop/gobl.fr.ctc/ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/convert"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubLinesRoundTrip writes line breakdowns out as sub-invoice lines and
// reads them back: the breakdowns and every total must survive.
func TestSubLinesRoundTrip(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(getConvertPath(), "france-extended", "invoice-sub-lines.json"))
	require.NoError(t, err)
	env := new(gobl.Envelope)
	require.NoError(t, json.Unmarshal(data, env))
	require.NoError(t, env.Calculate())
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)

	res, err := convert.Export(env, frubl.KeyExtended)
	require.NoError(t, err)
	imported, err := convert.Import(res.Data)
	require.NoError(t, err)
	penv := imported.Envelope
	out, ok := penv.Extract().(*bill.Invoice)
	require.True(t, ok)

	assert.False(t, out.HasTags(tax.TagBypass))
	require.Len(t, out.Lines, len(inv.Lines))
	for i, l := range inv.Lines {
		got := out.Lines[i]
		assert.Equal(t, l.Total.String(), got.Total.String(), "line %d total", i+1)
		require.Len(t, got.Breakdown, len(l.Breakdown), "line %d breakdown", i+1)
		for j, sl := range l.Breakdown {
			assert.Equal(t, sl.Item.Name, got.Breakdown[j].Item.Name)
			assert.True(t, sl.Quantity.Equals(got.Breakdown[j].Quantity), "sub-line %d.%d quantity", i+1, j+1)
		}
	}
	assert.Equal(t, inv.Totals.Payable.String(), out.Totals.Payable.String())
}
