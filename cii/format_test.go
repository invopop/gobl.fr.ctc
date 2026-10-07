package cii_test

import (
	"os"
	"path/filepath"
	"testing"

	goblcii "github.com/invopop/gobl.cii"
	"github.com/invopop/gobl.fr.ctc/addon/flow2"
	frcii "github.com/invopop/gobl.fr.ctc/cii"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportFormats(t *testing.T) {
	t.Run("with Factur-X context", func(t *testing.T) {
		env := loadEnvelope(t, "facturx/invoice-complete.json")
		out, err := goblcii.ExportInvoice(env, goblcii.WithFormat(frcii.FormatFacturX))
		require.NoError(t, err)

		assert.Equal(t, "urn:cen.eu:en16931:2017", out.ExchangedContext.GuidelineContext.ID)
		assert.Nil(t, out.ExchangedContext.BusinessContext)
	})

	t.Run("with France Extended context", func(t *testing.T) {
		env := loadEnvelope(t, "peppol-france-extended/invoice-standard.json")
		out, err := goblcii.ExportInvoice(env, goblcii.WithFormat(frcii.FormatPeppolFranceExtended))
		require.NoError(t, err)

		// BT-24 carries the extended-ctc-fr guideline, BT-23 the billing mode.
		assert.Equal(t, "urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr", out.ExchangedContext.GuidelineContext.ID)
		assert.Equal(t, "S1", out.ExchangedContext.BusinessContext.ID)
	})
}

func TestFindFormat(t *testing.T) {
	t.Run("find France CIUS by full GuidelineID", func(t *testing.T) {
		ctx := goblcii.FindFormat("urn:cen.eu:en16931:2017#compliant#urn:peppol:france:billing:cius:1.0", "urn:peppol:france:billing:regulated")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatPeppolFranceCIUS.GuidelineID, ctx.GuidelineID)
	})

	t.Run("find France CIUS by billing mode BusinessID", func(t *testing.T) {
		// France CIUS documents use EN16931 GuidelineID but have a billing mode as BusinessID
		ctx := goblcii.FindFormat("urn:cen.eu:en16931:2017", "B1")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatPeppolFranceCIUS.GuidelineID, ctx.GuidelineID)

		ctx = goblcii.FindFormat("urn:cen.eu:en16931:2017", "S1")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatPeppolFranceCIUS.GuidelineID, ctx.GuidelineID)

		ctx = goblcii.FindFormat("urn:cen.eu:en16931:2017", "M4")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatPeppolFranceCIUS.GuidelineID, ctx.GuidelineID)
	})

	t.Run("find the French contexts by their spec-level GuidelineID", func(t *testing.T) {
		// Some senders put the identifier that names the profile on the network
		// into the document instead of the guideline the profile emits.
		for _, mode := range []string{"B1", "S1", "M4"} {
			ctx := goblcii.FindFormat(frcii.FormatPeppolFranceCIUS.GuidelineID, mode)
			require.NotNil(t, ctx, "mode %s", mode)
			assert.Equal(t, frcii.FormatPeppolFranceCIUS.GuidelineID, ctx.GuidelineID)
			assert.Equal(t, frcii.FormatPeppolFranceCIUS.VESID, ctx.VESID)

			ctx = goblcii.FindFormat(frcii.FormatPeppolFranceFacturX.GuidelineID, mode)
			require.NotNil(t, ctx, "mode %s", mode)
			assert.Equal(t, frcii.FormatPeppolFranceFacturX.GuidelineID, ctx.GuidelineID)
			assert.Equal(t, frcii.FormatPeppolFranceFacturX.VESID, ctx.VESID)

			ctx = goblcii.FindFormat(frcii.FormatPeppolFranceExtended.GuidelineID, mode)
			require.NotNil(t, ctx, "mode %s", mode)
			assert.Equal(t, frcii.FormatPeppolFranceExtended.GuidelineID, ctx.GuidelineID)
			assert.Equal(t, frcii.FormatPeppolFranceExtended.VESID, ctx.VESID)
		}
	})

	t.Run("find France Extended by full GuidelineID", func(t *testing.T) {
		ctx := goblcii.FindFormat("urn:cen.eu:en16931:2017#conformant#urn:peppol:france:billing:extended:1.0", "urn:peppol:france:billing:regulated")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatPeppolFranceExtended.GuidelineID, ctx.GuidelineID)
	})

	t.Run("find France Extended by the guideline it emits", func(t *testing.T) {
		// The extended-ctc-fr guideline is unique to this context, so it
		// resolves both with and without a billing mode BusinessID.
		ctx := goblcii.FindFormat("urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr", "B1")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatPeppolFranceExtended.GuidelineID, ctx.GuidelineID)

		ctx = goblcii.FindFormat("urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr", "")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatPeppolFranceExtended.GuidelineID, ctx.GuidelineID)
	})

	t.Run("find France Factur-X by the guideline it emits", func(t *testing.T) {
		// A billing mode picks the French context; without one the same
		// guideline is the plain Factur-X EXTENDED profile.
		ctx := goblcii.FindFormat(frcii.FormatFacturXExtended.GuidelineID, "B1")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatPeppolFranceFacturX.GuidelineID, ctx.GuidelineID)

		ctx = goblcii.FindFormat(frcii.FormatFacturXExtended.GuidelineID, "")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatFacturXExtended.GuidelineID, ctx.GuidelineID)
	})
}

// BT-24 is never validated by the CTC schematron, so a mangled GuidelineID
// leaves the BT-23 billing mode as the only French signal.
func TestFrenchBillingModeFallback(t *testing.T) {
	for _, tt := range []struct {
		name        string
		guidelineID string
		want        *goblcii.Format
	}{
		{
			// Seen in the wild: "urn.eu:" for "urn:cen.eu:", no suffix.
			"mangled extended URN",
			"urn.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0",
			&frcii.FormatPeppolFranceExtended,
		},
		{
			"unsuffixed cpro URN",
			"urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0",
			&frcii.FormatPeppolFranceExtended,
		},
		{
			"mangled EN16931 URN",
			"urn.eu:en16931:2017",
			&frcii.FormatPeppolFranceExtended,
		},
		{
			// BT-24 absent: the billing mode is all there is.
			"absent guideline",
			"",
			&frcii.FormatPeppolFranceExtended,
		},
		{
			// Every other profile's BusinessProcessParameter is a long URN.
			"unrelated guideline still follows the billing mode",
			"urn:peppol:pint:billing-1@sg-1",
			&frcii.FormatPeppolFranceExtended,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := goblcii.FindFormat(tt.guidelineID, "B2")
			if tt.want == nil {
				assert.Nil(t, ctx)
				return
			}
			require.NotNil(t, ctx)
			assert.Equal(t, tt.want.GuidelineID, ctx.GuidelineID)
			assert.Equal(t, tt.want.Addons, ctx.Addons)
		})
	}

	t.Run("no billing mode means no fallback", func(t *testing.T) {
		ctx := goblcii.FindFormat("urn.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0", "")
		assert.Nil(t, ctx)
	})
}

// TestHybridProfileGuidelines pins each hybrid context to the BT-24 value its
// profile's codelist admits; a wrong value invalidates every document.
func TestHybridProfileGuidelines(t *testing.T) {
	for _, tc := range []struct {
		name      string
		context   goblcii.Format
		guideline string
		vesID     string
	}{
		{"Factur-X BASIC", frcii.FormatFacturXBasic,
			"urn:cen.eu:en16931:2017#compliant#urn:factur-x.eu:1p0:basic", "fr.factur-x:basic:1.0.8"},
		{"Factur-X EN 16931", frcii.FormatFacturX,
			"urn:cen.eu:en16931:2017", "fr.factur-x:en16931:1.0.8"},
		{"Factur-X EXTENDED", frcii.FormatFacturXExtended,
			"urn:cen.eu:en16931:2017#conformant#urn:factur-x.eu:1p0:extended", "fr.factur-x:extended:1.0.8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.guideline, tc.context.GuidelineID)
			assert.Equal(t, tc.vesID, tc.context.VESID)
			assert.Empty(t, tc.context.OutputGuidelineID, "hybrid profiles emit their own GuidelineID")
		})
	}
}

// TestPeppolFranceFacturXEmitsFacturXGuideline checks BT-24 names a Factur-X
// profile rather than the CTC one.
func TestPeppolFranceFacturXEmitsFacturXGuideline(t *testing.T) {
	assert.Equal(t,
		frcii.FormatFacturXExtended.GuidelineID,
		frcii.FormatPeppolFranceFacturX.OutputGuidelineID,
	)
}

// TestPeppolFranceExtendedGuidelines pins the extended context's external
// identification and the extended-ctc-fr guideline it emits in BT-24.
func TestPeppolFranceExtendedGuidelines(t *testing.T) {
	assert.Equal(t, "urn:cen.eu:en16931:2017#conformant#urn:peppol:france:billing:extended:1.0", frcii.FormatPeppolFranceExtended.GuidelineID)
	assert.Equal(t, "urn:peppol:france:billing:regulated", frcii.FormatPeppolFranceExtended.BusinessID)
	assert.Equal(t, "urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr", frcii.FormatPeppolFranceExtended.OutputGuidelineID)
}

func TestExtendedParties(t *testing.T) {
	const fixture = "peppol-france-extended/invoice-extended-parties.json"

	convert := func(t *testing.T, ctx goblcii.Format) *goblcii.Invoice {
		t.Helper()
		doc, err := goblcii.ExportInvoice(loadEnvelope(t, fixture), goblcii.WithFormat(ctx))
		require.NoError(t, err)
		return doc
	}

	t.Run("french extended maps the facturant, the addressee and the payer", func(t *testing.T) {
		stlm := convert(t, frcii.FormatPeppolFranceExtended).Transaction.Settlement

		// EXT-FR-FE-BG-05, pinned to UNCL 3035 "II" by EXT-FR-FE-113.
		require.NotNil(t, stlm.Invoicer)
		assert.Equal(t, "Facturant SARL", stlm.Invoicer.Name)
		assert.Equal(t, "II", stlm.Invoicer.RoleCode)
		assert.Equal(t, "524802931", stlm.Invoicer.LegalOrganization.ID.Value)

		// EXT-FR-FE-BG-04, pinned to UNCL 3035 "IV" by EXT-FR-FE-90.
		require.NotNil(t, stlm.Invoicee)
		assert.Equal(t, "Adressée SAS", stlm.Invoicee.Name)
		assert.Equal(t, "IV", stlm.Invoicee.RoleCode)
		require.NotEmpty(t, stlm.Invoicee.GlobalID)
		assert.Equal(t, "31419443800017", stlm.Invoicee.GlobalID[0].Value)
		assert.Equal(t, "0009", stlm.Invoicee.GlobalID[0].SchemeID)

		// EXT-FR-FE-BG-02.
		require.NotNil(t, stlm.Payer)
		assert.Equal(t, "Payeur SA", stlm.Payer.Name)
		require.NotEmpty(t, stlm.Payer.SpecifiedTaxRegistration)
		assert.Equal(t, "FR44391838042", stlm.Payer.SpecifiedTaxRegistration[0].ID.Value)
	})

	t.Run("french extended maps the seller and buyer agents", func(t *testing.T) {
		agmt := convert(t, frcii.FormatPeppolFranceExtended).Transaction.Agreement

		// CII keeps the agents beside the parties, not nested in them.
		require.NotNil(t, agmt.SalesAgent)
		assert.Equal(t, "Agent de Vendeur SAS", agmt.SalesAgent.Name)
		assert.Equal(t, "443061841", agmt.SalesAgent.LegalOrganization.ID.Value)

		require.NotNil(t, agmt.BuyerAgent)
		assert.Equal(t, "Agence Media SARL", agmt.BuyerAgent.Name)
		assert.Equal(t, "FR96552100554", agmt.BuyerAgent.SpecifiedTaxRegistration[0].ID.Value)
	})

	t.Run("the factur-x flavour of the profile maps them too", func(t *testing.T) {
		// ContextPeppolFranceFacturXV1 declares the Factur-X EXTENDED
		// guideline in BT-24 but is checked by the same EXTENDED-CTC-FR
		// rule set, so it carries the same parties.
		tr := convert(t, frcii.FormatPeppolFranceFacturX).Transaction

		require.NotNil(t, tr.Settlement.Invoicee)
		require.NotNil(t, tr.Settlement.Payer)
		require.NotNil(t, tr.Agreement.SalesAgent)
		require.NotNil(t, tr.Agreement.BuyerAgent)
		assert.Equal(t, "II", tr.Settlement.Invoicer.RoleCode)
	})

	t.Run("extended-only parties are ignored outside the french extended contexts", func(t *testing.T) {
		// The CIUS profile is the closest neighbour that must not carry them.
		for _, ctx := range []goblcii.Format{goblcii.FormatEN16931, frcii.FormatPeppolFranceCIUS} {
			tr := convert(t, ctx).Transaction
			assert.Nil(t, tr.Settlement.Invoicee)
			assert.Nil(t, tr.Settlement.Payer)
			assert.Nil(t, tr.Agreement.SalesAgent)
			assert.Nil(t, tr.Agreement.BuyerAgent)
			require.NotNil(t, tr.Settlement.Invoicer)
			assert.Empty(t, tr.Settlement.Invoicer.RoleCode)
		}
	})

	t.Run("parse restores every extended party", func(t *testing.T) {
		data, err := goblcii.Encode(convert(t, frcii.FormatPeppolFranceExtended))
		require.NoError(t, err)

		env, err := parseCII(data)
		require.NoError(t, err)
		out, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		require.NotNil(t, out.Ordering)
		require.NotNil(t, out.Ordering.Issuer)
		assert.Equal(t, "Facturant SARL", out.Ordering.Issuer.Name)
		require.NotNil(t, out.Ordering.Buyer)
		assert.Equal(t, "Adressée SAS", out.Ordering.Buyer.Name)
		require.NotNil(t, out.Payment)
		require.NotNil(t, out.Payment.Payer)
		assert.Equal(t, "Payeur SA", out.Payment.Payer.Name)
		require.NotNil(t, out.Supplier.Agent)
		assert.Equal(t, "Agent de Vendeur SAS", out.Supplier.Agent.Name)
		assert.Nil(t, out.Supplier.Agent.Agent)
		require.NotNil(t, out.Customer.Agent)
		assert.Equal(t, "Agence Media SARL", out.Customer.Agent.Name)
	})
}

// TestSubLinesRoundTrip writes a breakdown out as sub-invoice lines in the
// formats that allow them, and reads it back.
func TestSubLinesRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		fixture string
		format  goblcii.Format
	}{
		{"peppol-france-facturx/invoice-hierarchy.json", frcii.FormatPeppolFranceFacturX},
		{"peppol-france-extended/invoice-sub-lines.json", frcii.FormatPeppolFranceExtended},
	} {
		t.Run(tt.fixture, func(t *testing.T) {
			env := loadEnvelope(t, tt.fixture)
			inv, ok := env.Extract().(*bill.Invoice)
			require.True(t, ok)

			doc, err := goblcii.ExportInvoice(env, goblcii.WithFormat(tt.format))
			require.NoError(t, err)
			assert.Greater(t, len(doc.Transaction.Lines), len(inv.Lines), "sub-lines written")
			data, err := goblcii.Encode(doc)
			require.NoError(t, err)

			parsed, err := parseCII(data)
			require.NoError(t, err)
			out, ok := parsed.Extract().(*bill.Invoice)
			require.True(t, ok)
			assert.False(t, out.HasTags(tax.TagBypass))
			require.Len(t, out.Lines, len(inv.Lines))
			assert.Equal(t, inv.Totals.Payable.String(), out.Totals.Payable.String())
		})
	}
}

func TestConvertRegister(t *testing.T) {
	t.Run("formats", func(t *testing.T) {
		for _, k := range []cbc.Key{
			frcii.KeyPeppolFranceCIUS, frcii.KeyPeppolFranceExtended, frcii.KeyPeppolFranceFacturX,
			frcii.KeyFacturX, frcii.KeyFacturXBasic, frcii.KeyFacturXExtended, frcii.KeyChorusPro,
		} {
			f := convert.FormatFor(k)
			require.NotNil(t, f, k)
			assert.Equal(t, cbc.Key("cii"), f.Syntax)
		}
	})

	t.Run("detect and import", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(getParsePath(), "peppol-france-extended", "invoice-standard.xml"))
		require.NoError(t, err)
		f, err := convert.Detect(data)
		require.NoError(t, err)
		assert.Equal(t, frcii.KeyPeppolFranceExtended, f.Key)

		env, err := convert.Import(data)
		require.NoError(t, err)
		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		assert.Contains(t, inv.GetAddons(), flow2.V1)
	})

	t.Run("export", func(t *testing.T) {
		env := loadEnvelope(t, "peppol-france-cius/invoice-standard.json")
		out, err := convert.Export(env, frcii.KeyPeppolFranceCIUS)
		require.NoError(t, err)

		f, err := convert.Detect(out.Data)
		require.NoError(t, err)
		assert.Equal(t, frcii.KeyPeppolFranceCIUS, f.Key, "detected again")
	})
}
