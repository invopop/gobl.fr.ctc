package flow2

// extended.go will carry the EXTENDED-CTC-FR profile deltas, gated on
// the `extended` tag (see tags.go). The profile's wire-level rules ship
// as the fr.ctc:extended-* schematrons; the GOBL-side rules to add here
// are being derived from a warning-strict validation sweep of the
// converters against those rule sets. Known candidates:
//
//   - Line hierarchy: parent-line ID (EXT-FR-FE-162) and line subtype
//     DETAIL/GROUP (EXT-FR-FE-163), with GROUP net amounts equal to the
//     sum of their children — maps onto bill.Line.Breakdown.
//   - MIME-type and Incoterm restricted code lists
//     (BR-FREXT-CL-24/27).
//   - Additional VAT categories L (IGIC) and M (IPSI) with the
//     per-category taxable-amount tolerance checks
//     (BR-FREXT-*-08 ini/rev).
//
// EN16931 core rules that the Extended profile overrides or drops will
// be suppressed via the rule set's Ignore list (fully-qualified foreign
// fault codes), with flow2-namespaced replacements asserted alongside
// where the profile substitutes rather than removes a constraint.
