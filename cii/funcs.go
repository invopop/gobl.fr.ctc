package cii

import (
	"github.com/invopop/gobl"
	goblcii "github.com/invopop/gobl.cii"
	"github.com/invopop/gobl.fr.ctc/addon/dgfip"
	"github.com/invopop/gobl/addons/fr/choruspro"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/fr"
)

// UNCL 3035 role codes the French extended profile pins on the parties it
// adds: the facturant is the invoicer (EXT-FR-FE-113) and the party the
// invoice is addressed to the invoicee (EXT-FR-FE-90).
const (
	partyRoleInvoicer = "II"
	partyRoleInvoicee = "IV"
)

// invoices provides the GOBL and CII invoices of an export or import, if
// both are invoices.
func invoices(env *gobl.Envelope, doc goblcii.Document) (*bill.Invoice, *goblcii.Invoice, bool) {
	inv, ok := env.Extract().(*bill.Invoice)
	out, ok2 := doc.(*goblcii.Invoice)
	return inv, out, ok && ok2
}

// exportBillingMode writes the billing mode into BT-23.
func exportBillingMode(_ *goblcii.Format, env *gobl.Envelope, doc goblcii.Document) error {
	inv, out, ok := invoices(env, doc)
	if !ok {
		return nil
	}
	if profile := inv.Tax.GetExt(dgfip.ExtKeyBillingMode); profile != cbc.CodeEmpty {
		out.ExchangedContext.BusinessContext = &goblcii.ExchangedContextParameter{ID: profile.String()}
	}
	return nil
}

// importBillingMode reads the billing mode from BT-23.
func importBillingMode(_ *goblcii.Format, doc goblcii.Document, env *gobl.Envelope) error {
	inv, in, ok := invoices(env, doc)
	if !ok {
		return nil
	}
	if in.ExchangedContext != nil && in.ExchangedContext.BusinessContext != nil {
		inv.Tax.Ext = inv.Tax.Ext.Set(dgfip.ExtKeyBillingMode, cbc.Code(in.ExchangedContext.BusinessContext.ID))
	}
	return nil
}

// exportSubLines writes line breakdowns as sub-lines, which the EXTENDED
// profiles allow.
func exportSubLines(_ *goblcii.Format, env *gobl.Envelope, doc goblcii.Document) error {
	if inv, out, ok := invoices(env, doc); ok {
		goblcii.ExpandGroupLines(inv, out)
	}
	return nil
}

// exportExtended adds the parties only the French extended profile defines.
func exportExtended(_ *goblcii.Format, env *gobl.Envelope, doc goblcii.Document) error {
	inv, out, ok := invoices(env, doc)
	if !ok {
		return nil
	}

	// EXT-FR-FE-BG-03/BG-01: the agents acting for the seller and the buyer.
	// GOBL nests them inside the party they act for; CII keeps them as
	// siblings in the trade agreement.
	if agmt := out.Transaction.Agreement; agmt != nil {
		if inv.Supplier != nil && inv.Supplier.Agent != nil {
			agmt.SalesAgent = goblcii.NewParty(inv.Supplier.Agent)
		}
		if inv.Customer != nil && inv.Customer.Agent != nil {
			agmt.BuyerAgent = goblcii.NewParty(inv.Customer.Agent)
		}
	}

	// EXT-FR-FE-BG-04/BG-02: the party the invoice is addressed to and the
	// payer, with the role codes the profile fixes for the facturant
	// (EXT-FR-FE-113) and the addressee (EXT-FR-FE-90).
	if stlm := out.Transaction.Settlement; stlm != nil {
		if stlm.Invoicer != nil {
			stlm.Invoicer.RoleCode = partyRoleInvoicer
		}
		if inv.Ordering != nil && inv.Ordering.Buyer != nil {
			stlm.Invoicee = goblcii.NewParty(inv.Ordering.Buyer)
			stlm.Invoicee.RoleCode = partyRoleInvoicee
		}
		if inv.Payment != nil && inv.Payment.Payer != nil {
			stlm.Payer = goblcii.NewParty(inv.Payment.Payer)
		}
	}
	return nil
}

// importExtended reads the parties only the French extended profile defines.
func importExtended(_ *goblcii.Format, doc goblcii.Document, env *gobl.Envelope) error {
	inv, in, ok := invoices(env, doc)
	if !ok || in.Transaction == nil {
		return nil
	}

	if agmt := in.Transaction.Agreement; agmt != nil {
		if inv.Supplier != nil && agmt.SalesAgent != nil {
			inv.Supplier.Agent = goblcii.ParseParty(agmt.SalesAgent)
		}
		if inv.Customer != nil && agmt.BuyerAgent != nil {
			inv.Customer.Agent = goblcii.ParseParty(agmt.BuyerAgent)
		}
	}

	if stlm := in.Transaction.Settlement; stlm != nil {
		if stlm.Invoicee != nil {
			if inv.Ordering == nil {
				inv.Ordering = new(bill.Ordering)
			}
			inv.Ordering.Buyer = goblcii.ParseParty(stlm.Invoicee)
		}
		if stlm.Payer != nil {
			if inv.Payment == nil {
				inv.Payment = new(bill.PaymentDetails)
			}
			inv.Payment.Payer = goblcii.ParseParty(stlm.Payer)
		}
	}
	return nil
}

// exportChorusPro applies the Chorus Pro framework and party identifiers.
func exportChorusPro(_ *goblcii.Format, env *gobl.Envelope, doc goblcii.Document) error {
	inv, out, ok := invoices(env, doc)
	if !ok {
		return nil
	}

	// BT-24 carries the framework type.
	if inv.Tax != nil && inv.Tax.Ext.Has(choruspro.ExtKeyFramework) {
		out.ExchangedContext.GuidelineContext.ID = inv.Tax.Ext.Get(choruspro.ExtKeyFramework).String()
	}

	if agmt := out.Transaction.Agreement; agmt != nil {
		chorusProParty(inv.Supplier, agmt.Seller)
		chorusProParty(inv.Customer, agmt.Buyer)
	}
	if stlm := out.Transaction.Settlement; stlm != nil {
		if inv.Ordering != nil {
			chorusProParty(inv.Ordering.Issuer, stlm.Invoicer)
		}
		// The payee only keeps its identifiers, and the global ID is not used
		// in Chorus Pro.
		if stlm.Payee != nil {
			stlm.Payee.GlobalID = nil
		}
	}
	for i, l := range inv.Lines {
		if i < len(out.Transaction.Lines) && l.Seller != nil {
			if la := out.Transaction.Lines[i].Agreement; la != nil {
				chorusProParty(l.Seller, la.ItemSellerParty)
			}
		}
	}
	return nil
}

// chorusProParty suppresses the GlobalID, which is not used in Chorus Pro,
// and sets the LegalOrganization ID to the identifier required for the
// party's scheme type.
func chorusProParty(party *org.Party, p *goblcii.Party) {
	if party == nil || p == nil {
		return
	}
	p.GlobalID = nil
	if p.LegalOrganization == nil {
		p.LegalOrganization = &goblcii.LegalOrganization{}
	}
	p.LegalOrganization.ID = chorusProLegalOrgID(party)
	if p.LegalOrganization.ID.Value == "" {
		p.LegalOrganization.ID = nil
	}
}

// chorusProLegalOrgID returns the LegalOrganization ID for Chorus Pro based on
// the scheme extension value, which determines the type of identifier to use.
func chorusProLegalOrgID(party *org.Party) *goblcii.PartyID {
	scheme := party.Ext.Get(choruspro.ExtKeyScheme)
	pid := &goblcii.PartyID{
		SchemeID: scheme.String(),
	}
	switch scheme {
	case "1":
		// SIRET: identity with type SIRET
		for _, id := range party.Identities {
			if id.Type == fr.IdentityTypeSIRET {
				pid.Value = id.Code.String()
				return pid
			}
		}
	case "2":
		// Intra-community VAT number
		if party.TaxID != nil {
			pid.Value = party.TaxID.String()
		}
	case "3", "6":
		// Country code + first 16 characters of company name
		if party.TaxID != nil && party.TaxID.Country != "" {
			name := []rune(party.Name)
			if len(name) > 16 {
				name = name[:16]
			}
			pid.Value = party.TaxID.Country.String() + string(name)
		}
	case "4", "5":
		// RIDET / Tahiti: identity with scope legal
		for _, id := range party.Identities {
			if id.Scope == org.IdentityScopeLegal {
				pid.Value = id.Code.String()
				return pid
			}
		}
	}
	return pid
}
