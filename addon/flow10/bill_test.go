package flow10

import (
	"testing"

	"github.com/invopop/gobl.fr.ctc/addon/dgfip"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pct(s string) *num.Percentage {
	p, err := num.PercentageFromString(s)
	if err != nil {
		panic(err)
	}
	return &p
}

func TestNormalizeBillingMode(t *testing.T) {
	t.Run("keeps caller value", func(t *testing.T) {
		inv := &bill.Invoice{Tax: &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{dgfip.ExtKeyBillingMode: dgfip.BillingModeB2})}}
		normalizeBillingMode(inv)
		assert.Equal(t, dgfip.BillingModeB2, inv.Tax.Ext.Get(dgfip.ExtKeyBillingMode))
	})
	t.Run("M1 when unpaid", func(t *testing.T) {
		inv := &bill.Invoice{}
		normalizeBillingMode(inv)
		require.NotNil(t, inv.Tax)
		assert.Equal(t, dgfip.BillingModeM1, inv.Tax.Ext.Get(dgfip.ExtKeyBillingMode))
	})
	t.Run("M2 when fully paid", func(t *testing.T) {
		due := num.MakeAmount(0, 2)
		inv := &bill.Invoice{Totals: &bill.Totals{Due: &due}}
		normalizeBillingMode(inv)
		assert.Equal(t, dgfip.BillingModeM2, inv.Tax.Ext.Get(dgfip.ExtKeyBillingMode))
	})
}

func TestNormalizeB2CCategoryOnInvoice(t *testing.T) {
	t.Run("defaults to TNT1", func(t *testing.T) {
		inv := &bill.Invoice{}
		normalizeB2CCategoryOnInvoice(inv)
		require.NotNil(t, inv.Tax)
		assert.Equal(t, B2CCategoryNotTaxable, inv.Tax.Ext.Get(ExtKeyB2CCategory))
	})
	t.Run("keeps caller value", func(t *testing.T) {
		inv := &bill.Invoice{Tax: &bill.Tax{Ext: tax.ExtensionsOf(cbc.CodeMap{ExtKeyB2CCategory: B2CCategoryGoods})}}
		normalizeB2CCategoryOnInvoice(inv)
		assert.Equal(t, B2CCategoryGoods, inv.Tax.Ext.Get(ExtKeyB2CCategory))
	})
}

func TestNormalizeInvoiceDispatch(t *testing.T) {
	assert.NotPanics(t, func() { normalizeInvoice(nil) })

	t.Run("B2C invoice gets a category, no billing mode", func(t *testing.T) {
		inv := &bill.Invoice{} // no customer => B2C
		normalizeInvoice(inv)
		assert.Equal(t, B2CCategoryNotTaxable, inv.Tax.Ext.Get(ExtKeyB2CCategory))
		assert.Empty(t, inv.Tax.Ext.Get(dgfip.ExtKeyBillingMode))
	})

	t.Run("customer without legal identity is B2C", func(t *testing.T) {
		inv := &bill.Invoice{Customer: &org.Party{Name: "Jean Dupont"}}
		normalizeInvoice(inv)
		assert.Equal(t, B2CCategoryNotTaxable, inv.Tax.Ext.Get(ExtKeyB2CCategory))
		assert.Empty(t, inv.Tax.Ext.Get(dgfip.ExtKeyBillingMode))
	})

	t.Run("customer with legal identity is B2B", func(t *testing.T) {
		inv := &bill.Invoice{Customer: &org.Party{
			Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "732829320")},
		}}
		normalizeInvoice(inv)
		assert.Empty(t, inv.Tax.Ext.Get(ExtKeyB2CCategory))
		assert.NotEmpty(t, inv.Tax.Ext.Get(dgfip.ExtKeyBillingMode))
	})
}

func TestInvoiceIsB2CDoc(t *testing.T) {
	assert.False(t, invoiceIsB2CDoc(nil))
	assert.True(t, invoiceIsB2CDoc(&bill.Invoice{}))
	assert.True(t, invoiceIsB2CDoc(&bill.Invoice{Customer: &org.Party{Name: "Jean Dupont"}}))
	assert.False(t, invoiceIsB2CDoc(&bill.Invoice{Customer: &org.Party{
		Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "732829320")},
	}}))
}

func TestInvoiceVATPercentsAllowed(t *testing.T) {
	assert.True(t, invoiceVATPercentsAllowed("wrong-type"))
	assert.True(t, invoiceVATPercentsAllowed((*bill.Invoice)(nil)))

	mk := func(p string) *bill.Invoice {
		return &bill.Invoice{Lines: []*bill.Line{
			{Taxes: tax.Set{{Category: tax.CategoryVAT, Percent: pct(p)}}},
		}}
	}
	assert.True(t, invoiceVATPercentsAllowed(mk("20%")))
	assert.False(t, invoiceVATPercentsAllowed(mk("17%")))
	// nil line / nil combo / non-VAT / nil percent are skipped
	inv := &bill.Invoice{Lines: []*bill.Line{
		nil,
		{Taxes: tax.Set{nil, {Category: "OTHER"}, {Category: tax.CategoryVAT}}},
	}}
	assert.True(t, invoiceVATPercentsAllowed(inv))
}

func TestInvoiceHasExemptCombo(t *testing.T) {
	assert.False(t, invoiceHasExemptCombo("wrong-type"))
	assert.False(t, invoiceHasExemptCombo((*bill.Invoice)(nil)))

	exempt := &bill.Invoice{Lines: []*bill.Line{
		{Taxes: tax.Set{{Category: tax.CategoryVAT, Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: "E"})}}},
	}}
	assert.True(t, invoiceHasExemptCombo(exempt))

	notExempt := &bill.Invoice{Lines: []*bill.Line{
		nil,
		{Taxes: tax.Set{nil, {Category: "OTHER"}, {Category: tax.CategoryVAT, Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: "S"})}}},
	}}
	assert.False(t, invoiceHasExemptCombo(notExempt))
}

func TestPaymentVATPercentsAllowed(t *testing.T) {
	assert.True(t, paymentVATPercentsAllowed("wrong-type"))
	assert.True(t, paymentVATPercentsAllowed((*bill.Payment)(nil)))

	mk := func(p string) *bill.Payment {
		return &bill.Payment{Lines: []*bill.PaymentLine{
			{Tax: &tax.Total{Categories: []*tax.CategoryTotal{
				{Code: tax.CategoryVAT, Rates: []*tax.RateTotal{{Percent: pct(p)}}},
			}}},
		}}
	}
	assert.True(t, paymentVATPercentsAllowed(mk("20%")))
	assert.False(t, paymentVATPercentsAllowed(mk("17%")))

	// nil line / nil tax / non-VAT category / nil rate / nil percent skipped
	pmt := &bill.Payment{Lines: []*bill.PaymentLine{
		nil,
		{Tax: nil},
		{Tax: &tax.Total{Categories: []*tax.CategoryTotal{
			nil,
			{Code: "OTHER", Rates: []*tax.RateTotal{{Percent: pct("99%")}}},
			{Code: tax.CategoryVAT, Rates: []*tax.RateTotal{nil, {Percent: nil}}},
		}}},
	}}
	assert.True(t, paymentVATPercentsAllowed(pmt))
}

// --- BR-FR-08: BT-23 billing mode is a closed list (v1.4 codes) ---------

func TestBillingModeAcceptsV14Codes(t *testing.T) {
	codes := []cbc.Code{
		dgfip.BillingModeS3, dgfip.BillingModeB8, dgfip.BillingModeS8,
		dgfip.BillingModeM8, dgfip.BillingModeB9, dgfip.BillingModeS9,
		dgfip.BillingModeM9,
	}
	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			inv := testInvoiceB2BCrossBorder(t)
			inv.Tax.Ext = inv.Tax.Ext.Set(dgfip.ExtKeyBillingMode, code)
			require.NoError(t, inv.Calculate())
			assert.NoError(t, rules.Validate(inv))
		})
	}
}

func TestBillingModeRejectsUnknownCode(t *testing.T) {
	inv := testInvoiceB2BCrossBorder(t)
	inv.Tax.Ext = inv.Tax.Ext.Set(dgfip.ExtKeyBillingMode, cbc.Code("X1"))
	require.NoError(t, inv.Calculate())
	err := rules.Validate(inv)
	assert.ErrorContains(t, err, "valid BT-23 billing mode code")
}
