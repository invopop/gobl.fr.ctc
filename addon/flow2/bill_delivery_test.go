package flow2

import (
	"testing"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeliveryTermsRules checks that only the delivery terms codes the French
// CTC Extended rules accept pass validation (BR-FREXT-CL-27).
func TestDeliveryTermsRules(t *testing.T) {
	withTerms := func(t *testing.T, code cbc.Code) *bill.Invoice {
		t.Helper()
		inv := testInvoiceB2BStandard(t)
		inv.Delivery = &bill.DeliveryDetails{
			Ext: tax.ExtensionsOf(cbc.CodeMap{ExtKeyIncotermDeliveryTerms: code}),
		}
		require.NoError(t, inv.Calculate())
		return inv
	}

	for _, code := range []cbc.Code{"1", "2", "CFR", "CIF", "CIP", "CPT", "DAP", "DDP", "DPU", "EXW", "FAS", "FCA", "FOB"} {
		t.Run("accepts "+code.String(), func(t *testing.T) {
			assert.NoError(t, rules.Validate(withTerms(t, code)))
		})
	}

	for _, code := range []cbc.Code{"DAT", "DAF", "XXX"} {
		t.Run("rejects "+code.String(), func(t *testing.T) {
			assert.ErrorContains(t, rules.Validate(withTerms(t, code)), "delivery ext incoterm-delivery-terms must be one of")
		})
	}

	t.Run("no delivery terms is fine", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		inv.Delivery = &bill.DeliveryDetails{}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})
}
