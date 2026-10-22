// Package cii adds the French CII invoice formats — French CTC,
// Factur-X and Chorus Pro — on top of the gobl.cii base import and export,
// and registers them with gobl.cii and the GOBL convert register. Import it
// for its side effects:
//
//	import _ "github.com/invopop/gobl.fr.ctc/cii"
package cii

import (
	goblcii "github.com/invopop/gobl.cii"
	"github.com/invopop/gobl.fr.ctc/addon/flow2"
	"github.com/invopop/gobl/addons/fr/choruspro"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/schema"
)

// Format keys.
const (
	KeyCTCCIUS            cbc.Key = "cii+fr-ctc-cius-v1"
	KeyCTCExtended        cbc.Key = "cii+fr-ctc-extended-v1"
	KeyCTCFacturXCIUS     cbc.Key = "cii+fr-ctc-facturx-cius-v1"
	KeyCTCFacturXExtended cbc.Key = "cii+fr-ctc-facturx-extended-v1"
	KeyChorusPro          cbc.Key = "cii+fr-choruspro-v1"
)

// ProcessIDRegulated is the Peppol France regulated billing process.
const ProcessIDRegulated = "urn:peppol:france:billing:regulated"

// GuidelineIDFacturXExtended is the BT-24 of a Factur-X EXTENDED document,
// checked against a closed codelist per profile (FX-SCH-A-000026). Factur-X
// EN 16931 carries the plain EN 16931 guideline.
const GuidelineIDFacturXExtended = goblcii.GuidelineIDEN16931 + "#conformant#urn:factur-x.eu:1p0:extended"

var invoiceSchema = schema.Lookup(bill.Invoice{})

var countriesFR = []l10n.Code{l10n.FR}

// The Factur-X profiles of Flux 2 (XP Z12-012), each validated with its
// Factur-X schematron and the French Flux 2 rules. BASIC and MINIMUM are not
// Flux 2 profiles.

// FormatCTCFacturXCIUS is used for French CTC Factur-X EN 16931 documents.
// Their CII only differs from a CIUS one by the PDF it travels in, so the
// format is export-only: its CII imports as CIUS.
var FormatCTCFacturXCIUS = goblcii.Format{
	Key:         KeyCTCFacturXCIUS,
	Name:        i18n.NewString("CII France CTC Factur-X CIUS"),
	Countries:   countriesFR,
	Export:      []schema.ID{invoiceSchema},
	GuidelineID: goblcii.GuidelineIDEN16931,
	BusinessID:  ProcessIDRegulated,
	Version:     goblcii.VersionD22B,
	Addons:      []cbc.Key{flow2.V1},
	VESID:       "fr.ctc:facturx-en16931:1.4.0-04",
	ExportFuncs: []goblcii.ExportFunc{exportBillingMode},
	Attachments: true,
}

// FormatCTCFacturXExtended is used for French CTC Factur-X EXTENDED documents.
// BT-24 carries the Factur-X EXTENDED guideline: Factur-X only accepts its own
// per-profile values, and the CTC rules don't check BT-24 at all.
var FormatCTCFacturXExtended = goblcii.Format{
	Key:               KeyCTCFacturXExtended,
	Name:              i18n.NewString("CII France CTC Factur-X EXTENDED"),
	Countries:         countriesFR,
	Import:            []schema.ID{invoiceSchema},
	Export:            []schema.ID{invoiceSchema},
	GuidelineID:       goblcii.GuidelineIDEN16931 + "#conformant#urn:peppol:france:billing:Factur-X:1.0",
	BusinessID:        ProcessIDRegulated,
	OutputGuidelineID: GuidelineIDFacturXExtended,
	Version:           goblcii.VersionD16B,
	Addons:            []cbc.Key{flow2.V1},
	VESID:             "fr.ctc:facturx-extended:1.4.0-04",
	ExportFuncs:       []goblcii.ExportFunc{exportBillingMode, exportExtended, exportSubLines},
	ImportFuncs:       []goblcii.ImportFunc{importBillingMode, importExtended},
	Attachments:       true,
}

// FormatCTCCIUS is used for French CTC CIUS documents.
var FormatCTCCIUS = goblcii.Format{
	Key:               KeyCTCCIUS,
	Name:              i18n.NewString("CII France CTC CIUS"),
	Countries:         countriesFR,
	Import:            []schema.ID{invoiceSchema},
	Export:            []schema.ID{invoiceSchema},
	GuidelineID:       goblcii.GuidelineIDEN16931 + "#compliant#urn:peppol:france:billing:cius:1.0",
	BusinessID:        ProcessIDRegulated,
	OutputGuidelineID: goblcii.GuidelineIDEN16931,
	Version:           goblcii.VersionD22B,
	Addons:            []cbc.Key{flow2.V1},
	VESID:             "fr.ctc:cii:1.4.0-04",
	ExportFuncs:       []goblcii.ExportFunc{exportBillingMode},
	ImportFuncs:       []goblcii.ImportFunc{importBillingMode},
	Attachments:       true,
}

// FormatCTCExtended is used for French CTC Extended documents,
// which emit the extended-ctc-fr guideline in BT-24.
var FormatCTCExtended = goblcii.Format{
	Key:               KeyCTCExtended,
	Name:              i18n.NewString("CII France CTC Extended"),
	Countries:         countriesFR,
	Import:            []schema.ID{invoiceSchema},
	Export:            []schema.ID{invoiceSchema},
	GuidelineID:       goblcii.GuidelineIDEN16931 + "#conformant#urn:peppol:france:billing:extended:1.0",
	BusinessID:        ProcessIDRegulated,
	OutputGuidelineID: goblcii.GuidelineIDEN16931 + "#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr",
	Version:           goblcii.VersionD22B,
	Addons:            []cbc.Key{flow2.V1},
	VESID:             "fr.ctc:extended-cii:1.4.0-04",
	// The CTC schematron never checks BT-24, so a mangled GuidelineID
	// arrives validated and the billing mode is all that is left to trust.
	// Extended because its extra mappings are additive: a CIUS document
	// carries none of them.
	Fallback: func(_, businessID string) bool {
		return isBillingMode(businessID)
	},
	ExportFuncs: []goblcii.ExportFunc{exportBillingMode, exportExtended, exportSubLines},
	ImportFuncs: []goblcii.ImportFunc{importBillingMode, importExtended},
	Attachments: true,
}

// FormatChorusPro is used for Chorus Pro V1 documents.
var FormatChorusPro = goblcii.Format{
	Key:         KeyChorusPro,
	Name:        i18n.NewString("CII Chorus Pro"),
	Countries:   countriesFR,
	Import:      []schema.ID{invoiceSchema},
	Export:      []schema.ID{invoiceSchema},
	GuidelineID: "A1", // Default framework type
	Version:     goblcii.VersionD16B,
	Addons:      []cbc.Key{choruspro.V1},
	ExportFuncs: []goblcii.ExportFunc{exportChorusPro},
	Attachments: true,
}

func init() {
	// France CIUS documents use the same GuidelineID as EN 16931, but can be
	// identified by their BusinessID carrying a billing mode code.
	FormatCTCFacturXExtended.Match = matchBillingMode(FormatCTCFacturXExtended)
	FormatCTCCIUS.Match = matchBillingMode(FormatCTCCIUS)
	FormatCTCExtended.Match = matchBillingMode(FormatCTCExtended)
	goblcii.RegisterFormats(
		FormatCTCFacturXCIUS, FormatCTCFacturXExtended, FormatCTCCIUS, FormatCTCExtended,
		FormatChorusPro,
	)
}

// matchBillingMode claims documents declaring the format's specification
// together with a French billing mode.
func matchBillingMode(f goblcii.Format) func(guidelineID, businessID string) bool {
	return func(guidelineID, businessID string) bool {
		if !isBillingMode(businessID) {
			return false
		}
		return guidelineID == f.OutputGuidelineID || guidelineID == f.GuidelineID
	}
}

// isBillingMode checks if the given businessID matches a known French
// billing mode code pattern (e.g., "S1", "B1", "M4"). These codes consist of
// a letter (B for goods, S for services, M for mixed) followed by a digit.
func isBillingMode(businessID string) bool {
	if len(businessID) != 2 {
		return false
	}
	switch businessID[0] {
	case 'B', 'S', 'M':
		return businessID[1] >= '0' && businessID[1] <= '9'
	}
	return false
}
