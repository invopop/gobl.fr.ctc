package flow6

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// normalizePayment sets the status code and default party roles. The
// supplier is always the seller (SE) and the customer the buyer (BY);
// direction is derived from the payment type at generation, not by
// flipping roles here.
func normalizePayment(pmt *bill.Payment) {
	if pmt == nil {
		return
	}
	switch pmt.Type {
	case bill.PaymentTypeAdvice:
		pmt.Ext = pmt.Ext.Set(ExtKeyStatus, "211")
		setPartyRoleDefault(pmt.Supplier, RoleSeller)
		setPartyRoleDefault(pmt.Customer, RoleBuyer)
		pmt.Ext = pmt.Ext.SetOneOf(ExtKeyCondition,
			ConditionAmountPaid, ConditionAmountReceived, ConditionAmountRemaining,
		)
	case bill.PaymentTypeReceipt:
		pmt.Ext = pmt.Ext.Set(ExtKeyStatus, "212")
		setPartyRoleDefault(pmt.Supplier, RoleSeller)
		setPartyRoleDefault(pmt.Customer, RoleBuyer)
		pmt.Ext = pmt.Ext.SetOneOf(ExtKeyCondition,
			ConditionAmountReceived, ConditionAmountPaid, ConditionAmountRemaining,
		)
	}
	for _, line := range pmt.Lines {
		if line != nil {
			migrateDocRefType(line.Document)
		}
	}
}

func billPaymentRules() *rules.Set {
	return rules.For(new(bill.Payment),
		rules.Field("type",
			rules.Assert("01", "payment type must be 'advice' (CDAR 211) or 'receipt' (CDAR 212); 'request' is not a Flow 6 CDV event",
				is.In(bill.PaymentTypeAdvice, bill.PaymentTypeReceipt),
			),
		),
		rules.Field("code",
			rules.Assert("17", "payment code is required as the CDV document identifier (ram:ID; BR-FR-CDV-03, MDT-4)",
				is.Present,
			),
		),
		rules.Field("supplier",
			rules.Assert("02", "payment supplier is required (BR-FR-CDV-13)",
				is.Present,
			),
			rules.Field("identities",
				rules.Assert("03", "payment supplier must have an identity with ISO/IEC 6523 scheme 0002 (SIREN)",
					org.IdentitiesExtensionIn(iso.ExtKeySchemeID, identitySchemeIDSIREN),
				),
				rules.Each(
					rules.Field("ext",
						rules.Assert("04", "payment supplier identity ext iso-scheme-id must be in the Flow 6 allow-list; STC 0231 is a Flow 2 invoice concept",
							tax.ExtensionsHasCodes(iso.ExtKeySchemeID, allowedFlow6IdentitySchemes...),
						),
					),
				),
			),
		),
		rules.Field("customer",
			rules.Assert("05", "payment customer is required (BR-FR-CDV-CL-04)",
				is.Present,
			),
			rules.Field("identities",
				rules.Assert("06", "payment customer must have an identity with ISO/IEC 6523 scheme 0002 (SIREN)",
					org.IdentitiesExtensionIn(iso.ExtKeySchemeID, identitySchemeIDSIREN),
				),
				rules.Each(
					rules.Field("ext",
						rules.Assert("07", "payment customer identity ext iso-scheme-id must be in the Flow 6 allow-list; STC 0231 is a Flow 2 invoice concept",
							tax.ExtensionsHasCodes(iso.ExtKeySchemeID, allowedFlow6IdentitySchemes...),
						),
					),
				),
			),
		),
		rules.Field("payee",
			rules.Field("identities",
				rules.Each(
					rules.Field("ext",
						rules.Assert("08", "payment payee identity ext iso-scheme-id must be in the Flow 6 allow-list; STC 0231 is a Flow 2 invoice concept",
							tax.ExtensionsHasCodes(iso.ExtKeySchemeID, allowedFlow6IdentitySchemes...),
						),
					),
				),
			),
		),
		rules.Field("lines",
			rules.Assert("09", "payment lines must contain exactly one entry (a CDV references a single invoice)",
				is.Func("exactly one line", paymentHasExactlyOneLine),
			),
			rules.Each(
				rules.Field("document",
					rules.Assert("10", "payment line document is required (BR-FR-CDV-10)",
						is.Present,
					),
					rules.Field("code",
						rules.Assert("11", "payment line document code is required (BR-FR-CDV-10)",
							is.Present,
						),
					),
					rules.Field("issue_date",
						rules.Assert("12", "payment line document issue_date is required (BR-FR-CDV-11)",
							is.Present,
						),
					),
					rules.Assert("17", "payment line document must carry the untdid-document-type extension (MDT-91) with a valid invoice type code",
						is.Func("valid untdid-document-type", docRefHasValidType),
					),
				),
			),
		),
		rules.When(
			bill.PaymentTypeIn(bill.PaymentTypeReceipt),
			rules.Field("lines",
				rules.Assert("18", "a payment receipt must show the applicable VAT rate; the rate may be exempt",
					is.Func("line has VAT tax breakdown", paymentLineHasVATTax),
				),
			),
		),
		rules.Field("ext",
			rules.Assert("13", "payment ext fr-ctc-flow6-status must be a Payment-applicable ProcessConditionCode (211 advice or 212 receipt); codes 200-210, 213 belong on bill.Status",
				tax.ExtensionsHasCodes(ExtKeyStatus, paymentProcessCodes...),
			),
			rules.Assert("14", "payment ext fr-ctc-flow6-condition must be a Payment-applicable CharacteristicTypeCode (MEN, MPA, RAP); status-only codes (CBB, DIV, DVA, MAJ, MAP, MAPTTC, MNA, MNATTC, ESC, RAB, REM) belong on a bill.Reason under bill.Status",
				tax.ExtensionsHasCodes(ExtKeyCondition, paymentConditionCodes...),
			),
		),
		rules.When(
			bill.PaymentTypeIn(bill.PaymentTypeAdvice),
			rules.Field("ext",
				rules.Assert("15", "payment ext fr-ctc-flow6-status for an advice payment must be ProcessConditionCode 211 (Paiement transmis)",
					tax.ExtensionsHasCodes(ExtKeyStatus, "211"),
				),
			),
		),
		rules.When(
			bill.PaymentTypeIn(bill.PaymentTypeReceipt),
			rules.Field("ext",
				rules.Assert("16", "payment ext fr-ctc-flow6-status for a receipt payment must be ProcessConditionCode 212 (Encaissée)",
					tax.ExtensionsHasCodes(ExtKeyStatus, "212"),
				),
			),
		),
	)
}

func paymentHasExactlyOneLine(v any) bool {
	lines, ok := v.([]*bill.PaymentLine)
	if !ok {
		return false
	}
	return len(lines) == 1
}

// paymentLineHasVATTax reports whether the single line has a VAT breakdown.
// The rate may be exempt (nil percent); only its presence matters.
func paymentLineHasVATTax(v any) bool {
	lines, ok := v.([]*bill.PaymentLine)
	if !ok || len(lines) != 1 || lines[0] == nil {
		return false
	}
	t := lines[0].Tax
	if t == nil {
		return false
	}
	for _, cat := range t.Categories {
		if cat != nil && cat.Code == tax.CategoryVAT {
			return len(cat.Rates) > 0
		}
	}
	return false
}
