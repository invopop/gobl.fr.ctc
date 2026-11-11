package flow2

import (
	"testing"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/icc"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeliveryTermsRules checks that the delivery terms (EXT-FR-FE-185) take
// either a valid Incoterms 2010 or UNTDID 4053 code, but not both (BR-FREXT-CL-27).
func TestDeliveryTermsRules(t *testing.T) {
	withTerms := func(t *testing.T, codes cbc.CodeMap) *bill.Invoice {
		t.Helper()
		inv := testInvoiceB2BStandard(t)
		inv.Delivery = &bill.DeliveryDetails{Ext: tax.ExtensionsOf(codes)}
		require.NoError(t, inv.Calculate())
		return inv
	}

	for _, code := range cbc.DefinitionCodes(tax.ExtensionForKey(icc.ExtKeyIncoterms2010).Values) {
		t.Run("accepts icc-incoterms-2010 "+code.String(), func(t *testing.T) {
			assert.NoError(t, rules.Validate(withTerms(t, cbc.CodeMap{icc.ExtKeyIncoterms2010: code})))
		})
	}

	for _, code := range cbc.DefinitionCodes(tax.ExtensionForKey(untdid.ExtKeyDeliveryTerms).Values) {
		t.Run("accepts untdid "+code.String(), func(t *testing.T) {
			assert.NoError(t, rules.Validate(withTerms(t, cbc.CodeMap{untdid.ExtKeyDeliveryTerms: code})))
		})
	}

	t.Run("accepts the 2010-only incoterm DAT", func(t *testing.T) {
		assert.NoError(t, rules.Validate(withTerms(t, cbc.CodeMap{icc.ExtKeyIncoterms2010: "DAT"})))
	})

	t.Run("rejects both together", func(t *testing.T) {
		inv := withTerms(t, cbc.CodeMap{icc.ExtKeyIncoterms2010: "DAP", untdid.ExtKeyDeliveryTerms: "1"})
		assert.ErrorContains(t, rules.Validate(inv), "delivery ext may include only one of icc-incoterms-2010 or untdid-delivery-terms")
	})

	for _, code := range []cbc.Code{"DPU", "DAF", "XXX", "1"} {
		t.Run("rejects icc-incoterms-2010 "+code.String(), func(t *testing.T) {
			inv := withTerms(t, cbc.CodeMap{icc.ExtKeyIncoterms2010: code})
			assert.ErrorContains(t, rules.Validate(inv), "delivery ext icc-incoterms-2010 must be a valid Incoterms 2010 code (BR-FREXT-CL-27)")
		})
	}

	for _, code := range []cbc.Code{"3", "DAP", "XXX"} {
		t.Run("rejects untdid "+code.String(), func(t *testing.T) {
			inv := withTerms(t, cbc.CodeMap{untdid.ExtKeyDeliveryTerms: code})
			assert.ErrorContains(t, rules.Validate(inv), "delivery ext untdid-delivery-terms must be a valid UNTDID 4053 code (BR-FREXT-CL-27)")
		})
	}

	t.Run("no delivery terms is fine", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		inv.Delivery = &bill.DeliveryDetails{}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})
}
