package ubl

import (
	"github.com/invopop/gobl"
	"github.com/invopop/gobl.fr.ctc/addon/dgfip"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	cur "github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// UNCL 3035 role codes the French extended profile pins on the parties it
// adds: the facturant is the invoicer (EXT-FR-FE-113) and the party the
// invoice is addressed to the invoicee (EXT-FR-FE-90).
const (
	partyRoleInvoicer = "II"
	partyRoleInvoicee = "IV"
)

// paymentMeansNotDefined is the UNTDID 4461 code for an instrument that is
// not defined.
const paymentMeansNotDefined = "1"

// invoices provides the GOBL and UBL invoices of an export or import, if
// both are invoices.
func invoices(env *gobl.Envelope, doc goblubl.Document) (*bill.Invoice, *goblubl.Invoice, bool) {
	inv, ok := env.Extract().(*bill.Invoice)
	out, ok2 := doc.(*goblubl.Invoice)
	return inv, out, ok && ok2
}

// exportCIUS applies the French CIUS rules shared by the CIUS and Extended
// formats.
func exportCIUS(_ *goblubl.Format, env *gobl.Envelope, doc goblubl.Document) error {
	inv, out, ok := invoices(env, doc)
	if !ok {
		return nil
	}
	// The UBL ProfileID carries the billing mode.
	if profile := inv.Tax.GetExt(dgfip.ExtKeyBillingMode); profile != cbc.CodeEmpty {
		out.ProfileID = &goblubl.IDType{Value: profile.String()}
	}
	for _, p := range goblubl.InvoiceParties(inv, out) {
		taxRegistrationScheme(p.GOBL, p.UBL)
	}
	return nil
}

// taxRegistrationScheme sets the LOC scheme on the party's BT-32 tax
// registrations, which follow the BT-31 VAT identifier when there is one.
func taxRegistrationScheme(p *org.Party, out *goblubl.Party) {
	i := 0
	if p.TaxID != nil && p.TaxID.Code != "" {
		i = 1
	}
	for _, id := range p.Identities {
		if id.Scope != org.IdentityScopeTax {
			continue
		}
		out.PartyTaxScheme[i].TaxScheme.ID.Value = goblubl.TaxSchemeTaxRegistration
		i++
	}
}

// importCIUS restores the billing mode from the ProfileID and the document
// type from BT-3.
func importCIUS(_ *goblubl.Format, doc goblubl.Document, env *gobl.Envelope) error {
	inv, in, ok := invoices(env, doc)
	if !ok {
		return nil
	}
	profileID := ""
	if in.ProfileID != nil {
		profileID = in.ProfileID.Value
	}
	inv.Tax.Ext = inv.Tax.Ext.Set(dgfip.ExtKeyBillingMode, cbc.Code(profileID))
	importDocumentType(inv, in)
	return nil
}

// documentType pairs an UNTDID 1001 code with the GOBL invoice type and tags
// that stand for it.
type documentType struct {
	typ  cbc.Key
	tags []cbc.Key
}

// documentTypes inverts the Flow 2 scenarios: every UNTDID 1001 code the
// French profiles accept, against the type and tags whose scenario
// regenerates that same code on the way out. The base import only knows the
// codes EN 16931 itself defines, so the rest arrive as a plain "other".
var documentTypes = map[cbc.Code]documentType{
	// Simple invoices.
	"380": {bill.InvoiceTypeStandard, nil},
	"389": {bill.InvoiceTypeStandard, []cbc.Key{tax.TagSelfBilled}},
	"393": {bill.InvoiceTypeStandard, []cbc.Key{tax.TagFactoring}},
	"501": {bill.InvoiceTypeStandard, []cbc.Key{tax.TagSelfBilled, tax.TagFactoring}},
	// Deposit invoices.
	"386": {bill.InvoiceTypeStandard, []cbc.Key{tax.TagPrepayment}},
	"500": {bill.InvoiceTypeStandard, []cbc.Key{tax.TagSelfBilled, tax.TagPrepayment}},
	// Corrective invoices.
	"384": {bill.InvoiceTypeCorrective, nil},
	"471": {bill.InvoiceTypeCorrective, []cbc.Key{tax.TagSelfBilled}},
	"472": {bill.InvoiceTypeCorrective, []cbc.Key{tax.TagFactoring}},
	"473": {bill.InvoiceTypeCorrective, []cbc.Key{tax.TagSelfBilled, tax.TagFactoring}},
	// Credit notes.
	"381": {bill.InvoiceTypeCreditNote, nil},
	"261": {bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagSelfBilled}},
	"396": {bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagFactoring}},
	"502": {bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagSelfBilled, tax.TagFactoring}},
	"503": {bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagPrepayment}},
}

// importDocumentType restores BT-3, which the scenarios can only derive back
// from the invoice type and tags.
func importDocumentType(inv *bill.Invoice, in *goblubl.Invoice) {
	code := documentTypeCode(in)
	if code == cbc.CodeEmpty {
		return
	}
	if dt, ok := documentTypes[code]; ok {
		inv.Type = dt.typ
		inv.SetTags(dt.tags...)
		return
	}
	// A code with no type of its own, such as 262 for the consolidated credit
	// note, is kept as stated so the addon can report it for what it is.
	if inv.Type == bill.InvoiceTypeOther {
		inv.Tax.Ext = inv.Tax.Ext.Set(untdid.ExtKeyDocumentType, code)
	}
}

// documentTypeCode provides BT-3, whichever element the document carries it in.
func documentTypeCode(in *goblubl.Invoice) cbc.Code {
	tc := in.InvoiceTypeCode
	if tc == nil {
		tc = in.CreditNoteTypeCode
	}
	if tc == nil {
		return cbc.CodeEmpty
	}
	return cbc.Code(tc.Value)
}

// exportExtended adds the parties and amounts that only the French Extended
// profile defines.
func exportExtended(_ *goblubl.Format, env *gobl.Envelope, doc goblubl.Document) error {
	inv, out, ok := invoices(env, doc)
	if !ok {
		return nil
	}

	// BT-167/BT-167-1/BT-167-2/EXT-FR-FE-192: the VAT accounting currency
	// exchange rate, using the same rate gating as BT-6/BT-111.
	taxCurrency := inv.RegimeDef().GetCurrency()
	rate := cur.MatchExchangeRate(inv.ExchangeRates, inv.Currency, taxCurrency)
	addTaxExchangeRate(out, inv.Currency, taxCurrency, rate)

	// BT-8: VAT point date code on each line period, the same invoice-wide
	// value as the header.
	if inv.Tax != nil {
		if code, ok := goblubl.TaxPointCode(inv.Tax.Point); ok {
			lines := out.InvoiceLines
			if len(out.CreditNoteLines) > 0 {
				lines = out.CreditNoteLines
			}
			for i, l := range inv.Lines {
				if l.Period != nil {
					lines[i].InvoicePeriod.DescriptionCode = code
				}
			}
		}
	}

	if o := inv.Ordering; o != nil {
		// EXT-FR-FE-BG-05: the facturant, the service facturier raising the
		// invoice on the seller's behalf.
		if sp := out.AccountingSupplierParty.Party; o.Issuer != nil && sp != nil && sp.ServiceProviderParty != nil {
			sp.ServiceProviderParty.Party.IndustryClassificationCode = partyRoleInvoicer
		}
		// EXT-FR-FE-BG-04: the party the invoice is addressed to, which sits
		// under the buyer just as the facturant sits under the seller.
		if cp := out.AccountingCustomerParty.Party; o.Buyer != nil && cp != nil {
			addressee := goblubl.NewParty(o.Buyer)
			addressee.IndustryClassificationCode = partyRoleInvoicee
			cp.ServiceProviderParty = &goblubl.ServiceProviderParty{
				Party: addressee,
			}
		}
	}

	// EXT-FR-FE-BG-02: the payer, mapped to the PaymentMandate's PayerParty.
	if inv.Payment != nil && inv.Payment.Payer != nil {
		if len(out.PaymentMeans) == 0 {
			// PaymentMeans requires a PaymentMeansCode, so when the invoice
			// carries no payment instructions, fall back to an undefined
			// instrument.
			out.PaymentMeans = []goblubl.PaymentMeans{
				{PaymentMeansCode: goblubl.IDType{Value: paymentMeansNotDefined}},
			}
		}
		if out.PaymentMeans[0].PaymentMandate == nil {
			out.PaymentMeans[0].PaymentMandate = new(goblubl.PaymentMandate)
		}
		out.PaymentMeans[0].PaymentMandate.PayerParty = goblubl.NewParty(inv.Payment.Payer)
	}

	// EXT-FR-FE-BG-01/BG-03: the agent acting for a party, which UBL nests
	// inside the party it acts for.
	for _, p := range goblubl.InvoiceParties(inv, out) {
		if p.GOBL.Agent != nil && p.UBL.AgentParty == nil {
			p.UBL.AgentParty = goblubl.NewParty(p.GOBL.Agent)
		}
	}
	return nil
}

func addTaxExchangeRate(out *goblubl.Invoice, from, to cur.Code, rate *cur.ExchangeRate) {
	if from == to || rate == nil {
		return
	}
	source := string(from)
	target := string(to)
	calcRate := rate.Amount.String()
	out.TaxExchangeRate = &goblubl.ExchangeRate{
		SourceCurrencyCode: &source,
		TargetCurrencyCode: &target,
		CalculationRate:    &calcRate,
	}
	if rate.At != nil {
		date := rate.At.Date().String()
		out.TaxExchangeRate.Date = &date
	}
}

// importExtended restores the parties that only the French Extended profile
// defines.
func importExtended(_ *goblubl.Format, doc goblubl.Document, env *gobl.Envelope) error {
	inv, in, ok := invoices(env, doc)
	if !ok {
		return nil
	}

	// EXT-FR-FE-BG-04: the party the invoice is addressed to.
	if cp := in.AccountingCustomerParty.Party; cp != nil && cp.ServiceProviderParty != nil {
		if inv.Ordering == nil {
			inv.Ordering = new(bill.Ordering)
		}
		inv.Ordering.Buyer = goblubl.ParseParty(cp.ServiceProviderParty.Party)
	}

	// EXT-FR-FE-BG-02: the payer.
	if len(in.PaymentMeans) > 0 {
		if pm := in.PaymentMeans[0].PaymentMandate; pm != nil && pm.PayerParty != nil {
			if inv.Payment == nil {
				inv.Payment = new(bill.PaymentDetails)
			}
			inv.Payment.Payer = goblubl.ParseParty(pm.PayerParty)
		}
	}

	// EXT-FR-FE-BG-01/BG-03: the agent acting for a party.
	for _, p := range goblubl.InvoiceParties(inv, in) {
		if p.UBL.AgentParty != nil && p.GOBL.Agent == nil {
			p.GOBL.Agent = goblubl.ParseParty(p.UBL.AgentParty)
		}
	}
	return nil
}
