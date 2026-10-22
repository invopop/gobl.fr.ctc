package cdar

import (
	"testing"

	cii "github.com/invopop/gobl.cii"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupplierWithReferencedSIREN(t *testing.T) {
	withIssuer := func(gids ...*cii.CDARGlobalID) *cii.CDAR {
		return &cii.CDAR{AcknowledgementDocuments: []*cii.CDARAcknowledgement{{
			ReferenceReferencedDocument: []*cii.CDARReferencedDocument{{
				IssuerTradeParty: &cii.CDARTradeParty{GlobalIDs: gids},
			}},
		}}}
	}
	sirenGID := &cii.CDARGlobalID{SchemeID: schemeIDSIREN, Value: "100000009"}

	t.Run("no SE party: the MDT-129 issuer becomes the supplier", func(t *testing.T) {
		s := supplierWithReferencedSIREN(nil, withIssuer(sirenGID))
		require.NotNil(t, s)
		assert.Equal(t, "100000009", partySIREN(s))
	})

	t.Run("SE party without SIREN: MDT-129 SIREN is added", func(t *testing.T) {
		se := &org.Party{Name: "VENDEUR"}
		s := supplierWithReferencedSIREN(se, withIssuer(sirenGID))
		assert.Same(t, se, s)
		assert.Equal(t, "VENDEUR", s.Name)
		assert.Equal(t, "100000009", partySIREN(s))
	})

	t.Run("SE party with SIREN: left untouched", func(t *testing.T) {
		se := &org.Party{Identities: []*org.Identity{sirenIdentity("300000007")}}
		s := supplierWithReferencedSIREN(se, withIssuer(sirenGID))
		require.Len(t, s.Identities, 1)
		assert.Equal(t, "300000007", partySIREN(s))
	})

	t.Run("MDT-129 without 0002: nothing is invented", func(t *testing.T) {
		se := &org.Party{Name: "VENDEUR"}
		s := supplierWithReferencedSIREN(se, withIssuer(&cii.CDARGlobalID{SchemeID: "0225", Value: "100000009_00012"}))
		assert.Empty(t, s.Identities)
	})

	t.Run("no MDT-129: supplier unchanged", func(t *testing.T) {
		assert.Nil(t, supplierWithReferencedSIREN(nil, &cii.CDAR{}))
	})
}

func TestCDARReasonText(t *testing.T) {
	note := func(content ...string) *cii.CDARNote { return &cii.CDARNote{Content: content} }

	t.Run("joins every content line", func(t *testing.T) {
		notes := []*cii.CDARNote{note("Facture déjà reçue", "le 12/08/2026"), note("sous la référence F202500001")}
		ds := &cii.CDARDocumentStatus{IncludedNotes: notes}
		assert.Equal(t, "Facture déjà reçue\nle 12/08/2026\nsous la référence F202500001", cdarReasonText(ds))
	})

	t.Run("nil and empty notes give no text", func(t *testing.T) {
		assert.Equal(t, "", cdarReasonText(&cii.CDARDocumentStatus{IncludedNotes: []*cii.CDARNote{nil, note()}}))
	})

	t.Run("the Reason label comes first", func(t *testing.T) {
		ds := &cii.CDARDocumentStatus{
			Reason:        []string{"Doublon"},
			IncludedNotes: []*cii.CDARNote{note("Facture déjà reçue")},
		}
		assert.Equal(t, "Doublon\nFacture déjà reçue", cdarReasonText(ds))
	})
}

// A code-less entry carrying field-level characteristics already hosts them
// on an "other" reason; the note it travels with explains those faults, so
// it belongs on that reason rather than on the line.
func TestStatusLineNoteWithCharacteristics(t *testing.T) {
	ref := &cii.CDARReferencedDocument{
		SpecifiedDocumentStatuses: []*cii.CDARDocumentStatus{{
			IncludedNotes: []*cii.CDARNote{{Content: []string{"Montant HT approuvé après remise"}}},
			SpecifiedDocumentCharacteristics: []*cii.CDARDocumentCharacteristic{{
				TypeCode: "MAP",
				Name:     "Montant HT",
			}},
		}},
	}
	line := goblStatusLineFromCDAR(ref)
	require.NotNil(t, line)
	require.Len(t, line.Reasons, 1)
	assert.Equal(t, bill.ReasonKeyOther, line.Reasons[0].Key)
	assert.Equal(t, "Montant HT approuvé après remise", line.Reasons[0].Description)
	require.Len(t, line.Reasons[0].Faults, 1)
	assert.Empty(t, line.Description)
}
