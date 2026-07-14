package flow6

import (
	"slices"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// migrateDocRefType is a soft migration: when a referenced document carries a
// raw UNTDID document type code on the legacy Type key (the old convention)
// and not yet on the untdid-document-type extension, it promotes the code into
// the extension — the canonical representation validation checks. Only codes in
// referencedInvoiceTypes are promoted (a semantic key like "standard" is left
// alone), and Type is intentionally NOT cleared, so existing consumers reading
// it keep working while callers are steered towards the extension.
func migrateDocRefType(dr *org.DocumentRef) {
	if dr == nil || !dr.Ext.Get(untdid.ExtKeyDocumentType).IsEmpty() {
		return
	}
	if code := cbc.Code(dr.Type); slices.Contains(referencedInvoiceTypes, code) {
		dr.Ext = dr.Ext.Set(untdid.ExtKeyDocumentType, code)
	}
}

// referencedInvoiceTypes are the UNTDID 1001 document type codes a CDV may
// reference in MDT-91 (the type of the invoice the status/payment is about).
// Mirrors the invoice document types flow2 permits.
var referencedInvoiceTypes = []cbc.Code{
	"380", "389", "393", "501", "386", "500",
	"384", "471", "472", "473",
	"381", "261", "396", "502", "503", "262",
}

// docRefHasValidType reports whether a referenced document carries the
// untdid-document-type extension (MDT-91) set to a valid invoice type code.
// Presence is required — tax.ExtensionsHasCodes only constrains the value
// when the key is present, so it can't be used to make MDT-91 mandatory.
func docRefHasValidType(v any) bool {
	dr, ok := v.(*org.DocumentRef)
	if !ok || dr == nil {
		return false
	}
	dt := dr.Ext.Get(untdid.ExtKeyDocumentType)
	if dt == "" {
		return false
	}
	for _, c := range referencedInvoiceTypes {
		if dt == c {
			return true
		}
	}
	return false
}

// paymentLineHasVATTax reports whether the single payment line carries a tax
// total with a VAT category (MDT-224). The rate may be exempt — the rule
// requires the VAT breakdown to be present, not a particular percentage.
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
			return true
		}
	}
	return false
}
