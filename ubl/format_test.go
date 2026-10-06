package ubl_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/invopop/gobl.fr.ctc/addon"
	"github.com/invopop/gobl.fr.ctc/addon/flow2"
	frubl "github.com/invopop/gobl.fr.ctc/ubl"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatPeppolFranceCIUS(t *testing.T) {
	t.Run("basic conversion", func(t *testing.T) {
		env := loadTestEnvelope(t, "france-cius/invoice-fr-cius.json")

		// Convert with France CIUS format
		doc, err := ubl.Export(env, ubl.WithFormat(frubl.FormatCIUS))
		require.NoError(t, err)

		ublInv, ok := doc.(*ubl.Invoice)
		require.True(t, ok)

		// Verify OutputCustomizationID is used in the output
		assert.Equal(t, "urn:cen.eu:en16931:2017", ublInv.CustomizationID)
		// Verify ProfileID comes from the fr-ctc-billing-mode extension
		assert.Equal(t, "S1", ublInv.ProfileID.Value)
	})

	t.Run("external identification uses full CustomizationID", func(t *testing.T) {
		// Verify the format itself has the full identification
		assert.Equal(t, "urn:cen.eu:en16931:2017#compliant#urn:peppol:france:billing:cius:1.0", frubl.FormatCIUS.CustomizationID)
		assert.Equal(t, "urn:peppol:france:billing:regulated", frubl.FormatCIUS.ProfileID)
		assert.Equal(t, "urn:cen.eu:en16931:2017", frubl.FormatCIUS.OutputCustomizationID)
	})
}

func TestFormatPeppolFranceExtended(t *testing.T) {
	t.Run("basic conversion", func(t *testing.T) {
		env := loadTestEnvelope(t, "france-extended/invoice-standard.json")

		// Convert with France Extended format
		doc, err := ubl.Export(env, ubl.WithFormat(frubl.FormatExtended))
		require.NoError(t, err)

		ublInv, ok := doc.(*ubl.Invoice)
		require.True(t, ok)

		// Verify OutputCustomizationID is used
		assert.Equal(t, "urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr", ublInv.CustomizationID)
		// Verify ProfileID comes from the fr-ctc-billing-mode extension
		assert.Equal(t, "S1", ublInv.ProfileID.Value)
	})

	t.Run("external identification uses full CustomizationID", func(t *testing.T) {
		// Verify the format itself has the full identification
		assert.Equal(t, "urn:cen.eu:en16931:2017#conformant#urn:peppol:france:billing:extended:1.0", frubl.FormatExtended.CustomizationID)
		assert.Equal(t, "urn:peppol:france:billing:regulated", frubl.FormatExtended.ProfileID)
		assert.Equal(t, "urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr", frubl.FormatExtended.OutputCustomizationID)
	})
}

func TestFrenchBillingModeResolution(t *testing.T) {
	// Senders differ on which identifier they put in the document: the one the
	// profile emits, or the spec-level one that names it on the network.
	const (
		specCIUS     = "urn:cen.eu:en16931:2017#compliant#urn:peppol:france:billing:cius:1.0"
		specExtended = "urn:cen.eu:en16931:2017#conformant#urn:peppol:france:billing:extended:1.0"
		docCIUS      = "urn:cen.eu:en16931:2017"
		docExtended  = "urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr"
	)

	for _, tt := range []struct {
		name            string
		customizationID string
		want            ubl.Format
	}{
		{"in-document CIUS", docCIUS, frubl.FormatCIUS},
		{"in-document Extended", docExtended, frubl.FormatExtended},
		{"spec-level CIUS", specCIUS, frubl.FormatCIUS},
		{"spec-level Extended", specExtended, frubl.FormatExtended},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, mode := range []string{"B1", "S1", "M4"} {
				ctx := ubl.FindFormat(tt.customizationID, mode)
				require.NotNil(t, ctx, "mode %s", mode)
				assert.Equal(t, tt.want.CustomizationID, ctx.CustomizationID, "mode %s", mode)
				assert.Equal(t, tt.want.VESIDs.Invoice, ctx.VESIDs.Invoice, "mode %s", mode)
			}
		})
	}

	t.Run("spec-level CustomizationID still carries the addon through Convert", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(getParsePath(), "france-cius", "b2b-reg.xml"))
		require.NoError(t, err)
		old := []byte("<cbc:CustomizationID>" + docCIUS + "</cbc:CustomizationID>")
		require.Contains(t, string(data), string(old))
		data = bytes.Replace(data, old, []byte("<cbc:CustomizationID>"+specCIUS+"</cbc:CustomizationID>"), 1)

		doc, err := ubl.Decode(data)
		require.NoError(t, err)
		env, err := ubl.Import(doc.(*ubl.Invoice))
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		assert.Contains(t, inv.GetAddons(), flow2.V1)
	})

	t.Run("unmodelled CustomizationID still parses best-effort", func(t *testing.T) {
		// Without a billing mode there is nothing to fall back on.
		ctx := ubl.FindFormat("urn:peppol:pint:billing-1@sg-1", "")
		assert.Nil(t, ctx)
	})
}

// BT-24 is never validated by the CTC schematron, so a mangled CustomizationID
// leaves the BT-23 billing mode as the only French signal.
func TestFrenchBillingModeFallback(t *testing.T) {
	for _, tt := range []struct {
		name            string
		customizationID string
		want            *ubl.Format
	}{
		{
			// Seen in the wild: "urn.eu:" for "urn:cen.eu:", no suffix.
			"mangled extended URN",
			"urn.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0",
			&frubl.FormatExtended,
		},
		{
			"unsuffixed cpro URN",
			"urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0",
			&frubl.FormatExtended,
		},
		{
			"mangled EN16931 URN",
			"urn.eu:en16931:2017",
			&frubl.FormatExtended,
		},
		{
			// Every other profile's ProfileID is a long URN.
			"unrelated customization still follows the billing mode",
			"urn:peppol:pint:billing-1@sg-1",
			&frubl.FormatExtended,
		},
		{
			// BT-24 absent: the billing mode is all there is.
			"absent customization",
			"",
			&frubl.FormatExtended,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ubl.FindFormat(tt.customizationID, "B2")
			if tt.want == nil {
				assert.Nil(t, ctx)
				return
			}
			require.NotNil(t, ctx)
			assert.Equal(t, tt.want.CustomizationID, ctx.CustomizationID)
			assert.Equal(t, tt.want.Addons, ctx.Addons)
		})
	}

	t.Run("no billing mode means no fallback", func(t *testing.T) {
		ctx := ubl.FindFormat("urn.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0", "")
		assert.Nil(t, ctx)
	})

	t.Run("mangled URN carries the addon through Convert", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(getParsePath(), "france-cius", "b2b-reg.xml"))
		require.NoError(t, err)
		old := []byte("<cbc:CustomizationID>urn:cen.eu:en16931:2017</cbc:CustomizationID>")
		require.Contains(t, string(data), string(old))
		data = bytes.Replace(data, old, []byte("<cbc:CustomizationID>urn.eu:en16931:2017</cbc:CustomizationID>"), 1)

		doc, err := ubl.Decode(data)
		require.NoError(t, err)
		env, err := ubl.Import(doc.(*ubl.Invoice))
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		assert.Contains(t, inv.GetAddons(), flow2.V1)
	})
}

func TestGetVESID(t *testing.T) {
	t.Run("France CIUS VESID", func(t *testing.T) {
		env := loadTestEnvelope(t, "invoice-minimal.json")

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		// Get VESID for France CIUS format
		vesid := frubl.FormatCIUS.GetVESID(inv)
		assert.Equal(t, "fr.ctc:ubl-invoice:1.4.0-03", vesid)
	})

	t.Run("France Extended VESID", func(t *testing.T) {
		env := loadTestEnvelope(t, "invoice-minimal.json")

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		// Get VESID for France Extended format
		vesid := frubl.FormatExtended.GetVESID(inv)
		assert.Equal(t, "fr.ctc:extended-ubl-invoice:1.4.0-03", vesid)
	})
}

func TestFindFormat(t *testing.T) {
	t.Run("find France CIUS by full CustomizationID", func(t *testing.T) {
		ctx := ubl.FindFormat("urn:cen.eu:en16931:2017#compliant#urn:peppol:france:billing:cius:1.0", "urn:peppol:france:billing:regulated")
		require.NotNil(t, ctx)
		assert.Equal(t, frubl.FormatCIUS.CustomizationID, ctx.CustomizationID)
		assert.Equal(t, frubl.FormatCIUS.ProfileID, ctx.ProfileID)
	})

	t.Run("find France CIUS by billing mode ProfileID", func(t *testing.T) {
		// France CIUS documents use EN16931 CustomizationID but have a billing mode as ProfileID
		ctx := ubl.FindFormat("urn:cen.eu:en16931:2017", "B1")
		require.NotNil(t, ctx)
		assert.Equal(t, frubl.FormatCIUS.CustomizationID, ctx.CustomizationID)

		ctx = ubl.FindFormat("urn:cen.eu:en16931:2017", "S1")
		require.NotNil(t, ctx)
		assert.Equal(t, frubl.FormatCIUS.CustomizationID, ctx.CustomizationID)

		ctx = ubl.FindFormat("urn:cen.eu:en16931:2017", "M4")
		require.NotNil(t, ctx)
		assert.Equal(t, frubl.FormatCIUS.CustomizationID, ctx.CustomizationID)
	})

	t.Run("find France Extended by OutputCustomizationID", func(t *testing.T) {
		// Simulates parsing a French Extended document
		ctx := ubl.FindFormat("urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr", "")
		require.NotNil(t, ctx)
		assert.Equal(t, frubl.FormatExtended.CustomizationID, ctx.CustomizationID)
		assert.Equal(t, "urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr", ctx.OutputCustomizationID)
	})
}
