package cii_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl"
	goblcii "github.com/invopop/gobl.cii"
	"github.com/invopop/gobl.fr.ctc/addon/flow2"
	frcii "github.com/invopop/gobl.fr.ctc/cii"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportFormats(t *testing.T) {
	t.Run("with France Factur-X CIUS context", func(t *testing.T) {
		env := loadEnvelope(t, "peppol-france-cius/invoice-standard.json")
		out, err := goblcii.ExportInvoice(env, goblcii.WithFormat(frcii.FormatCTCFacturXCIUS))
		require.NoError(t, err)

		// BT-24 is Factur-X EN 16931's own value, BT-23 the billing mode.
		assert.Equal(t, "urn:cen.eu:en16931:2017", out.ExchangedContext.GuidelineContext.ID)
		require.NotNil(t, out.ExchangedContext.BusinessContext)
		assert.Equal(t, "S1", out.ExchangedContext.BusinessContext.ID)
	})

	t.Run("with France Extended context", func(t *testing.T) {
		env := loadEnvelope(t, "peppol-france-extended/invoice-standard.json")
		out, err := goblcii.ExportInvoice(env, goblcii.WithFormat(frcii.FormatCTCExtended))
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
		assert.Equal(t, frcii.FormatCTCCIUS.GuidelineID, ctx.GuidelineID)
	})

	t.Run("find France CIUS by billing mode BusinessID", func(t *testing.T) {
		// France CIUS documents use EN16931 GuidelineID but have a billing mode as BusinessID
		ctx := goblcii.FindFormat("urn:cen.eu:en16931:2017", "B1")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatCTCCIUS.GuidelineID, ctx.GuidelineID)

		ctx = goblcii.FindFormat("urn:cen.eu:en16931:2017", "S1")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatCTCCIUS.GuidelineID, ctx.GuidelineID)

		ctx = goblcii.FindFormat("urn:cen.eu:en16931:2017", "M4")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatCTCCIUS.GuidelineID, ctx.GuidelineID)
	})

	t.Run("find the French contexts by their spec-level GuidelineID", func(t *testing.T) {
		// Some senders put the identifier that names the profile on the network
		// into the document instead of the guideline the profile emits.
		for _, mode := range []string{"B1", "S1", "M4"} {
			ctx := goblcii.FindFormat(frcii.FormatCTCCIUS.GuidelineID, mode)
			require.NotNil(t, ctx, "mode %s", mode)
			assert.Equal(t, frcii.FormatCTCCIUS.GuidelineID, ctx.GuidelineID)
			assert.Equal(t, frcii.FormatCTCCIUS.VESID, ctx.VESID)

			ctx = goblcii.FindFormat(frcii.FormatCTCFacturXExtended.GuidelineID, mode)
			require.NotNil(t, ctx, "mode %s", mode)
			assert.Equal(t, frcii.FormatCTCFacturXExtended.GuidelineID, ctx.GuidelineID)
			assert.Equal(t, frcii.FormatCTCFacturXExtended.VESID, ctx.VESID)

			ctx = goblcii.FindFormat(frcii.FormatCTCExtended.GuidelineID, mode)
			require.NotNil(t, ctx, "mode %s", mode)
			assert.Equal(t, frcii.FormatCTCExtended.GuidelineID, ctx.GuidelineID)
			assert.Equal(t, frcii.FormatCTCExtended.VESID, ctx.VESID)
		}
	})

	t.Run("find France Extended by full GuidelineID", func(t *testing.T) {
		ctx := goblcii.FindFormat("urn:cen.eu:en16931:2017#conformant#urn:peppol:france:billing:extended:1.0", "urn:peppol:france:billing:regulated")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatCTCExtended.GuidelineID, ctx.GuidelineID)
	})

	t.Run("find France Extended by the guideline it emits", func(t *testing.T) {
		// The extended-ctc-fr guideline is unique to this context, so it
		// resolves both with and without a billing mode BusinessID.
		ctx := goblcii.FindFormat("urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr", "B1")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatCTCExtended.GuidelineID, ctx.GuidelineID)

		ctx = goblcii.FindFormat("urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr", "")
		require.NotNil(t, ctx)
		assert.Equal(t, frcii.FormatCTCExtended.GuidelineID, ctx.GuidelineID)
	})

	t.Run("find France Factur-X by the guideline it emits and a French BT-23", func(t *testing.T) {
		for _, bt23 := range []string{"B1", "S5"} {
			ctx := goblcii.FindFormat(frcii.GuidelineIDFacturXExtended, bt23)
			require.NotNil(t, ctx, "BT-23 %q", bt23)
			assert.Equal(t, frcii.KeyCTCFacturXExtended, ctx.Key)
		}
	})

	t.Run("find France Factur-X BASIC WL by either guideline and a French BT-23", func(t *testing.T) {
		for _, guidelineID := range []string{frcii.GuidelineIDFacturXBasicWL, frcii.GuidelineIDZUGFeRDBasicWL} {
			ctx := goblcii.FindFormat(guidelineID, "M1")
			require.NotNil(t, ctx, guidelineID)
			assert.Equal(t, frcii.KeyCTCFacturXBasicWL, ctx.Key)
		}
	})

	t.Run("leave BASIC WL without a French BT-23 alone", func(t *testing.T) {
		// flow2 would otherwise fill in a billing mode the document never gave.
		for _, guidelineID := range []string{frcii.GuidelineIDFacturXBasicWL, frcii.GuidelineIDZUGFeRDBasicWL} {
			for _, bt23 := range []string{"", frcii.ProcessIDRegulated, "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0"} {
				assert.Nil(t, goblcii.FindFormat(guidelineID, bt23), "%s with BT-23 %q", guidelineID, bt23)
			}
		}
	})

	t.Run("leave plain Factur-X and ZUGFeRD EXTENDED to EN 16931", func(t *testing.T) {
		// ZUGFeRD 2.1+ EXTENDED carries the same guideline, so without a
		// French BT-23 it is not a French CTC document.
		for _, bt23 := range []string{"", "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0", frcii.ProcessIDRegulated, "A1"} {
			assert.Nil(t, goblcii.FindFormat(frcii.GuidelineIDFacturXExtended, bt23), "BT-23 %q", bt23)
		}
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
			&frcii.FormatCTCExtended,
		},
		{
			"unsuffixed cpro URN",
			"urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0",
			&frcii.FormatCTCExtended,
		},
		{
			"mangled EN16931 URN",
			"urn.eu:en16931:2017",
			&frcii.FormatCTCExtended,
		},
		{
			// BT-24 absent: the billing mode is all there is.
			"absent guideline",
			"",
			&frcii.FormatCTCExtended,
		},
		{
			// Every other profile's BusinessProcessParameter is a long URN.
			"unrelated guideline still follows the billing mode",
			"urn:peppol:pint:billing-1@sg-1",
			&frcii.FormatCTCExtended,
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

// TestFacturXProfiles pins each Factur-X profile to the BT-24 value its
// codelist admits and to its Flux 2 rule set; a wrong guideline invalidates
// every document.
func TestFacturXProfiles(t *testing.T) {
	assert.Equal(t, "urn:cen.eu:en16931:2017", frcii.FormatCTCFacturXCIUS.GuidelineID)
	assert.Empty(t, frcii.FormatCTCFacturXCIUS.OutputGuidelineID)
	assert.Equal(t, "fr.ctc:facturx-en16931:1.4.0-04", frcii.FormatCTCFacturXCIUS.VESID)

	assert.Empty(t, frcii.FormatCTCFacturXExtended.OutputGuidelineID)
	assert.Equal(t, "urn:cen.eu:en16931:2017#conformant#urn:factur-x.eu:1p0:extended", frcii.GuidelineIDFacturXExtended)
	assert.Equal(t, "fr.ctc:facturx-extended:1.4.0-04", frcii.FormatCTCFacturXExtended.VESID)

	assert.Equal(t, "urn:factur-x.eu:1p0:basicwl", frcii.GuidelineIDFacturXBasicWL)
	assert.Equal(t, "urn:zugferd.de:2p0:basicwl", frcii.GuidelineIDZUGFeRDBasicWL)
	assert.Equal(t, "fr.ctc:facturx-basicwl:1.4.0-04", frcii.FormatCTCFacturXBasicWL.VESID)
}

// BASIC WL documents carry their totals and VAT breakdown but no lines: they
// import with the totals they state, and validate without lines.
func TestFacturXBasicWLImport(t *testing.T) {
	for _, tt := range []struct {
		fixture string
		sum     string
		total   string
		tax     string
		payable string
	}{
		{"invoice-allowances-charges.xml", "95.00", "100.00", "4.90", "104.90"},
		{"invoice-exempt.xml", "95.00", "100.00", "8.00", "108.00"},
	} {
		t.Run(tt.fixture, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(getParsePath(), "peppol-france-facturx-basicwl", tt.fixture))
			require.NoError(t, err)
			f, err := convert.Detect(data)
			require.NoError(t, err)
			assert.Equal(t, frcii.KeyCTCFacturXBasicWL, f.Key)

			env, err := parseCII(data)
			require.NoError(t, err)
			inv, ok := env.Extract().(*bill.Invoice)
			require.True(t, ok)
			assert.Empty(t, inv.Lines)
			assert.True(t, inv.HasTags(tax.TagNoLines, tax.TagBypass), "tags %v", inv.GetTags())
			assert.Contains(t, inv.GetAddons(), flow2.V1)
			require.NotNil(t, inv.Totals)
			assert.Equal(t, tt.sum, inv.Totals.Sum.String())
			assert.Equal(t, tt.total, inv.Totals.Total.String())
			assert.Equal(t, tt.tax, inv.Totals.Tax.String())
			assert.Equal(t, tt.payable, inv.Totals.Payable.String())
			assert.NoError(t, env.Validate())
		})
	}
}

// The plain Factur-X profiles share their guidelines with ZUGFeRD and EN 16931,
// so they are export-only, which keeps them out of detection.
func TestPlainFacturX(t *testing.T) {
	t.Run("never detected", func(t *testing.T) {
		assert.Equal(t, goblcii.FormatEN16931.Key, goblcii.FindFormat(goblcii.GuidelineIDEN16931, "").Key)
		assert.Nil(t, goblcii.FindFormat(frcii.GuidelineIDFacturXExtended, ""))
	})

	t.Run("export-only in the convert register", func(t *testing.T) {
		for _, k := range []cbc.Key{frcii.KeyFacturX, frcii.KeyFacturXExtended} {
			f := convert.FormatFor(k)
			require.NotNil(t, f, k)
			assert.NotEmpty(t, f.Export, k)
			assert.Empty(t, f.Import, k)
		}
		assert.Nil(t, convert.FormatFor("cii+fr-facturx-v1+basic"))
	})

	t.Run("export writes each profile's guideline", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join("testdata", "convert", "facturx-extended", "out", "invoice-complete.xml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), frcii.GuidelineIDFacturXExtended)
		data, err = os.ReadFile(filepath.Join("testdata", "convert", "facturx", "out", "invoice-complete.xml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "<ram:ID>"+goblcii.GuidelineIDEN16931+"</ram:ID>")
	})
}

// TestPeppolFranceExtendedGuidelines pins the extended context's external
// identification and the extended-ctc-fr guideline it emits in BT-24.
func TestPeppolFranceExtendedGuidelines(t *testing.T) {
	assert.Equal(t, "urn:cen.eu:en16931:2017#conformant#urn:peppol:france:billing:extended:1.0", frcii.FormatCTCExtended.GuidelineID)
	assert.Equal(t, "urn:peppol:france:billing:regulated", frcii.FormatCTCExtended.BusinessID)
	assert.Equal(t, "urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr", frcii.FormatCTCExtended.OutputGuidelineID)
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
		stlm := convert(t, frcii.FormatCTCExtended).Transaction.Settlement

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
		agmt := convert(t, frcii.FormatCTCExtended).Transaction.Agreement

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
		tr := convert(t, frcii.FormatCTCFacturXExtended).Transaction

		require.NotNil(t, tr.Settlement.Invoicee)
		require.NotNil(t, tr.Settlement.Payer)
		require.NotNil(t, tr.Agreement.SalesAgent)
		require.NotNil(t, tr.Agreement.BuyerAgent)
		assert.Equal(t, "II", tr.Settlement.Invoicer.RoleCode)
	})

	t.Run("extended-only parties are ignored outside the french extended contexts", func(t *testing.T) {
		// The CIUS profile is the closest neighbour that must not carry them.
		for _, ctx := range []goblcii.Format{goblcii.FormatEN16931, frcii.FormatCTCCIUS} {
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
		data, err := goblcii.Encode(convert(t, frcii.FormatCTCExtended))
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
		{"peppol-france-facturx/invoice-hierarchy.json", frcii.FormatCTCFacturXExtended},
		{"peppol-france-extended/invoice-sub-lines.json", frcii.FormatCTCExtended},
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
			frcii.KeyCTCCIUS, frcii.KeyCTCExtended,
			frcii.KeyCTCFacturXCIUS, frcii.KeyCTCFacturXExtended, frcii.KeyChorusPro,
		} {
			f := convert.FormatFor(k)
			require.NotNil(t, f, k)
			assert.Equal(t, cbc.Key("cii"), f.Syntax)
			if k != frcii.KeyCTCFacturXCIUS {
				assert.NotEmpty(t, f.Import, k)
			}
			assert.True(t, f.Attachments, k)
		}
	})

	t.Run("BASIC WL is import-only", func(t *testing.T) {
		f := convert.FormatFor(frcii.KeyCTCFacturXBasicWL)
		require.NotNil(t, f)
		assert.NotEmpty(t, f.Import)
		assert.Empty(t, f.Export)
	})

	t.Run("detect and import", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(getParsePath(), "peppol-france-extended", "invoice-standard.xml"))
		require.NoError(t, err)
		f, err := convert.Detect(data)
		require.NoError(t, err)
		assert.Equal(t, frcii.KeyCTCExtended, f.Key)

		imported, err := convert.Import(data)
		require.NoError(t, err)
		env := imported.Envelope
		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		assert.Contains(t, inv.GetAddons(), flow2.V1)
	})

	t.Run("export", func(t *testing.T) {
		env := loadEnvelope(t, "peppol-france-cius/invoice-standard.json")
		out, err := convert.Export(env, frcii.KeyCTCCIUS)
		require.NoError(t, err)

		f, err := convert.Detect(out.Data)
		require.NoError(t, err)
		assert.Equal(t, frcii.KeyCTCCIUS, f.Key, "detected again")
	})

	t.Run("attachments", func(t *testing.T) {
		env := loadEnvelope(t, "peppol-france-cius/invoice-standard.json")
		pdf := &convert.Attachment{Name: "invoice.pdf", MIME: "application/pdf", Data: []byte("%PDF")}
		out, err := convert.Export(env, frcii.KeyCTCFacturXCIUS, convert.WithAttachments(pdf))
		require.NoError(t, err)
		doc, err := goblcii.Decode(out.Data)
		require.NoError(t, err)
		atts := doc.(*goblcii.Invoice).ExtractBinaryAttachments()
		require.Len(t, atts, 1)
		assert.Equal(t, "invoice.pdf", atts[0].Filename)

		imported, err := convert.Import(out.Data)
		require.NoError(t, err)
		require.Len(t, imported.Attachments, 1)
		assert.Equal(t, pdf.Data, imported.Attachments[0].Data)
	})

	t.Run("factur-x cius is export-only", func(t *testing.T) {
		env := loadEnvelope(t, "peppol-france-cius/invoice-standard.json")
		out, err := convert.Export(env, frcii.KeyCTCFacturXCIUS)
		require.NoError(t, err)
		d, err := convert.Detect(out.Data)
		require.NoError(t, err)
		assert.Equal(t, frcii.KeyCTCCIUS, d.Key, "its CII reads as CIUS")
	})

	t.Run("factur-x extended is detected", func(t *testing.T) {
		env := loadEnvelope(t, "peppol-france-facturx/invoice-standard.json")
		out, err := convert.Export(env, frcii.KeyCTCFacturXExtended)
		require.NoError(t, err)
		d, err := convert.Detect(out.Data)
		require.NoError(t, err)
		assert.Equal(t, frcii.KeyCTCFacturXExtended, d.Key)

		imported, err := convert.Import(out.Data, convert.WithFormats(frcii.KeyCTCFacturXExtended))
		require.NoError(t, err)
		assert.Equal(t, frcii.KeyCTCFacturXExtended, imported.Format.Key)
	})
}

// TestDeliveryTerms maps the Incoterms delivery terms (EXT-FR-FE-BG-14): the
// code to the delivery extension and the location name to the label of the
// receiver's address, apart from the delivery location ID (BT-71).
func TestDeliveryTerms(t *testing.T) {
	const fixture = "peppol-france-extended/invoice-standard.json"
	withDelivery := func(t *testing.T) *gobl.Envelope {
		t.Helper()
		env := loadEnvelope(t, fixture)
		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		inv.Delivery = &bill.DeliveryDetails{
			Receiver: &org.Party{
				Name:      "Warehouse SAS",
				Addresses: []*org.Address{{Label: "Port of Le Havre", Street: "1 Quai", Locality: "Le Havre", Code: "76600", Country: "FR"}},
			},
			Identities: []*org.Identity{{Code: "3012345678901", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: "0088"})}},
			Ext:        tax.ExtensionsOf(cbc.CodeMap{flow2.ExtKeyDeliveryTerms: "DAP"}),
		}
		return env
	}

	for _, f := range []goblcii.Format{frcii.FormatCTCExtended, frcii.FormatCTCFacturXExtended} {
		t.Run(f.Key.String()+" writes the terms next to BT-71", func(t *testing.T) {
			doc, err := goblcii.ExportInvoice(withDelivery(t), goblcii.WithFormat(f))
			require.NoError(t, err)

			terms := doc.Transaction.Agreement.DeliveryTerms
			require.NotNil(t, terms)
			assert.Equal(t, "DAP", terms.Code)
			require.NotNil(t, terms.Location)
			assert.Equal(t, "Port of Le Havre", terms.Location.Name)

			rcv := doc.Transaction.Delivery.Receiver
			require.NotNil(t, rcv)
			require.Len(t, rcv.GlobalID, 1)
			assert.Equal(t, "3012345678901", rcv.GlobalID[0].Value)
			assert.Equal(t, "0088", rcv.GlobalID[0].SchemeID)
		})
	}

	t.Run("cius leaves the terms out", func(t *testing.T) {
		doc, err := goblcii.ExportInvoice(withDelivery(t), goblcii.WithFormat(frcii.FormatCTCCIUS))
		require.NoError(t, err)
		assert.Nil(t, doc.Transaction.Agreement.DeliveryTerms)
	})

	t.Run("parse restores the terms and BT-71", func(t *testing.T) {
		doc, err := goblcii.ExportInvoice(withDelivery(t), goblcii.WithFormat(frcii.FormatCTCExtended))
		require.NoError(t, err)
		data, err := goblcii.Encode(doc)
		require.NoError(t, err)

		env, err := parseCII(data)
		require.NoError(t, err)
		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		require.NotNil(t, inv.Delivery)
		assert.Equal(t, cbc.Code("DAP"), inv.Delivery.Ext.Get(flow2.ExtKeyDeliveryTerms))
		require.NotNil(t, inv.Delivery.Receiver)
		require.Len(t, inv.Delivery.Receiver.Addresses, 1)
		assert.Equal(t, "Port of Le Havre", inv.Delivery.Receiver.Addresses[0].Label)
		require.Len(t, inv.Delivery.Identities, 1)
		assert.Equal(t, cbc.Code("3012345678901"), inv.Delivery.Identities[0].Code)
		assert.Equal(t, cbc.Code("0088"), inv.Delivery.Identities[0].Ext.Get(iso.ExtKeySchemeID))
	})
}

// EXT-FR-FE-01: the contract type travels as a UNCL 1153 code (BR-FR-CPRO-01).
func TestContractType(t *testing.T) {
	env := loadEnvelope(t, "peppol-france-extended/invoice-standard.json")
	inv := env.Extract().(*bill.Invoice)
	if inv.Ordering == nil {
		inv.Ordering = new(bill.Ordering)
	}
	inv.Ordering.Contracts = []*org.DocumentRef{{
		Code: "MARCHE-2026-0042",
		Ext:  tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyReference: "CT"}),
	}}
	require.NoError(t, env.Calculate())

	doc, err := goblcii.ExportInvoice(env, goblcii.WithFormat(frcii.FormatCTCExtended))
	require.NoError(t, err)
	require.NotNil(t, doc.Transaction.Agreement.Contract)
	assert.Equal(t, "CT", doc.Transaction.Agreement.Contract.ReferenceTypeCode)

	data, err := goblcii.Encode(doc)
	require.NoError(t, err)
	imported, err := convert.Import(data)
	require.NoError(t, err)
	c := imported.Envelope.Extract().(*bill.Invoice).Ordering.Contracts[0]
	assert.Equal(t, cbc.Code("CT"), c.Ext.Get(untdid.ExtKeyReference))
	assert.NoError(t, imported.Envelope.Validate())
}

func TestImportDocumentType(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(getParsePath(), "peppol-france-extended", "invoice-standard.xml"))
	require.NoError(t, err)

	tests := []struct {
		code string
		typ  cbc.Key
		tags []cbc.Key
	}{
		{"386", bill.InvoiceTypeStandard, []cbc.Key{tax.TagPrepayment}},
		{"393", bill.InvoiceTypeStandard, []cbc.Key{tax.TagFactoring}},
		{"500", bill.InvoiceTypeStandard, []cbc.Key{tax.TagSelfBilled, tax.TagPrepayment}},
		{"501", bill.InvoiceTypeStandard, []cbc.Key{tax.TagSelfBilled, tax.TagFactoring}},
		{"471", bill.InvoiceTypeCorrective, []cbc.Key{tax.TagSelfBilled}},
		{"472", bill.InvoiceTypeCorrective, []cbc.Key{tax.TagFactoring}},
		{"473", bill.InvoiceTypeCorrective, []cbc.Key{tax.TagSelfBilled, tax.TagFactoring}},
		{"261", bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagSelfBilled}},
		{"396", bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagFactoring}},
		{"502", bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagSelfBilled, tax.TagFactoring}},
		{"503", bill.InvoiceTypeCreditNote, []cbc.Key{tax.TagPrepayment}},
		{"262", bill.InvoiceTypeCreditNote, []cbc.Key{flow2.TagGlobal}},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			data := strings.Replace(string(raw),
				"<ram:TypeCode>380</ram:TypeCode>",
				"<ram:TypeCode>"+tt.code+"</ram:TypeCode>", 1)
			imported, err := convert.Import([]byte(data))
			require.NoError(t, err)
			inv, ok := imported.Envelope.Extract().(*bill.Invoice)
			require.True(t, ok)

			assert.Equal(t, tt.typ, inv.Type)
			assert.Equal(t, tt.tags, inv.Tags.List)
			assert.Equal(t, cbc.Code(tt.code), inv.Tax.Ext.Get(untdid.ExtKeyDocumentType),
				"the scenario must regenerate the stated code")
		})
	}
}
