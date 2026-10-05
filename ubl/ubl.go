// Package ubl adds the French CTC UBL contexts on top of the gobl.ubl base
// conversion, and registers them with gobl.ubl and the GOBL convert
// register. Import it for its side effects:
//
//	import _ "github.com/invopop/gobl.fr.ctc/ubl"
package ubl

import (
	"github.com/invopop/gobl.fr.ctc/addon/flow2"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/schema"
)

// Context keys.
const (
	KeyCIUS     cbc.Key = "ubl+peppol+fr-cius-v1"
	KeyExtended cbc.Key = "ubl+peppol+fr-extended-v1"
)

// Peppol France process IDs.
const (
	ProcessIDRegulated    = "urn:peppol:france:billing:regulated"
	ProcessIDNonRegulated = "urn:peppol:france:billing:non-regulated"
)

// ContextCIUS defines the context for France UBL Invoice CIUS.
var ContextCIUS = goblubl.Context{
	Key:                   KeyCIUS,
	Name:                  i18n.NewString("UBL Peppol France CIUS"),
	Countries:             []l10n.Code{l10n.FR},
	Schemas:               []schema.ID{schema.Lookup(bill.Invoice{})},
	CustomizationID:       "urn:cen.eu:en16931:2017#compliant#urn:peppol:france:billing:cius:1.0",
	ProfileID:             ProcessIDRegulated,
	OutputCustomizationID: "urn:cen.eu:en16931:2017",
	Addons:                []cbc.Key{flow2.V1},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    "fr.ctc:ubl-invoice:1.4.0-03",
		CreditNote: "fr.ctc:ubl-creditnote:1.4.0-03",
	},
	Layers: []*goblubl.Layer{LayerCIUS},
}

// ContextExtended defines the context for France UBL Invoice Extended.
var ContextExtended = goblubl.Context{
	Key:                   KeyExtended,
	Name:                  i18n.NewString("UBL Peppol France Extended"),
	Countries:             []l10n.Code{l10n.FR},
	Schemas:               []schema.ID{schema.Lookup(bill.Invoice{})},
	CustomizationID:       "urn:cen.eu:en16931:2017#conformant#urn:peppol:france:billing:extended:1.0",
	ProfileID:             ProcessIDRegulated,
	OutputCustomizationID: "urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr",
	Addons:                []cbc.Key{flow2.V1},
	VESIDs: goblubl.VESIDMapping{
		Invoice:    "fr.ctc:extended-ubl-invoice:1.4.0-03",
		CreditNote: "fr.ctc:extended-ubl-creditnote:1.4.0-03",
	},
	// The CTC schematron never checks BT-24, so a mangled CustomizationID
	// arrives validated and the billing mode is all that is left to trust.
	// Extended because its extra mappings are additive: a CIUS document
	// carries none of them.
	Fallback: func(_, profileID string) bool {
		return isBillingMode(profileID)
	},
	Layers: []*goblubl.Layer{LayerCIUS, LayerExtended},
}

func init() {
	// France CIUS documents use the same CustomizationID as EN 16931, but
	// can be identified by their ProfileID carrying a billing mode code.
	ContextCIUS.Match = matchBillingMode(ContextCIUS)
	ContextExtended.Match = matchBillingMode(ContextExtended)
	goblubl.RegisterContexts(ContextCIUS, ContextExtended)
}

// matchBillingMode claims documents declaring the context's specification
// together with a French billing mode.
func matchBillingMode(ctx goblubl.Context) func(customizationID, profileID string) bool {
	return func(customizationID, profileID string) bool {
		if !isBillingMode(profileID) {
			return false
		}
		return customizationID == ctx.OutputCustomizationID || customizationID == ctx.CustomizationID
	}
}

// isBillingMode checks if the given profileID matches a known French
// billing mode code pattern (e.g., "S1", "B1", "M4"). These codes consist of
// a letter (B for goods, S for services, M for mixed) followed by a digit.
func isBillingMode(profileID string) bool {
	if len(profileID) != 2 {
		return false
	}
	switch profileID[0] {
	case 'B', 'S', 'M':
		return profileID[1] >= '0' && profileID[1] <= '9'
	}
	return false
}
