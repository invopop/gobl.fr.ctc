package flow10

import (
	"testing"

	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaxComboCountryIsDomestic(t *testing.T) {
	assert.False(t, taxComboCountryIsDomestic("nope"))
	assert.True(t, taxComboCountryIsDomestic(l10n.TaxCountryCode("")))
	assert.True(t, taxComboCountryIsDomestic(l10n.FR.Tax()))
	assert.False(t, taxComboCountryIsDomestic(l10n.TaxCountryCode("GB")))
}

func TestTaxComboRulesBareCombo(t *testing.T) {
	t.Run("domestic standard rate passes", func(t *testing.T) {
		c := &tax.Combo{Category: tax.CategoryVAT, Percent: pct("20%")}
		assert.NoError(t, rules.Validate(c, tax.AddonContext(V1)))
	})

	t.Run("domestic outside-scope key passes", func(t *testing.T) {
		c := &tax.Combo{Category: tax.CategoryVAT, Key: tax.KeyOutsideScope}
		assert.NoError(t, rules.Validate(c, tax.AddonContext(V1)))
	})

	t.Run("explicit France passes", func(t *testing.T) {
		c := &tax.Combo{Category: tax.CategoryVAT, Country: l10n.FR.Tax(), Percent: pct("20%")}
		assert.NoError(t, rules.Validate(c, tax.AddonContext(V1)))
	})

	t.Run("GB VAT rejected", func(t *testing.T) {
		c := &tax.Combo{Category: tax.CategoryVAT, Country: "GB", Percent: pct("20%")}
		err := rules.Validate(c, tax.AddonContext(V1))
		assert.ErrorContains(t, err, "foreign VAT jurisdiction")
	})

	t.Run("CH VAT rejected", func(t *testing.T) {
		c := &tax.Combo{Category: tax.CategoryVAT, Country: "CH", Percent: pct("8.1%")}
		err := rules.Validate(c, tax.AddonContext(V1))
		assert.ErrorContains(t, err, "foreign VAT jurisdiction")
	})

	t.Run("non-VAT category rejected", func(t *testing.T) {
		c := &tax.Combo{Category: "IGST"}
		err := rules.Validate(c, tax.AddonContext(V1))
		assert.ErrorContains(t, err, "tax category must be VAT")
	})
}

func TestInvoiceRejectsForeignTaxCombo(t *testing.T) {
	t.Run("GB VAT rejected", func(t *testing.T) {
		inv := testInvoiceB2BCrossBorder(t)
		inv.Lines[0].Taxes[0].Country = "GB"
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "foreign VAT jurisdiction")
	})

	t.Run("CH VAT rejected", func(t *testing.T) {
		inv := testInvoiceB2BCrossBorder(t)
		inv.Lines[0].Taxes[0].Country = "CH"
		inv.Lines[0].Taxes[0].Percent = num.NewPercentage(81, 3)
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "foreign VAT jurisdiction")
	})

	t.Run("non-VAT category rejected", func(t *testing.T) {
		inv := testInvoiceB2BCrossBorder(t)
		inv.Lines[0].Taxes[0].Category = "IGST"
		inv.Lines[0].Taxes[0].Percent = nil
		// No Calculate: FR only defines VAT, so Calculate itself would hard-fail here.
		assert.ErrorContains(t, rules.Validate(inv), "tax category must be VAT")
	})

	t.Run("explicit France passes", func(t *testing.T) {
		inv := testInvoiceB2BCrossBorder(t)
		inv.Lines[0].Taxes[0].Country = l10n.FR.Tax()
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})
}

// PaymentLine.Tax is a computed tax.Total/tax.RateTotal, not a tax.Combo, so the combo rule can't see it.
func TestPaymentValidationUnaffectedByTaxComboRules(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Lines[0].Tax = &tax.Total{Categories: []*tax.CategoryTotal{
		{Code: tax.CategoryVAT, Rates: []*tax.RateTotal{
			{Country: "GB", Percent: pct("20%"), Base: num.MakeAmount(10000, 2)},
		}},
	}}
	require.NoError(t, rules.Validate(pmt))
}
