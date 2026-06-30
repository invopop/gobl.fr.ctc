package flow6

import (
	"testing"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// paymentParty returns a French party with a SIREN identity, used as
// supplier or customer on a Flow 6 payment.
func paymentParty(name, siren string) *org.Party {
	return &org.Party{
		Name: name,
		Identities: []*org.Identity{{
			Code: cbc.Code(siren),
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				iso.ExtKeySchemeID: identitySchemeIDSIREN,
			}),
		}},
	}
}

func testPaymentReceipt(t *testing.T) *bill.Payment {
	t.Helper()
	issue := cal.MakeDate(2026, 5, 2)
	return &bill.Payment{
		Regime:    tax.WithRegime("FR"),
		Addons:    tax.WithAddons(V1),
		IssueDate: issue,
		Code:      "PMT-2026-0001",
		Currency:  "EUR",
		Type:      bill.PaymentTypeReceipt,
		Supplier:  paymentParty("VENDEUR SARL", "732829320"),
		Customer:  paymentParty("ACHETEUR SARL", "200000008"),
		Methods:   []*pay.Record{{Key: pay.MeansKeyCreditTransfer}},
		Lines: []*bill.PaymentLine{{
			Amount: num.MakeAmount(120000, 2),
			Tax: &tax.Total{Categories: []*tax.CategoryTotal{{
				Code:   tax.CategoryVAT,
				Amount: num.MakeAmount(20000, 2),
				Rates:  []*tax.RateTotal{{Base: num.MakeAmount(100000, 2), Percent: num.NewPercentage(20, 2), Amount: num.MakeAmount(20000, 2)}},
			}}},
			Document: &org.DocumentRef{
				Ext:       tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyDocumentType: "380"}),
				Code:      "2026-00042",
				IssueDate: cal.NewDate(2026, 4, 15),
			},
		}},
	}
}

func TestPaymentReceiptHappyPath(t *testing.T) {
	pmt := testPaymentReceipt(t)
	runNormalize(t, pmt)
	require.NoError(t, rules.Validate(pmt))
}

// BR-FR-CDV-03 (MDT-4): a CDV must carry a document identifier, which
// gobl.cii maps from Code (falling back to Series).
func TestPaymentRequiresDocumentID(t *testing.T) {
	t.Run("missing code and series is rejected", func(t *testing.T) {
		pmt := testPaymentReceipt(t)
		pmt.Code = ""
		pmt.Series = ""
		runNormalize(t, pmt)
		assert.ErrorContains(t, rules.Validate(pmt), "document identifier")
	})
	t.Run("series alone satisfies the identifier", func(t *testing.T) {
		pmt := testPaymentReceipt(t)
		pmt.Code = ""
		pmt.Series = "PMT-2026"
		runNormalize(t, pmt)
		require.NoError(t, rules.Validate(pmt))
	})
}

func TestPaymentReceiptSetsCDARStatusCode212(t *testing.T) {
	pmt := testPaymentReceipt(t)
	runNormalize(t, pmt)
	assert.Equal(t, cbc.Code("212"), pmt.Ext.Get(ExtKeyStatus))
}

// Default Condition extension on a receipt is MEN (Amount received).
func TestPaymentReceiptDefaultsConditionToMEN(t *testing.T) {
	pmt := testPaymentReceipt(t)
	runNormalize(t, pmt)
	assert.Equal(t, ConditionAmountReceived, pmt.Ext.Get(ExtKeyCondition))
}

// Default Condition extension on an advice is MPA (Amount paid).
func TestPaymentAdviceDefaultsConditionToMPA(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Type = bill.PaymentTypeAdvice
	runNormalize(t, pmt)
	assert.Equal(t, ConditionAmountPaid, pmt.Ext.Get(ExtKeyCondition))
}

// Partial-payment scenario: caller pins RAP (Amount remaining); the
// SetOneOf chain keeps the explicit override.
func TestPaymentAcceptsRAPOverride(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Ext = pmt.Ext.Set(ExtKeyCondition, ConditionAmountRemaining)
	runNormalize(t, pmt)
	require.NoError(t, rules.Validate(pmt))
	assert.Equal(t, ConditionAmountRemaining, pmt.Ext.Get(ExtKeyCondition))
}

// Status-only Condition codes are rejected on a Payment.
func TestPaymentRejectsStatusOnlyConditionCodes(t *testing.T) {
	for _, code := range []cbc.Code{
		ConditionBankDetailsUpdate, ConditionInvalidData,
		ConditionExpectedData, ConditionReplacementData,
		ConditionAmountApprovedHT, ConditionDiscount,
	} {
		pmt := testPaymentReceipt(t)
		runNormalize(t, pmt)
		// Replace the normalized MEN default with a Status-only code.
		pmt.Ext = pmt.Ext.Set(ExtKeyCondition, code)
		err := rules.Validate(pmt)
		assert.ErrorContains(t, err, "Payment-applicable", "code %s", code)
	}
}

// Status-only ProcessConditionCodes are rejected on a Payment.
func TestPaymentRejectsStatusProcessCodes(t *testing.T) {
	pmt := testPaymentReceipt(t)
	runNormalize(t, pmt)
	pmt.Ext = pmt.Ext.Set(ExtKeyStatus, "205") // Approved — Status-only
	err := rules.Validate(pmt)
	assert.ErrorContains(t, err, "Payment-applicable")
}

func TestPaymentAdviceSetsCDARStatusCode211(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Type = bill.PaymentTypeAdvice
	runNormalize(t, pmt)
	assert.Equal(t, cbc.Code("211"), pmt.Ext.Get(ExtKeyStatus))
}

func TestPaymentReceiptDefaultsSupplierRoleSeller(t *testing.T) {
	pmt := testPaymentReceipt(t)
	runNormalize(t, pmt)
	assert.Equal(t, RoleSeller, pmt.Supplier.Ext.Get(ExtKeyRole))
	assert.Equal(t, RoleBuyer, pmt.Customer.Ext.Get(ExtKeyRole))
}

func TestPaymentAdviceKeepsSellerBuyerRoles(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Type = bill.PaymentTypeAdvice
	runNormalize(t, pmt)
	// The role code names the party, not the issuer: the supplier is the
	// seller (SE) and the customer the buyer (BY), same as a receipt. An
	// advice being payer-issued is reflected at generation time by making
	// the buyer the CDAR issuer (cdar_payment.go), not by flipping roles.
	assert.Equal(t, RoleSeller, pmt.Supplier.Ext.Get(ExtKeyRole))
	assert.Equal(t, RoleBuyer, pmt.Customer.Ext.Get(ExtKeyRole))
}

func TestPaymentRejectsRequestType(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Type = bill.PaymentTypeRequest
	runNormalize(t, pmt)
	err := rules.Validate(pmt)
	assert.ErrorContains(t, err, "advice")
}

func TestPaymentRequiresSupplierSIREN(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Supplier.Identities = nil
	runNormalize(t, pmt)
	err := rules.Validate(pmt)
	assert.ErrorContains(t, err, "SIREN")
}

func TestPaymentRequiresCustomerSIREN(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Customer.Identities = nil
	runNormalize(t, pmt)
	err := rules.Validate(pmt)
	assert.ErrorContains(t, err, "SIREN")
}

func TestPaymentRequiresExactlyOneLine(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Lines = append(pmt.Lines, &bill.PaymentLine{
		Amount: num.MakeAmount(5000, 2),
		Document: &org.DocumentRef{
			Code:      "2026-00043",
			IssueDate: cal.NewDate(2026, 4, 15),
		},
	})
	runNormalize(t, pmt)
	err := rules.Validate(pmt)
	assert.ErrorContains(t, err, "exactly one")
}

func TestPaymentRequiresDocumentReference(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Lines[0].Document = nil
	runNormalize(t, pmt)
	err := rules.Validate(pmt)
	assert.ErrorContains(t, err, "payment line document is required")
}

func TestPaymentRequiresDocumentCode(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Lines[0].Document.Code = ""
	runNormalize(t, pmt)
	err := rules.Validate(pmt)
	assert.ErrorContains(t, err, "payment line document code")
}

func TestPaymentRequiresDocumentIssueDate(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Lines[0].Document.IssueDate = nil
	runNormalize(t, pmt)
	err := rules.Validate(pmt)
	assert.ErrorContains(t, err, "payment line document issue_date")
}

func TestPaymentRejectsSTCIdentityScheme(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Supplier.Identities = append(pmt.Supplier.Identities, &org.Identity{
		Code: "12345678",
		Ext: tax.ExtensionsOf(cbc.CodeMap{
			iso.ExtKeySchemeID: "0231",
		}),
	})
	runNormalize(t, pmt)
	err := rules.Validate(pmt)
	assert.ErrorContains(t, err, "Flow 6 allow-list")
}

func TestPaymentStatusCodeMismatchRejected(t *testing.T) {
	pmt := testPaymentReceipt(t)
	runNormalize(t, pmt)
	pmt.Ext = pmt.Ext.Set(ExtKeyStatus, "211") // wrong code for receipt
	err := rules.Validate(pmt)
	assert.ErrorContains(t, err, "ProcessConditionCode")
}

// Document the assumption that the payment-line currency is not
// inspected at the Flow 6 layer — it is taken from bill.Payment.Currency
// at the top level.
func TestPaymentTotalCurrencyEUR(t *testing.T) {
	pmt := testPaymentReceipt(t)
	assert.Equal(t, currency.Code("EUR"), pmt.Currency)
}

func TestPaymentMigratesLegacyDocTypeCode(t *testing.T) {
	// Legacy form: raw UNTDID code on Doc.Type, no untdid-document-type ext.
	pmt := testPaymentReceipt(t)
	pmt.Lines[0].Document.Ext = tax.Extensions{}
	pmt.Lines[0].Document.Type = "380"
	runNormalize(t, pmt)
	assert.Equal(t, cbc.Code("380"), pmt.Lines[0].Document.Ext.Get(untdid.ExtKeyDocumentType),
		"raw Type code should be promoted to the extension")
	assert.Equal(t, cbc.Key("380"), pmt.Lines[0].Document.Type,
		"Type must NOT be cleared after promotion")
	assert.NoError(t, rules.Validate(pmt))
}

func TestPaymentDoesNotMigrateNonCodeType(t *testing.T) {
	// A semantic key (not a UNTDID code) must not be promoted; validation
	// then fails, steering the caller to set the extension explicitly.
	pmt := testPaymentReceipt(t)
	pmt.Lines[0].Document.Ext = tax.Extensions{}
	pmt.Lines[0].Document.Type = "standard"
	runNormalize(t, pmt)
	assert.True(t, pmt.Lines[0].Document.Ext.Get(untdid.ExtKeyDocumentType).IsEmpty(),
		"semantic key must not be promoted")
	assert.ErrorContains(t, rules.Validate(pmt), "untdid-document-type")
}

// --- MDT-224: a payment receipt must carry a VAT breakdown ---------------

func TestPaymentReceiptRequiresVATBreakdown(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Lines[0].Tax = nil
	runNormalize(t, pmt)
	assert.ErrorContains(t, rules.Validate(pmt), "VAT")
}

// A VAT category with no rate entries does not satisfy MDT-224: the rule
// requires the breakdown to be present, even though an exempt rate is fine.
func TestPaymentReceiptRejectsVATCategoryWithoutRates(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Lines[0].Tax = &tax.Total{Categories: []*tax.CategoryTotal{{Code: tax.CategoryVAT}}}
	runNormalize(t, pmt)
	assert.ErrorContains(t, rules.Validate(pmt), "VAT")
}

// An advice payment (211) is not subject to the receipt-only VAT-rate rule.
func TestPaymentAdviceDoesNotRequireVATBreakdown(t *testing.T) {
	pmt := testPaymentReceipt(t)
	pmt.Type = bill.PaymentTypeAdvice
	pmt.Lines[0].Tax = nil
	runNormalize(t, pmt)
	assert.NoError(t, rules.Validate(pmt))
}

// paymentLineHasVATTax unit coverage, including the exempt-rate case
// ("the rate may be exempt") and the empty-Rates rejection.
func TestPaymentLineHasVATTax(t *testing.T) {
	assert.False(t, paymentLineHasVATTax("wrong-type"))
	assert.False(t, paymentLineHasVATTax([]*bill.PaymentLine{}))
	assert.False(t, paymentLineHasVATTax([]*bill.PaymentLine{{}}))
	// VAT category but no rate breakdown → false
	assert.False(t, paymentLineHasVATTax([]*bill.PaymentLine{{
		Tax: &tax.Total{Categories: []*tax.CategoryTotal{{Code: tax.CategoryVAT}}},
	}}))
	// non-VAT category with rates → false
	assert.False(t, paymentLineHasVATTax([]*bill.PaymentLine{{
		Tax: &tax.Total{Categories: []*tax.CategoryTotal{{Code: "GST", Rates: []*tax.RateTotal{{}}}}},
	}}))
	// VAT with an exempt rate (nil Percent) → true
	assert.True(t, paymentLineHasVATTax([]*bill.PaymentLine{{
		Tax: &tax.Total{Categories: []*tax.CategoryTotal{{Code: tax.CategoryVAT, Rates: []*tax.RateTotal{{}}}}},
	}}))
	// VAT with a standard rate → true
	assert.True(t, paymentLineHasVATTax([]*bill.PaymentLine{{
		Tax: &tax.Total{Categories: []*tax.CategoryTotal{{Code: tax.CategoryVAT, Rates: []*tax.RateTotal{{Percent: num.NewPercentage(20, 2)}}}}},
	}}))
}
