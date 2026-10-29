package cii

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/gobl"
	goblcii "github.com/invopop/gobl.cii"
	"github.com/invopop/gobl.fr.ctc/addon/ctc"
	"github.com/invopop/gobl.fr.ctc/addon/flow2"
	"github.com/invopop/gobl/addons/fr/choruspro"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/fr"
	"github.com/invopop/gobl/tax"
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

// exportFacturXExtended writes the Factur-X EXTENDED guideline into BT-24.
func exportFacturXExtended(_ *goblcii.Format, _ *gobl.Envelope, doc goblcii.Document) error {
	if out, ok := doc.(*goblcii.Invoice); ok && out.ExchangedContext != nil && out.ExchangedContext.GuidelineContext != nil {
		out.ExchangedContext.GuidelineContext.ID = GuidelineIDFacturXExtended
	}
	return nil
}

// exportBillingMode writes the billing mode into BT-23.
func exportBillingMode(_ *goblcii.Format, env *gobl.Envelope, doc goblcii.Document) error {
	inv, out, ok := invoices(env, doc)
	if !ok {
		return nil
	}
	if profile := inv.Tax.GetExt(ctc.ExtKeyBillingMode); profile != cbc.CodeEmpty {
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
		inv.Tax.Ext = inv.Tax.Ext.Set(ctc.ExtKeyBillingMode, cbc.Code(in.ExchangedContext.BusinessContext.ID))
	}
	return nil
}

// importDocumentType restores the invoice type and tags that BT-3 stands for
// under the Flow 2 scenarios, which the base import only knows for the codes
// EN 16931 defines. The base already keeps the code itself.
func importDocumentType(_ *goblcii.Format, doc goblcii.Document, env *gobl.Envelope) error {
	inv, in, ok := invoices(env, doc)
	if !ok {
		return nil
	}
	typ, tags, ok := flow2.DocumentType(cbc.Code(in.ExchangedDocument.TypeCode))
	if !ok {
		return nil
	}
	inv.Type = typ
	for _, tag := range tags {
		if !inv.HasTags(tag) {
			inv.SetTags(append(inv.GetTags(), tag)...)
		}
	}
	return nil
}

// importVATBreakdownLines gives an invoice that arrived without lines, as
// BASIC WL ones do, one line per VAT breakdown (BG-23), as XP Z12-012 §4.7
// asks of a conversion to a format with lines. Each line carries the rate's
// base without the document charges and allowances already in it. The base
// import bypassed the calculation for the missing lines, so the bypass goes
// if the totals now calculate to the stated ones.
func importVATBreakdownLines(_ *goblcii.Format, _ goblcii.Document, env *gobl.Envelope) error {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok || len(inv.Lines) > 0 || inv.Totals == nil || inv.Totals.Taxes == nil {
		return nil
	}
	for _, ct := range inv.Totals.Taxes.Categories {
		for _, rt := range ct.Rates {
			l := vatBreakdownLine(inv, ct.Code, rt)
			l.Index = len(inv.Lines) + 1
			inv.Lines = append(inv.Lines, l)
		}
	}
	if !inv.HasTags(tax.TagBypass) {
		return nil
	}
	agree, err := totalsAgree(inv)
	if err != nil {
		return err
	}
	if agree {
		inv.RemoveTags(tax.TagBypass)
	}
	return nil
}

// vatBreakdownLine builds the line for one VAT breakdown, with its amounts
// already set, since a bypassed invoice doesn't calculate its lines.
func vatBreakdownLine(inv *bill.Invoice, cat cbc.Code, rt *tax.RateTotal) *bill.Line {
	price := rt.Base
	for _, c := range inv.Charges {
		if sameRate(c.Taxes, cat, rt) {
			price = price.Subtract(c.Amount)
		}
	}
	for _, d := range inv.Discounts {
		if sameRate(d.Taxes, cat, rt) {
			price = price.Add(d.Amount)
		}
	}
	code := rt.Ext.Get(untdid.ExtKeyTaxCategory)
	combo := &tax.Combo{Category: cat, Ext: rt.Ext}
	name := fmt.Sprintf("%s %s", cat, code)
	// As the base import reads it, a 0% rate is only kept as Z.
	if rt.Percent != nil && (!rt.Percent.IsZero() || code == "Z") {
		p := *rt.Percent
		combo.Percent = &p
		name += " " + p.String()
	}
	sum, total := price, price
	return &bill.Line{
		Quantity: num.MakeAmount(1, 0),
		Item:     &org.Item{Name: name, Price: &price},
		Sum:      &sum,
		Taxes:    tax.Set{combo},
		Total:    &total,
	}
}

// sameRate reports whether a charge or allowance falls in the VAT breakdown.
func sameRate(taxes tax.Set, cat cbc.Code, rt *tax.RateTotal) bool {
	combo := taxes.Get(cat)
	if combo == nil || combo.Ext.Get(untdid.ExtKeyTaxCategory) != rt.Ext.Get(untdid.ExtKeyTaxCategory) {
		return false
	}
	pct := func(p *num.Percentage) num.Percentage {
		if p == nil {
			return num.PercentageZero
		}
		return *p
	}
	return pct(combo.Percent).Equals(pct(rt.Percent))
}

// totalsAgree reports whether the invoice calculates, without bypass, to the
// totals it states.
func totalsAgree(inv *bill.Invoice) (bool, error) {
	data, err := json.Marshal(inv)
	if err != nil {
		return false, err
	}
	calc := new(bill.Invoice)
	if err := json.Unmarshal(data, calc); err != nil {
		return false, err
	}
	calc.RemoveTags(tax.TagBypass)
	if err := calc.Calculate(); err != nil {
		return false, nil //nolint:nilerr // a failed calculation just keeps the bypass
	}
	s, c := inv.Totals, calc.Totals
	return s.Sum.Equals(c.Sum) && s.Total.Equals(c.Total) && s.Tax.Equals(c.Tax) &&
		s.TotalWithTax.Equals(c.TotalWithTax) && s.Payable.Equals(c.Payable), nil
}

// exportSubLines writes line breakdowns as sub-lines, which the EXTENDED
// profiles allow.
func exportSubLines(_ *goblcii.Format, env *gobl.Envelope, doc goblcii.Document) error {
	if inv, out, ok := invoices(env, doc); ok {
		goblcii.ExpandGroupLines(inv, out)
	}
	return nil
}

// exportExtended adds the parties and delivery terms only the French extended
// profile defines.
func exportExtended(_ *goblcii.Format, env *gobl.Envelope, doc goblcii.Document) error {
	inv, out, ok := invoices(env, doc)
	if !ok {
		return nil
	}

	// EXT-FR-FE-BG-03/BG-01: the agents acting for the seller and the buyer.
	// GOBL nests them inside the party they act for; CII keeps them as
	// siblings in the trade agreement.
	if agmt := out.Transaction.Agreement; agmt != nil {
		// EXT-FR-FE-01: the contract type (UNCL 1153) of the one contract the
		// base writes.
		if agmt.Contract != nil {
			agmt.Contract.ReferenceTypeCode = inv.Ordering.Contracts[0].Ext.Get(untdid.ExtKeyReference).String()
		}
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

	// EXT-FR-FE-BG-14: the Incoterm code (EXT-FR-FE-185) from the delivery
	// extension, and the delivery location name (EXT-FR-FE-186) from the
	// label of the receiver's address.
	if del := inv.Delivery; del != nil && out.Transaction.Agreement != nil {
		if code := del.Ext.Get(flow2.ExtKeyDeliveryTerms); code != cbc.CodeEmpty {
			terms := &goblcii.DeliveryTerms{Code: code.String()}
			if del.Receiver != nil && len(del.Receiver.Addresses) > 0 && del.Receiver.Addresses[0].Label != "" {
				terms.Location = &goblcii.TradeLocation{Name: del.Receiver.Addresses[0].Label}
			}
			out.Transaction.Agreement.DeliveryTerms = terms
		}
	}
	return nil
}

// importExtended reads the parties and delivery terms only the French extended
// profile defines.
func importExtended(_ *goblcii.Format, doc goblcii.Document, env *gobl.Envelope) error {
	inv, in, ok := invoices(env, doc)
	if !ok || in.Transaction == nil {
		return nil
	}

	if agmt := in.Transaction.Agreement; agmt != nil {
		// EXT-FR-FE-01: the contract type.
		if agmt.Contract != nil && inv.Ordering != nil && len(inv.Ordering.Contracts) > 0 {
			c := inv.Ordering.Contracts[0]
			c.Ext = c.Ext.Set(untdid.ExtKeyReference, cbc.Code(agmt.Contract.ReferenceTypeCode))
		}
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

	// EXT-FR-FE-BG-14: the Incoterm code into the delivery extension, and the
	// delivery location name into the label of the receiver's address.
	if agmt := in.Transaction.Agreement; agmt != nil && agmt.DeliveryTerms != nil && agmt.DeliveryTerms.Code != "" {
		dt := agmt.DeliveryTerms
		if inv.Delivery == nil {
			inv.Delivery = new(bill.DeliveryDetails)
		}
		inv.Delivery.Ext = inv.Delivery.Ext.Set(flow2.ExtKeyDeliveryTerms, cbc.Code(dt.Code))
		if dt.Location != nil && dt.Location.Name != "" {
			if inv.Delivery.Receiver == nil {
				inv.Delivery.Receiver = new(org.Party)
			}
			if len(inv.Delivery.Receiver.Addresses) == 0 {
				inv.Delivery.Receiver.Addresses = []*org.Address{{}}
			}
			inv.Delivery.Receiver.Addresses[0].Label = dt.Location.Name
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
