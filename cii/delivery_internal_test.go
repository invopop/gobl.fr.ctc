package cii

import (
	"testing"

	"github.com/invopop/gobl/catalogues/icc"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

// TestDeliveryTermsCode checks the EXT-FR-FE-185 code is read from whichever
// delivery terms extension is set.
func TestDeliveryTermsCode(t *testing.T) {
	assert.Equal(t, cbc.Code("DAP"), deliveryTermsCode(tax.ExtensionsOf(cbc.CodeMap{icc.ExtKeyIncoterms2010Rule: "DAP"})))
	assert.Equal(t, cbc.Code("1"), deliveryTermsCode(tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyDeliveryTerms: "1"})))
	assert.Equal(t, cbc.CodeEmpty, deliveryTermsCode(tax.Extensions{}))
}

// TestDeliveryTermsKey checks UNTDID 4053 codes map to untdid-delivery-terms
// and every other code to icc-incoterms-2010-rule.
func TestDeliveryTermsKey(t *testing.T) {
	assert.Equal(t, untdid.ExtKeyDeliveryTerms, deliveryTermsKey("1"))
	assert.Equal(t, untdid.ExtKeyDeliveryTerms, deliveryTermsKey("2"))
	assert.Equal(t, icc.ExtKeyIncoterms2010Rule, deliveryTermsKey("DAP"))
	assert.Equal(t, icc.ExtKeyIncoterms2010Rule, deliveryTermsKey("XXX"))
}
