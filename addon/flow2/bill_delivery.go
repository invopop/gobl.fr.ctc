package flow2

import (
	"strings"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
)

// billDeliveryDetailsRules restricts the delivery terms code (EXT-FR-FE-185)
// to the values the French CTC Extended rules accept (BR-FREXT-CL-27).
func billDeliveryDetailsRules() *rules.Set {
	return rules.For(new(bill.DeliveryDetails),
		rules.Field("ext",
			rules.Assert("01", "delivery ext incoterm-delivery-terms must be one of "+incotermDeliveryTermsCodes()+" (BR-FREXT-CL-27)",
				tax.ExtensionHasValidCode(ExtKeyIncotermDeliveryTerms),
			),
		),
	)
}

// incotermDeliveryTermsCodes lists the codes of the incoterm-delivery-terms
// extension, so the rule message follows its definition.
func incotermDeliveryTermsCodes() string {
	return strings.Join(cbc.CodeStrings(cbc.DefinitionCodes(extIncotermDeliveryTerms.Values)), ", ")
}
