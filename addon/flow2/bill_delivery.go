package flow2

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/incoterms"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
)

// billDeliveryDetailsRules allows at most one delivery terms code (EXT-FR-FE-185).
func billDeliveryDetailsRules() *rules.Set {
	return rules.For(new(bill.DeliveryDetails),
		rules.Field("ext",
			rules.Assert("01", "delivery ext may include only one of icc-incoterm or untdid-delivery-terms (BR-FREXT-CL-27, EXT-FR-FE-185)",
				tax.ExtensionsAllowOneOf(incoterms.ExtKeyIncoterm, untdid.ExtKeyDeliveryTerms),
			),
			rules.Assert("02", "delivery ext icc-incoterm must be a valid Incoterms code (BR-FREXT-CL-27)",
				tax.ExtensionHasValidCode(incoterms.ExtKeyIncoterm),
			),
			rules.Assert("03", "delivery ext untdid-delivery-terms must be a valid UNTDID 4053 code (BR-FREXT-CL-27)",
				tax.ExtensionHasValidCode(untdid.ExtKeyDeliveryTerms),
			),
		),
	)
}
