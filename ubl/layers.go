package ubl

import (
	"github.com/invopop/gobl.fr.ctc/addon/dgfip"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	cur "github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/org"
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

// LayerCIUS applies the French CIUS rules shared by the CIUS and Extended
// contexts.
var LayerCIUS = &goblubl.Layer{
	ConvertInvoice: func(_ *goblubl.Context, inv *bill.Invoice, out *goblubl.Invoice) error {
		// The UBL ProfileID carries the billing mode.
		if profile := inv.Tax.GetExt(dgfip.ExtKeyBillingMode); profile != cbc.CodeEmpty {
			out.ProfileID = &goblubl.IDType{Value: profile.String()}
		}
		return nil
	},
	ConvertParty: func(_ *goblubl.Context, p *org.Party, out *goblubl.Party) {
		// BT-32: tax registrations use the LOC scheme, after the BT-31 VAT
		// identifier when there is one.
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
	},
	ParseInvoice: func(_ *goblubl.Context, in *goblubl.Invoice, out *bill.Invoice) error {
		profileID := ""
		if in.ProfileID != nil {
			profileID = in.ProfileID.Value
		}
		out.Tax.Ext = out.Tax.Ext.Set(dgfip.ExtKeyBillingMode, cbc.Code(profileID))
		return nil
	},
}

// LayerExtended adds the parties and amounts that only the French Extended
// profile defines.
var LayerExtended = &goblubl.Layer{
	ConvertInvoice: convertExtendedInvoice,
	ConvertParty: func(ctx *goblubl.Context, p *org.Party, out *goblubl.Party) {
		// EXT-FR-FE-BG-01/BG-03: the agent acting for the buyer or the seller,
		// which UBL nests inside the party it acts for. GOBL forbids an agent
		// of an agent, so this recurses at most once.
		if p.Agent != nil {
			out.AgentParty = goblubl.NewParty(p.Agent, ctx)
		}
	},
	ParseParty: func(ctx *goblubl.Context, in *goblubl.Party, out *org.Party) {
		if in.AgentParty != nil {
			out.Agent = goblubl.ParseParty(in.AgentParty, ctx)
		}
	},
	ParseInvoice: parseExtendedInvoice,
}

func convertExtendedInvoice(ctx *goblubl.Context, inv *bill.Invoice, out *goblubl.Invoice) error {
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
			addressee := goblubl.NewParty(o.Buyer, ctx)
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
		out.PaymentMeans[0].PaymentMandate.PayerParty = goblubl.NewParty(inv.Payment.Payer, ctx)
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

func parseExtendedInvoice(ctx *goblubl.Context, in *goblubl.Invoice, out *bill.Invoice) error {
	// EXT-FR-FE-BG-04: the party the invoice is addressed to.
	if cp := in.AccountingCustomerParty.Party; cp != nil && cp.ServiceProviderParty != nil {
		if out.Ordering == nil {
			out.Ordering = new(bill.Ordering)
		}
		out.Ordering.Buyer = goblubl.ParseParty(cp.ServiceProviderParty.Party, ctx)
	}

	// EXT-FR-FE-BG-02: the payer.
	if len(in.PaymentMeans) > 0 {
		if pm := in.PaymentMeans[0].PaymentMandate; pm != nil && pm.PayerParty != nil {
			if out.Payment == nil {
				out.Payment = new(bill.PaymentDetails)
			}
			out.Payment.Payer = goblubl.ParseParty(pm.PayerParty, ctx)
		}
	}
	return nil
}
