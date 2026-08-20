package flow2

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// Flow 2 profile tags. The French CTC reform layers two further
// profiles on top of the B2B clearance baseline; both are selected by
// tagging the invoice rather than by separate addons:
const (
	// TagExtended marks an invoice as using the EXTENDED-CTC-FR
	// profile — the Factur-X-EXTENDED-style superset of EN16931 with
	// line hierarchies and additional code lists. Validated on the
	// wire against the fr.ctc:extended-* rule sets.
	TagExtended cbc.Key = "extended"

	// TagB2G marks an invoice as being in the B2G perimeter (the
	// buyer is a public-sector actor; the PPF routes it to Chorus
	// Pro). Activates the BR-FR-CPRO rules, which have no published
	// schematron — these GOBL rules are the only validation line.
	TagB2G cbc.Key = "b2g"
)

// invoiceTags declares the Flow 2 profile tags accepted on a
// bill.Invoice.
func invoiceTags() *tax.TagSet {
	return &tax.TagSet{
		Schema: bill.ShortSchemaInvoice,
		List: []*cbc.Definition{
			{
				Key: TagExtended,
				Name: i18n.String{
					i18n.EN: "Extended",
					i18n.FR: "Étendu",
				},
				Desc: i18n.String{
					i18n.EN: "Apply the French CTC EXTENDED-CTC-FR profile rules.",
					i18n.FR: "Applique les règles du profil EXTENDED-CTC-FR.",
				},
			},
			{
				Key: TagB2G,
				Name: i18n.String{
					i18n.EN: "B2G",
					i18n.FR: "B2G",
				},
				Desc: i18n.String{
					i18n.EN: "Invoice addressed to a public-sector buyer (Chorus Pro); applies the BR-FR-CPRO rules.",
					i18n.FR: "Facture adressée à un acheteur public (Chorus Pro) ; applique les règles BR-FR-CPRO.",
				},
			},
		},
	}
}

// invoiceHasTag returns a rules test that passes when the invoice
// carries the given tag.
func invoiceHasTag(tag cbc.Key) rules.Test {
	return is.Func("invoice tagged "+tag.String(), func(v any) bool {
		inv, ok := v.(*bill.Invoice)
		return ok && inv != nil && inv.HasTags(tag)
	})
}
