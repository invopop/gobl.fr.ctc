package flow2

// extended.go documents the EXTENDED-CTC-FR profile, gated on the
// `extended` tag (see tags.go). The profile's wire-level rules ship as
// the fr.ctc:extended-cii schematron. A warning-strict validation sweep
// of the converter output against that rule set (fr.ctc:extended-cii:
// 1.3.1) found that the profile needs essentially no extended-only
// GOBL validation rules — the candidates originally scoped here either
// live elsewhere or are converter concerns:
//
//   - Line hierarchy (EXT-FR-FE-162 parent-line ID, EXT-FR-FE-163
//     GROUP/DETAIL subtype, GROUP net = Σ children — BR-FREXT-06/08):
//     handled entirely on the CONVERTER side. gobl.cii emits a
//     bill.Line.Breakdown as the GROUP/DETAIL hierarchy under the
//     ContextPeppolFranceFacturXV1 context; the GOBL model already
//     guarantees the sub-line sums, so no flow2 rule is required.
//   - VAT categories L (IGIC) and M (IPSI): NOT additions — France
//     forbids them (BR-FR-15). This is a base-profile constraint
//     applying to every FR invoice, so it lives in billInvoiceRules
//     (rule 42 / invoiceVATCategoriesInFRSet), not here.
//   - MIME-type and Incoterm restricted code lists (BR-FREXT-CL-24/27):
//     gobl.cii does not emit Incoterm/DeliveryTerms at all, and a MIME
//     type is only carried via the manual AddBinaryAttachment API, never
//     from GOBL invoice data on the normal Convert path. Until those are
//     emitted there is nothing for a flow2 rule to constrain.
//
// If a future sweep of converter output that exercises more extended
// features surfaces an EXTENDED-only constraint that GOBL would
// otherwise permit, add it here behind invoiceHasTag(TagExtended) and
// wire extendedInvoiceRules() into flow2.go's RegisterWithGuard.
