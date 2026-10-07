// Package cii adds the French CII invoice formats — Peppol France CTC,
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
	"github.com/invopop/gobl/addons/fr/facturx"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/schema"
)

// Format keys.
const (
	KeyPeppolFranceCIUS     cbc.Key = "cii+peppol+fr-cius-v1"
	KeyPeppolFranceExtended cbc.Key = "cii+peppol+fr-extended-v1"
	KeyPeppolFranceFacturX  cbc.Key = "cii+peppol+fr-facturx-v1"
	KeyFacturX              cbc.Key = "cii+fr-facturx-v1"
	KeyFacturXBasic         cbc.Key = "cii+fr-facturx-v1+basic"
	KeyFacturXExtended      cbc.Key = "cii+fr-facturx-v1+extended"
	KeyChorusPro            cbc.Key = "cii+fr-choruspro-v1"
)

// ProfileIDPeppolFranceBilling is the Peppol France business process.
const ProfileIDPeppolFranceBilling = "urn:peppol:france:billing:regulated"

// Hybrid-PDF guideline IDs. BT-24 is checked against a closed codelist per
// profile (FX-SCH-A-000026), so these are not interchangeable. MINIMUM and
// BASIC WL are omitted: not EN 16931 conformant, so out of scope here.
const (
	GuidelineIDFacturXBasic    = goblcii.GuidelineIDEN16931 + "#compliant#urn:factur-x.eu:1p0:basic"
	GuidelineIDFacturXExtended = goblcii.GuidelineIDEN16931 + "#conformant#urn:factur-x.eu:1p0:extended"
)

var invoiceSchema = schema.Lookup(bill.Invoice{})

var countriesFR = []l10n.Code{l10n.FR}

// FormatFacturX is the Factur-X EN 16931 (COMFORT) profile.
var FormatFacturX = goblcii.Format{
	Key:         KeyFacturX,
	Name:        i18n.NewString("CII Factur-X EN 16931"),
	Countries:   countriesFR,
	Schemas:     []schema.ID{invoiceSchema},
	GuidelineID: goblcii.GuidelineIDEN16931,
	Version:     goblcii.VersionD22B,
	Addons:      []cbc.Key{facturx.V1},
	VESID:       "fr.factur-x:en16931:1.0.8",
}

// FormatFacturXBasic is the Factur-X BASIC profile, a CIUS of EN 16931.
var FormatFacturXBasic = goblcii.Format{
	Key:         KeyFacturXBasic,
	Name:        i18n.NewString("CII Factur-X BASIC"),
	Countries:   countriesFR,
	Schemas:     []schema.ID{invoiceSchema},
	GuidelineID: GuidelineIDFacturXBasic,
	Version:     goblcii.VersionD22B,
	Addons:      []cbc.Key{facturx.V1},
	VESID:       "fr.factur-x:basic:1.0.8",
}

// FormatFacturXExtended is the Factur-X EXTENDED profile.
var FormatFacturXExtended = goblcii.Format{
	Key:         KeyFacturXExtended,
	Name:        i18n.NewString("CII Factur-X EXTENDED"),
	Countries:   countriesFR,
	Schemas:     []schema.ID{invoiceSchema},
	GuidelineID: GuidelineIDFacturXExtended,
	Version:     goblcii.VersionD22B,
	Addons:      []cbc.Key{facturx.V1},
	VESID:       "fr.factur-x:extended:1.0.8",
	ExportFuncs: []goblcii.ExportFunc{exportSubLines},
}

// FormatPeppolFranceFacturX is used for Peppol France Factur-X documents.
// BT-24 carries the Factur-X EXTENDED guideline: Factur-X only accepts its own
// per-profile values, and the CTC rules don't check BT-24 at all.
var FormatPeppolFranceFacturX = goblcii.Format{
	Key:               KeyPeppolFranceFacturX,
	Name:              i18n.NewString("CII Peppol France Factur-X"),
	Countries:         countriesFR,
	Schemas:           []schema.ID{invoiceSchema},
	GuidelineID:       goblcii.GuidelineIDEN16931 + "#conformant#urn:peppol:france:billing:Factur-X:1.0",
	BusinessID:        ProfileIDPeppolFranceBilling,
	OutputGuidelineID: GuidelineIDFacturXExtended,
	Version:           goblcii.VersionD16B,
	Addons:            []cbc.Key{flow2.V1},
	VESID:             "fr.ctc:extended-cii:1.4.0-03",
	ExportFuncs:       []goblcii.ExportFunc{exportBillingMode, exportExtended, exportSubLines},
	ImportFuncs:       []goblcii.ImportFunc{importBillingMode, importExtended},
}

// FormatPeppolFranceCIUS is used for Peppol France CIUS documents.
var FormatPeppolFranceCIUS = goblcii.Format{
	Key:               KeyPeppolFranceCIUS,
	Name:              i18n.NewString("CII Peppol France CIUS"),
	Countries:         countriesFR,
	Schemas:           []schema.ID{invoiceSchema},
	GuidelineID:       goblcii.GuidelineIDEN16931 + "#compliant#urn:peppol:france:billing:cius:1.0",
	BusinessID:        ProfileIDPeppolFranceBilling,
	OutputGuidelineID: goblcii.GuidelineIDEN16931,
	Version:           goblcii.VersionD22B,
	Addons:            []cbc.Key{flow2.V1},
	VESID:             "fr.ctc:cii:1.4.0-03",
	ExportFuncs:       []goblcii.ExportFunc{exportBillingMode},
	ImportFuncs:       []goblcii.ImportFunc{importBillingMode},
}

// FormatPeppolFranceExtended is used for Peppol France Extended documents,
// which emit the extended-ctc-fr guideline in BT-24.
var FormatPeppolFranceExtended = goblcii.Format{
	Key:               KeyPeppolFranceExtended,
	Name:              i18n.NewString("CII Peppol France Extended"),
	Countries:         countriesFR,
	Schemas:           []schema.ID{invoiceSchema},
	GuidelineID:       goblcii.GuidelineIDEN16931 + "#conformant#urn:peppol:france:billing:extended:1.0",
	BusinessID:        ProfileIDPeppolFranceBilling,
	OutputGuidelineID: goblcii.GuidelineIDEN16931 + "#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr",
	Version:           goblcii.VersionD22B,
	Addons:            []cbc.Key{flow2.V1},
	VESID:             "fr.ctc:extended-cii:1.4.0-03",
	// The CTC schematron never checks BT-24, so a mangled GuidelineID
	// arrives validated and the billing mode is all that is left to trust.
	// Extended because its extra mappings are additive: a CIUS document
	// carries none of them.
	Fallback: func(_, businessID string) bool {
		return isBillingMode(businessID)
	},
	ExportFuncs: []goblcii.ExportFunc{exportBillingMode, exportExtended, exportSubLines},
	ImportFuncs: []goblcii.ImportFunc{importBillingMode, importExtended},
}

// FormatChorusPro is used for Chorus Pro V1 documents.
var FormatChorusPro = goblcii.Format{
	Key:         KeyChorusPro,
	Name:        i18n.NewString("CII Chorus Pro"),
	Countries:   countriesFR,
	Schemas:     []schema.ID{invoiceSchema},
	GuidelineID: "A1", // Default framework type
	Version:     goblcii.VersionD16B,
	Addons:      []cbc.Key{choruspro.V1},
	ExportFuncs: []goblcii.ExportFunc{exportChorusPro},
}

func init() {
	// France CIUS documents use the same GuidelineID as EN 16931, but can be
	// identified by their BusinessID carrying a billing mode code.
	FormatPeppolFranceFacturX.Match = matchBillingMode(FormatPeppolFranceFacturX)
	FormatPeppolFranceCIUS.Match = matchBillingMode(FormatPeppolFranceCIUS)
	FormatPeppolFranceExtended.Match = matchBillingMode(FormatPeppolFranceExtended)
	goblcii.RegisterFormats(
		FormatFacturX, FormatFacturXBasic, FormatFacturXExtended,
		FormatPeppolFranceFacturX, FormatPeppolFranceCIUS, FormatPeppolFranceExtended,
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
