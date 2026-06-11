package flow2

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"

	"github.com/invopop/gobl.fr.ctc/addon/dgfip"
)

// b2g.go encodes the statically-verifiable subset of the BR-FR-CPRO
// rules (XP Z12-012 Annexe A V1.3, sheet "BR-France-CTC-CPRO"), which
// apply when an invoice is in the B2G perimeter — marked here with the
// `b2g` tag. The CPRO rules have no published schematron (the PPF /
// Chorus Pro applies them server-side, surfacing failures as
// REJ_CONT_B2G rejections), so these rules are the only pre-flight
// validation line.
//
// CPRO rules NOT encoded here, and why:
//
//	BR-FR-CPRO-01     contract type MARCHE/CONTRAT — EXT-FR-FE-01 has no
//	                  GOBL slot yet (candidate future extension).
//	BR-FR-CPRO-02     the line-level previous-invoice part (EXT-FR-FE-136)
//	                  has no GOBL slot; the BT-1/BT-25 parts ARE encoded.
//	BR-FR-CPRO-11/12/13  require the recipient directory (annuaire):
//	                  service-code / engagement-number obligations are
//	                  per-buyer settings — enforced in gov-fr, which holds
//	                  the directory.
//	BR-FR-CPRO-16/22  declared "règle non vérifiable" by the spec.
//	BR-FR-CPRO-18/21/22/28  concern the seller-agent block
//	                  (EXT-FR-FE-BG-03), which has no GOBL slot yet.
//	BR-FR-CPRO-23     "the S3 recipient must be the actual buyer" —
//	                  transport/directory-level business rule.
//	BR-FR-CPRO-25     single BT-20 occurrence — structurally guaranteed:
//	                  GOBL carries one pay.Terms (its notes length IS
//	                  encoded, CPRO-35).
//	BR-FR-CPRO-29     BT-120 exemption text is converter-derived; its
//	                  length is bounded by the scenario notes.
//	BR-FR-CPRO-40     line-level delivery location id (EXT-FR-FE-146)
//	                  has no GOBL slot.

// b2gSellerIdentitySchemes lists the ISO 6523 scheme IDs admissible for
// the seller's private identifier (BT-29) on a B2G invoice
// (BR-FR-CPRO-03).
var b2gSellerIdentitySchemes = []cbc.Code{
	"0009", // SIRET
	"0223", // EU VAT
	"0226", // Individual (particulier)
	"0227", // Non-EU
	"0228", // RIDET (New Caledonia)
	"0229", // TAHITI (French Polynesia)
}

func b2gInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		rules.When(invoiceHasTag(TagB2G),
			// -- Identification -------------------------------------------
			rules.Assert("cpro-02", "invoice and preceding invoice numbers must not exceed 20 characters (BR-FR-CPRO-02)",
				b2gTest(b2gInvoiceCodesMax20),
			),
			rules.Assert("cpro-03", "supplier requires an identity with an ISO scheme of 0009, 0223, 0226, 0227, 0228 or 0229 (BR-FR-CPRO-03)",
				b2gTest(b2gSupplierIdentityScheme),
			),
			rules.Assert("cpro-04", "supplier identity codes must match their ISO scheme format (BR-FR-CPRO-04..08)",
				b2gTest(b2gSupplierIdentityFormats),
			),
			rules.Assert("cpro-09", "supplier with a SIREN identity requires a SIRET identity (scheme 0009) (BR-FR-CPRO-09)",
				b2gTest(func(inv *bill.Invoice) bool { return b2gPartySIRENImpliesSIRET(inv.Supplier) }),
			),
			rules.Assert("cpro-10", "customer requires a SIRET identity (scheme 0009) (BR-FR-CPRO-10)",
				b2gTest(func(inv *bill.Invoice) bool {
					return inv.Customer == nil || partyIdentityWithScheme(inv.Customer, "0009") != nil
				}),
			),
			rules.Assert("cpro-17", "payee with a SIREN identity requires a SIRET identity (scheme 0009) (BR-FR-CPRO-17)",
				b2gTest(func(inv *bill.Invoice) bool {
					if inv.Payment == nil {
						return true
					}
					return b2gPartySIRENImpliesSIRET(inv.Payment.Payee)
				}),
			),

			// -- Cardinality ----------------------------------------------
			rules.Assert("cpro-19", "no more than 999999 invoice lines (BR-FR-CPRO-19)",
				b2gTest(func(inv *bill.Invoice) bool { return len(inv.Lines) <= 999999 }),
			),
			rules.Assert("cpro-20", "only one preceding invoice reference is allowed (BR-FR-CPRO-20)",
				b2gTest(func(inv *bill.Invoice) bool { return len(inv.Preceding) <= 1 }),
			),
			rules.Assert("cpro-26", "only one supplier contact person is allowed (BR-FR-CPRO-26)",
				b2gTest(func(inv *bill.Invoice) bool { return inv.Supplier == nil || len(inv.Supplier.People) <= 1 }),
			),
			rules.Assert("cpro-27", "only one customer contact person is allowed (BR-FR-CPRO-27)",
				b2gTest(func(inv *bill.Invoice) bool { return inv.Customer == nil || len(inv.Customer.People) <= 1 }),
			),

			// -- Billing framework ----------------------------------------
			rules.Assert("cpro-24", "billing mode (cadre de facturation) S5 is not allowed on B2G invoices — use S3 for direct-payment subcontracting (BR-FR-CPRO-24)",
				b2gTest(func(inv *bill.Invoice) bool {
					if inv.Tax == nil {
						return true
					}
					return inv.Tax.Ext.Get(dgfip.ExtKeyBillingMode) != dgfip.BillingModeS5
				}),
			),

			// -- Length limits --------------------------------------------
			rules.Assert("cpro-14", "contract references must not exceed 50 characters (BR-FR-CPRO-14)",
				b2gMaxLen(50, func(inv *bill.Invoice) []string {
					if inv.Ordering == nil {
						return nil
					}
					var out []string
					for _, c := range inv.Ordering.Contracts {
						if c != nil {
							out = append(out, c.Series.Join(c.Code).String())
						}
					}
					return out
				}),
			),
			rules.Assert("cpro-15", "purchase order / engagement reference must not exceed 50 characters (BR-FR-CPRO-15)",
				b2gMaxLen(50, func(inv *bill.Invoice) []string {
					if inv.Ordering == nil {
						return nil
					}
					return []string{inv.Ordering.Code.String()}
				}),
			),
			rules.Assert("cpro-30", "supporting document references must not exceed 50 characters (BR-FR-CPRO-30)",
				b2gMaxLen(50, func(inv *bill.Invoice) []string {
					var out []string
					for _, a := range inv.Attachments {
						if a != nil {
							out = append(out, a.Code.String())
						}
					}
					return out
				}),
			),
			rules.Assert("cpro-31", "item descriptions must not exceed 1024 characters (BR-FR-CPRO-31)",
				b2gMaxLen(1024, func(inv *bill.Invoice) []string {
					var out []string
					for _, l := range inv.Lines {
						if l != nil && l.Item != nil {
							out = append(out, l.Item.Description)
						}
					}
					return out
				}),
			),
			rules.Assert("cpro-32", "supplier street addresses must not exceed 400 characters (BR-FR-CPRO-32)",
				b2gMaxLen(400, func(inv *bill.Invoice) []string {
					return b2gPartyAddressField(inv.Supplier, func(a *org.Address) string { return a.Street })
				}),
			),
			rules.Assert("cpro-33", "supplier address localities must not exceed 400 characters (BR-FR-CPRO-33)",
				b2gMaxLen(400, func(inv *bill.Invoice) []string {
					return b2gPartyAddressField(inv.Supplier, func(a *org.Address) string { return a.Locality })
				}),
			),
			rules.Assert("cpro-34", "customer trading name (alias) must not exceed 99 characters (BR-FR-CPRO-34)",
				b2gMaxLen(99, func(inv *bill.Invoice) []string { return b2gPartyAlias(inv.Customer) }),
			),
			rules.Assert("cpro-35", "payment terms notes must not exceed 1024 characters (BR-FR-CPRO-35)",
				b2gMaxLen(1024, func(inv *bill.Invoice) []string {
					if inv.Payment == nil || inv.Payment.Terms == nil {
						return nil
					}
					return []string{inv.Payment.Terms.Notes}
				}),
			),
			rules.Assert("cpro-36", "supplier trading name (alias) must not exceed 99 characters (BR-FR-CPRO-36)",
				b2gMaxLen(99, func(inv *bill.Invoice) []string { return b2gPartyAlias(inv.Supplier) }),
			),
			rules.Assert("cpro-37", "payee name must not exceed 99 characters (BR-FR-CPRO-37)",
				b2gMaxLen(99, func(inv *bill.Invoice) []string {
					if inv.Payment == nil || inv.Payment.Payee == nil {
						return nil
					}
					return []string{inv.Payment.Payee.Name}
				}),
			),
			rules.Assert("cpro-38", "delivery location identifiers must not exceed 20 characters (BR-FR-CPRO-38)",
				b2gMaxLen(20, func(inv *bill.Invoice) []string {
					if inv.Delivery == nil {
						return nil
					}
					var out []string
					for _, id := range inv.Delivery.Identities {
						if id != nil {
							out = append(out, id.Code.String())
						}
					}
					if inv.Delivery.Receiver != nil {
						for _, id := range inv.Delivery.Receiver.Identities {
							if id != nil {
								out = append(out, id.Code.String())
							}
						}
					}
					return out
				}),
			),
			rules.Assert("cpro-39", "payment account identifiers must not exceed 27 characters (BR-FR-CPRO-39)",
				b2gMaxLen(27, func(inv *bill.Invoice) []string {
					if inv.Payment == nil || inv.Payment.Instructions == nil {
						return nil
					}
					var out []string
					for _, ct := range inv.Payment.Instructions.CreditTransfer {
						if ct == nil {
							continue
						}
						out = append(out, ct.IBAN, ct.Number)
					}
					return out
				}),
			),
			rules.Assert("cpro-41", "attachment names must not exceed 50 characters (BR-FR-CPRO-41)",
				b2gMaxLen(50, func(inv *bill.Invoice) []string {
					var out []string
					for _, a := range inv.Attachments {
						if a != nil {
							out = append(out, a.Name)
						}
					}
					return out
				}),
			),
			rules.Assert("cpro-42", "invoice notes must not exceed 1024 characters (BR-FR-CPRO-42)",
				b2gMaxLen(1024, func(inv *bill.Invoice) []string {
					var out []string
					for _, n := range inv.Notes {
						if n != nil {
							out = append(out, n.Text)
						}
					}
					return out
				}),
			),
			rules.Assert("cpro-43", "supplier name must not exceed 99 characters (BR-FR-CPRO-43)",
				b2gMaxLen(99, func(inv *bill.Invoice) []string { return b2gPartyName(inv.Supplier) }),
			),
			rules.Assert("cpro-44", "customer name must not exceed 99 characters (BR-FR-CPRO-44)",
				b2gMaxLen(99, func(inv *bill.Invoice) []string { return b2gPartyName(inv.Customer) }),
			),
		),
	)
}

// -- helpers ---------------------------------------------------------------

// b2gTest wraps an invoice predicate as a rules test, passing on any
// non-invoice value.
func b2gTest(fn func(*bill.Invoice) bool) rules.Test {
	return is.Func("b2g invoice check", func(v any) bool {
		inv, ok := v.(*bill.Invoice)
		if !ok || inv == nil {
			return true
		}
		return fn(inv)
	})
}

// b2gMaxLen builds a test asserting every string yielded by get is at
// most max characters long.
func b2gMaxLen(max int, get func(*bill.Invoice) []string) rules.Test {
	return is.Func(fmt.Sprintf("length <= %d", max), func(v any) bool {
		inv, ok := v.(*bill.Invoice)
		if !ok || inv == nil {
			return true
		}
		for _, s := range get(inv) {
			if utf8.RuneCountInString(s) > max {
				return false
			}
		}
		return true
	})
}

// b2gInvoiceCodesMax20 checks BR-FR-CPRO-02: the invoice number (BT-1)
// and any preceding invoice numbers (BT-25) are limited to 20
// characters.
func b2gInvoiceCodesMax20(inv *bill.Invoice) bool {
	if utf8.RuneCountInString(inv.Series.Join(inv.Code).String()) > 20 {
		return false
	}
	for _, p := range inv.Preceding {
		if p == nil {
			continue
		}
		if utf8.RuneCountInString(p.Series.Join(p.Code).String()) > 20 {
			return false
		}
	}
	return true
}

// partyIdentityWithScheme returns the party's first identity carrying
// the given iso-scheme-id extension, or nil.
func partyIdentityWithScheme(p *org.Party, scheme cbc.Code) *org.Identity {
	if p == nil {
		return nil
	}
	for _, id := range p.Identities {
		if id != nil && id.Ext.Get(iso.ExtKeySchemeID) == scheme {
			return id
		}
	}
	return nil
}

// b2gSupplierIdentityScheme checks BR-FR-CPRO-03: the supplier carries
// at least one identity with an admissible private-identifier scheme.
func b2gSupplierIdentityScheme(inv *bill.Invoice) bool {
	if inv.Supplier == nil {
		return true // supplier presence is the core rules' concern
	}
	for _, scheme := range b2gSellerIdentitySchemes {
		if partyIdentityWithScheme(inv.Supplier, scheme) != nil {
			return true
		}
	}
	return false
}

// b2gSupplierIdentityFormats checks BR-FR-CPRO-04..08: per-scheme
// format constraints on the supplier's private identifiers.
func b2gSupplierIdentityFormats(inv *bill.Invoice) bool {
	if inv.Supplier == nil {
		return true
	}
	for _, id := range inv.Supplier.Identities {
		if id == nil {
			continue
		}
		code := id.Code.String()
		n := utf8.RuneCountInString(code)
		switch id.Ext.Get(iso.ExtKeySchemeID) {
		case "0223", "0227": // EU VAT / non-EU id: under 18 chars
			if n >= 18 {
				return false
			}
		case "0228", "0229": // RIDET / TAHITI: 9-10 chars
			if n < 9 || n > 10 {
				return false
			}
		case "0226": // individual: 10 leading digits, 70 further chars
			if n < 10 || n > 80 {
				return false
			}
			for i, r := range code {
				if i >= 10 {
					break
				}
				if !unicode.IsDigit(r) {
					return false
				}
			}
		}
	}
	return true
}

// b2gPartySIRENImpliesSIRET checks the CPRO-09/17 pattern: a party
// identified by a SIREN (scheme 0002) must also carry its SIRET
// (scheme 0009).
func b2gPartySIRENImpliesSIRET(p *org.Party) bool {
	if p == nil || partyIdentityWithScheme(p, "0002") == nil {
		return true
	}
	return partyIdentityWithScheme(p, "0009") != nil
}

func b2gPartyAddressField(p *org.Party, get func(*org.Address) string) []string {
	if p == nil {
		return nil
	}
	var out []string
	for _, a := range p.Addresses {
		if a != nil {
			out = append(out, get(a))
		}
	}
	return out
}

func b2gPartyAlias(p *org.Party) []string {
	if p == nil {
		return nil
	}
	return []string{p.Alias}
}

func b2gPartyName(p *org.Party) []string {
	if p == nil {
		return nil
	}
	return []string{p.Name}
}
