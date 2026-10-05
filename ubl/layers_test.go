package ubl_test

import (
	"testing"

	"github.com/invopop/gobl"
	_ "github.com/invopop/gobl.fr.ctc/addon"
	"github.com/invopop/gobl.fr.ctc/addon/flow2"
	frubl "github.com/invopop/gobl.fr.ctc/ubl"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaxExchangeRate(t *testing.T) {
	const fixture = "france-extended/invoice-tax-exchange-rate.json"

	t.Run("french extended maps the VAT accounting currency exchange rate", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, frubl.ContextExtended)
		require.NoError(t, err)

		require.NotNil(t, doc.TaxExchangeRate)
		require.NotNil(t, doc.TaxExchangeRate.SourceCurrencyCode)
		assert.Equal(t, "USD", *doc.TaxExchangeRate.SourceCurrencyCode)
		require.NotNil(t, doc.TaxExchangeRate.TargetCurrencyCode)
		assert.Equal(t, "EUR", *doc.TaxExchangeRate.TargetCurrencyCode)
		require.NotNil(t, doc.TaxExchangeRate.CalculationRate)
		assert.Equal(t, "0.92", *doc.TaxExchangeRate.CalculationRate)
		require.NotNil(t, doc.TaxExchangeRate.Date)
		assert.Equal(t, "2024-06-13", *doc.TaxExchangeRate.Date)
	})

	t.Run("exchange rate is ignored outside the french extended context", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, ubl.ContextPeppol)
		require.NoError(t, err)

		assert.Nil(t, doc.TaxExchangeRate)
	})

	t.Run("parse restores the exchange rate", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, frubl.ContextExtended)
		require.NoError(t, err)
		data, err := ubl.Bytes(doc)
		require.NoError(t, err)

		parsed, err := ubl.Parse(data)
		require.NoError(t, err)
		in, ok := parsed.(*ubl.Invoice)
		require.True(t, ok)
		env, err := in.Convert()
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.Len(t, inv.ExchangeRates, 1)
		rate := inv.ExchangeRates[0]
		assert.Equal(t, "USD", rate.From.String())
		assert.Equal(t, "EUR", rate.To.String())
		assert.Equal(t, "0.92", rate.Amount.String())
		require.NotNil(t, rate.At)
		assert.Equal(t, "2024-06-13T00:00:00", rate.At.String())
	})

	t.Run("parse honors cac:TaxExchangeRate regardless of context", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, frubl.ContextExtended)
		require.NoError(t, err)
		data, err := ubl.Bytes(doc)
		require.NoError(t, err)

		parsed, err := ubl.Parse(data)
		require.NoError(t, err)
		in, ok := parsed.(*ubl.Invoice)
		require.True(t, ok)
		// Force a non-French-extended context on parse; the element should
		// still be honored since the source document carries it explicitly.
		env, err := in.Convert(ubl.WithContext(ubl.ContextPeppol))
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.Len(t, inv.ExchangeRates, 1)
		rate := inv.ExchangeRates[0]
		assert.Equal(t, "USD", rate.From.String())
		assert.Equal(t, "EUR", rate.To.String())
		assert.Equal(t, "0.92", rate.Amount.String())
	})

	t.Run("mismatched cac:TaxExchangeRate currencies are ignored", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, frubl.ContextExtended)
		require.NoError(t, err)
		data, err := ubl.Bytes(doc)
		require.NoError(t, err)

		parsed, err := ubl.Parse(data)
		require.NoError(t, err)
		in, ok := parsed.(*ubl.Invoice)
		require.True(t, ok)

		// Corrupt the source currency so it no longer matches
		// DocumentCurrencyCode; it should not be trusted.
		mismatched := "GBP"
		in.TaxExchangeRate.SourceCurrencyCode = &mismatched

		env, err := in.Convert()
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.Len(t, inv.ExchangeRates, 1)
		// Falls back to the TaxTotal-derived heuristic instead of the
		// inconsistent element.
		assert.Equal(t, "USD", inv.ExchangeRates[0].From.String())
		assert.Equal(t, "EUR", inv.ExchangeRates[0].To.String())
	})
}

func TestOrderingExtendedParties(t *testing.T) {
	const fixture = "france-extended/invoice-addressee.json"

	t.Run("french extended maps the addressee and the facturant", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, frubl.ContextExtended)
		require.NoError(t, err)

		// EXT-FR-FE-BG-05 sits under the seller, EXT-FR-FE-BG-04 under the buyer.
		require.NotNil(t, doc.AccountingSupplierParty.Party.ServiceProviderParty)
		facturant := doc.AccountingSupplierParty.Party.ServiceProviderParty.Party
		require.NotNil(t, facturant)
		assert.Equal(t, "Facturant SARL", facturant.PartyName.Name)
		assert.Equal(t, "II", facturant.IndustryClassificationCode)
		assert.Equal(t, "524802931", facturant.PartyLegalEntity.CompanyID.Value)
		assert.Equal(t, "0002", *facturant.PartyLegalEntity.CompanyID.SchemeID)

		require.NotNil(t, doc.AccountingCustomerParty.Party.ServiceProviderParty)
		addressee := doc.AccountingCustomerParty.Party.ServiceProviderParty.Party
		require.NotNil(t, addressee)
		assert.Equal(t, "Adressée SAS", addressee.PartyName.Name)
		assert.Equal(t, "IV", addressee.IndustryClassificationCode)
		require.NotEmpty(t, addressee.PartyIdentification)
		assert.Equal(t, "31419443800017", addressee.PartyIdentification[0].ID.Value)
		assert.Equal(t, "0009", *addressee.PartyIdentification[0].ID.SchemeID)
		require.NotNil(t, addressee.Contact)
		assert.Equal(t, "factures@adressee.fr", *addressee.Contact.ElectronicMail)
	})

	t.Run("addressee is ignored outside the french extended context", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, ubl.ContextPeppol)
		require.NoError(t, err)

		assert.Nil(t, doc.AccountingCustomerParty.Party.ServiceProviderParty)
		// The facturant is not extended-only, but its role code is.
		require.NotNil(t, doc.AccountingSupplierParty.Party.ServiceProviderParty)
		assert.Empty(t, doc.AccountingSupplierParty.Party.ServiceProviderParty.Party.IndustryClassificationCode)
	})

	t.Run("parse restores both parties", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, frubl.ContextExtended)
		require.NoError(t, err)
		data, err := ubl.Bytes(doc)
		require.NoError(t, err)

		parsed, err := ubl.Parse(data)
		require.NoError(t, err)
		in, ok := parsed.(*ubl.Invoice)
		require.True(t, ok)
		outEnv, err := in.Convert()
		require.NoError(t, err)
		outInv, ok := outEnv.Extract().(*bill.Invoice)
		require.True(t, ok)

		require.NotNil(t, outInv.Ordering)
		require.NotNil(t, outInv.Ordering.Issuer)
		assert.Equal(t, "Facturant SARL", outInv.Ordering.Issuer.Name)
		require.NotNil(t, outInv.Ordering.Buyer)
		assert.Equal(t, "Adressée SAS", outInv.Ordering.Buyer.Name)
		assert.Equal(t, "FR85314194438", outInv.Ordering.Buyer.TaxID.String())
	})
}

func TestPartyAgent(t *testing.T) {
	const fixture = "france-extended/invoice-addressee.json"

	t.Run("french extended nests the buyer and seller agents", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, frubl.ContextExtended)
		require.NoError(t, err)

		// EXT-FR-FE-BG-03 sits under the seller, EXT-FR-FE-BG-01 under the buyer.
		sellerAgent := doc.AccountingSupplierParty.Party.AgentParty
		require.NotNil(t, sellerAgent)
		assert.Equal(t, "Agent de Vendeur SAS", sellerAgent.PartyName.Name)
		assert.Equal(t, "443061841", sellerAgent.PartyLegalEntity.CompanyID.Value)
		assert.Equal(t, "0002", *sellerAgent.PartyLegalEntity.CompanyID.SchemeID)

		buyerAgent := doc.AccountingCustomerParty.Party.AgentParty
		require.NotNil(t, buyerAgent)
		assert.Equal(t, "Agence Media SARL", buyerAgent.PartyName.Name)
		assert.Equal(t, "FR96552100554", buyerAgent.PartyTaxScheme[0].CompanyID.Value)
	})

	t.Run("agents are ignored outside the french extended context", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, ubl.ContextPeppol)
		require.NoError(t, err)

		assert.Nil(t, doc.AccountingSupplierParty.Party.AgentParty)
		assert.Nil(t, doc.AccountingCustomerParty.Party.AgentParty)
	})

	t.Run("parse restores both agents", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, frubl.ContextExtended)
		require.NoError(t, err)
		data, err := ubl.Bytes(doc)
		require.NoError(t, err)

		parsed, err := ubl.Parse(data)
		require.NoError(t, err)
		in, ok := parsed.(*ubl.Invoice)
		require.True(t, ok)
		outEnv, err := in.Convert()
		require.NoError(t, err)
		outInv, ok := outEnv.Extract().(*bill.Invoice)
		require.True(t, ok)

		require.NotNil(t, outInv.Supplier.Agent)
		assert.Equal(t, "Agent de Vendeur SAS", outInv.Supplier.Agent.Name)
		assert.Nil(t, outInv.Supplier.Agent.Agent)
		require.NotNil(t, outInv.Customer.Agent)
		assert.Equal(t, "Agence Media SARL", outInv.Customer.Agent.Name)
	})
}

func TestPaymentPayer(t *testing.T) {
	const fixture = "france-extended/invoice-payer.json"

	t.Run("french extended maps the payer to the payment mandate", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, frubl.ContextExtended)
		require.NoError(t, err)

		require.NotEmpty(t, doc.PaymentMeans)
		mandate := doc.PaymentMeans[0].PaymentMandate
		require.NotNil(t, mandate)
		payer := mandate.PayerParty
		require.NotNil(t, payer)
		assert.Equal(t, "Payeur SA", payer.PartyName.Name)
		require.NotEmpty(t, payer.PartyIdentification)
		assert.Equal(t, "39183804200003", payer.PartyIdentification[0].ID.Value)
		assert.Equal(t, "0009", *payer.PartyIdentification[0].ID.SchemeID)
		require.NotNil(t, payer.PartyLegalEntity)
		assert.Equal(t, "391838042", payer.PartyLegalEntity.CompanyID.Value)
		assert.Equal(t, "0002", *payer.PartyLegalEntity.CompanyID.SchemeID)

		// The payee travels alongside the payer (BG-10).
		require.NotNil(t, doc.PayeeParty)
		assert.Equal(t, "Bénéficiaire SARL", doc.PayeeParty.PartyName.Name)
	})

	t.Run("payer without payment instructions synthesizes the payment means", func(t *testing.T) {
		env := loadTestEnvelope(t, fixture)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		inv.Payment.Instructions = nil

		doc, err := ubl.ConvertInvoice(env, ubl.WithContext(frubl.ContextExtended))
		require.NoError(t, err)
		require.NotEmpty(t, doc.PaymentMeans)
		assert.Equal(t, "1", doc.PaymentMeans[0].PaymentMeansCode.Value)
		require.NotNil(t, doc.PaymentMeans[0].PaymentMandate)
		assert.Nil(t, doc.PaymentMeans[0].PaymentMandate.ID)
		assert.NotNil(t, doc.PaymentMeans[0].PaymentMandate.PayerParty)
	})

	t.Run("payer is ignored outside the french extended context", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, ubl.ContextPeppol)
		require.NoError(t, err)

		require.NotEmpty(t, doc.PaymentMeans)
		assert.Nil(t, doc.PaymentMeans[0].PaymentMandate)
	})

	t.Run("parse restores the payer without inventing a direct debit", func(t *testing.T) {
		doc, err := testInvoiceFromContext(fixture, frubl.ContextExtended)
		require.NoError(t, err)
		data, err := ubl.Bytes(doc)
		require.NoError(t, err)

		parsed, err := ubl.Parse(data)
		require.NoError(t, err)
		in, ok := parsed.(*ubl.Invoice)
		require.True(t, ok)
		env, err := in.Convert()
		require.NoError(t, err)

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, inv.Payment)
		require.NotNil(t, inv.Payment.Payer)
		assert.Equal(t, "Payeur SA", inv.Payment.Payer.Name)
		require.NotNil(t, inv.Payment.Payee)
		assert.Equal(t, "Bénéficiaire SARL", inv.Payment.Payee.Name)
		require.NotNil(t, inv.Payment.Instructions)
		assert.Nil(t, inv.Payment.Instructions.DirectDebit)
	})
}

func TestLineTaxPoint(t *testing.T) {
	t.Run("line tax point conversion", func(t *testing.T) {
		env := loadTestEnvelope(t, "france-extended/invoice-standard.json")

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		inv.Tax.Point = tax.PointDelivery
		inv.Lines[0].Period = &cal.Period{
			Start: cal.NewDate(2024, 1, 1),
			End:   cal.NewDate(2024, 1, 31),
		}

		out, err := ubl.ConvertInvoice(env, ubl.WithContext(frubl.ContextExtended))
		require.NoError(t, err)

		require.NotNil(t, out.InvoiceLines[0].InvoicePeriod)
		assert.Equal(t, "35", out.InvoiceLines[0].InvoicePeriod.DescriptionCode)

		// Outside the France extended context the line period carries no code.
		out, err = ubl.ConvertInvoice(env)
		require.NoError(t, err)

		require.NotNil(t, out.InvoiceLines[0].InvoicePeriod)
		assert.Empty(t, out.InvoiceLines[0].InvoicePeriod.DescriptionCode)
	})
}

// TestParseSupplierIdentifiers checks that the supplier keeps its own BT-31
// VAT number and BT-34 endpoint when the party carries extra identifiers and
// the invoice names a BG-11 tax representative, as the French "assujetti
// unique" (VAT group) invoices do.
func TestParseSupplierIdentifiers(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		taxID       cbc.Code
		inboxScheme cbc.Code
		inboxCode   cbc.Code
		groupSIREN  cbc.Code // BT-29d, the 0231 VAT group identifier
		repName     string   // BT-62, empty when there is no tax representative
		repTaxID    cbc.Code // BT-63
	}{
		{
			name:        "ordinary identifiers",
			file:        "france-extended/b2g-invoice.xml",
			taxID:       "53341200068",
			inboxScheme: "0225",
			inboxCode:   "341200068",
		},
		{
			name:        "assujetti unique",
			file:        "france-extended/b2g-assujetti-unique.xml",
			taxID:       "53341200068",
			inboxScheme: "0225",
			inboxCode:   "341200068",
			groupSIREN:  "123456789",
			repName:     "Fournisseur 34120006871491 ASSUJETTI UNIQUE",
			repTaxID:    "00123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := parseXMLInvoice(t, tt.file)

			inv, ok := e.Extract().(*bill.Invoice)
			require.True(t, ok)

			supplier := inv.Supplier
			require.NotNil(t, supplier)
			require.NotNil(t, supplier.TaxID)
			assert.Equal(t, l10n.TaxCountryCode("FR"), supplier.TaxID.Country)
			assert.Equal(t, tt.taxID, supplier.TaxID.Code)

			require.Len(t, supplier.Inboxes, 1)
			assert.Equal(t, tt.inboxScheme, supplier.Inboxes[0].Scheme)
			assert.Equal(t, tt.inboxCode, supplier.Inboxes[0].Code)

			assert.Equal(t, tt.groupSIREN, identityWithScheme(supplier, "0231"))

			var rep *org.Party
			if inv.Ordering != nil {
				rep = inv.Ordering.Seller
			}
			if tt.repName == "" {
				assert.Nil(t, rep)
				return
			}
			require.NotNil(t, rep)
			assert.Equal(t, tt.repName, rep.Name)
			require.NotNil(t, rep.TaxID)
			assert.Equal(t, tt.repTaxID, rep.TaxID.Code)
		})
	}
}

// identityWithScheme returns the code of the party identity issued under the
// given ISO 6523 scheme, or an empty code when there is none.
func identityWithScheme(party *org.Party, scheme cbc.Code) cbc.Code {
	for _, id := range party.Identities {
		if id.Ext.Get(iso.ExtKeySchemeID) == scheme {
			return id.Code
		}
	}
	return cbc.CodeEmpty
}

// TestNewPartyTaxRegistration pins BT-32: the tax scheme code is the French
// one under a French context, and the identity's own type elsewhere.
func TestNewPartyTaxRegistration(t *testing.T) {
	convert := func(t *testing.T, fixture string, id *org.Identity, opts ...ubl.Option) []PartyTaxSchemeView {
		t.Helper()
		env := loadTestEnvelope(t, fixture)
		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		inv.Supplier.Identities = []*org.Identity{id}
		require.NoError(t, env.Calculate())

		doc, err := ubl.ConvertInvoice(env, opts...)
		require.NoError(t, err)

		out := make([]PartyTaxSchemeView, 0)
		for _, pts := range doc.AccountingSupplierParty.Party.PartyTaxScheme {
			out = append(out, PartyTaxSchemeView{
				Scheme: pts.TaxScheme.ID.Value,
				Code:   pts.CompanyID.Value,
			})
		}
		return out
	}

	t.Run("french context pins the scheme", func(t *testing.T) {
		schemes := convert(t, "france-cius/invoice-fr-cius.json",
			&org.Identity{Scope: org.IdentityScopeTax, Code: "483671517"},
			ubl.WithContext(frubl.ContextCIUS))

		require.NotEmpty(t, schemes)
		last := schemes[len(schemes)-1]
		assert.Equal(t, "LOC", last.Scheme)
		assert.Equal(t, "483671517", last.Code)
	})
}

// PartyTaxSchemeView flattens a PartyTaxScheme for the assertions above.
type PartyTaxSchemeView struct {
	Scheme string
	Code   string
}

func TestConvertAddsRequiredAddons(t *testing.T) {
	t.Run("injects missing addon from context", func(t *testing.T) {
		// Load a France CTC-shaped invoice, strip the ctc addon, and verify
		// that Convert injects it back in before producing the UBL document.
		env := loadTestEnvelope(t, "france-cius/invoice-fr-cius.json")

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)

		// Drop the ctc addon; keep the en16931 one. SetAddons replaces the list.
		inv.SetAddons(en16931.V2017)
		require.NotContains(t, inv.GetAddons(), flow2.V1,
			"precondition: ctc addon must be absent before Convert runs")

		_, err := ubl.Convert(env, ubl.WithContext(frubl.ContextCIUS))
		require.NoError(t, err)

		// After Convert the addon should have been appended in-place.
		assert.Contains(t, inv.GetAddons(), flow2.V1)
		// And the pre-existing addon must be preserved.
		assert.Contains(t, inv.GetAddons(), en16931.V2017)
	})
}

func TestConvertSurfacesValidationFaultsAfterAutoAddon(t *testing.T) {
	// When Convert auto-injects a stricter addon, the resulting validation
	// failure must be surfaced as a *gobl.Error whose cause is rules.Faults,
	// so consumers can render the []*rules.Fault list (code, paths, message)
	// instead of a flattened string.

	// Minimal DE invoice doesn't satisfy the France CTC rule set. Convert
	// with the France CIUS context to force ensureAddons to add flow2.V1
	// and then fail validation.
	env := loadTestEnvelope(t, "invoice-minimal.json")

	_, err := ubl.Convert(env, ubl.WithContext(frubl.ContextCIUS))
	require.Error(t, err)

	// Must be the GOBL validation error — not wrapped in anything ubl-specific.
	assert.ErrorIs(t, err, gobl.ErrValidation)

	var ge *gobl.Error
	require.ErrorAs(t, err, &ge, "error must be a *gobl.Error so faults survive")

	faults := ge.Faults()
	require.NotNil(t, faults, "cause must be rules.Faults, not a plain error")
	require.Greater(t, faults.Len(), 0)

	// Faults().List() returns []*rules.Fault — each fault keeps its
	// structured code, paths, and message so it can be rendered by a client.
	list := faults.List()
	assert.IsType(t, []*rules.Fault{}, list)
	require.NotEmpty(t, list)

	first := list[0]
	assert.NotEmpty(t, first.Code(), "fault must carry a rule code")
	assert.NotEmpty(t, first.Message(), "fault must carry a message")
	assert.NotEmpty(t, first.Paths(), "fault must carry at least one JSON path")

	// The France CTC addon's "supplier endpoint is required for French B2B
	// invoices" rule must be among the reported faults. (The billing-mode rule
	// is now auto-satisfied by the addon's normalization, so a structural rule
	// the minimal invoice cannot satisfy is used instead.)
	assert.True(t, faults.HasCode("GOBL-FR-CTC-FLOW2-BILL-INVOICE-44"),
		"expected supplier-endpoint-required fault; got: %s", err)
}
