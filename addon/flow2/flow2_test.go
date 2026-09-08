package flow2

import (
	"strings"
	"testing"

	"github.com/invopop/gobl.fr.ctc/addon/dgfip"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/norm"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/regimes/fr"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// frPartyWithSIREN returns a French party with a SIREN identity.
func frPartyWithSIREN(name, taxCode, siren string) *org.Party {
	return &org.Party{
		Name: name,
		TaxID: &tax.Identity{
			Country: "FR",
			Code:    cbc.Code(taxCode),
		},
		Identities: []*org.Identity{
			{
				Type:  fr.IdentityTypeSIREN,
				Code:  cbc.Code(siren),
				Scope: org.IdentityScopeLegal,
				Ext: tax.ExtensionsOf(cbc.CodeMap{
					iso.ExtKeySchemeID: identitySchemeIDSIREN,
				}),
			},
		},
		Addresses: []*org.Address{
			{
				Street:   "1 Rue",
				Code:     "75001",
				Locality: "Paris",
				Country:  "FR",
			},
		},
		Inboxes: []*org.Inbox{
			{
				Key:    org.InboxKeyPeppol,
				Scheme: cbc.Code("0225"),
				Code:   cbc.Code(siren),
			},
		},
	}
}

func testInvoiceB2BStandard(t *testing.T) *bill.Invoice {
	t.Helper()
	return &bill.Invoice{
		Regime:   tax.WithRegime("FR"),
		Addons:   tax.WithAddons(V1, en16931.V2017),
		Code:     "FAC-2024-001",
		Currency: "EUR",
		Type:     bill.InvoiceTypeStandard,
		Tax: &bill.Tax{
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				dgfip.ExtKeyBillingMode:   dgfip.BillingModeS1,
				untdid.ExtKeyDocumentType: "380",
			}),
		},
		Supplier:  frPartyWithSIREN("Supplier SARL", "39356000000", "356000000"),
		Customer:  frPartyWithSIREN("Customer SAS", "44732829320", "732829320"),
		IssueDate: cal.MakeDate(2024, 6, 13),
		Lines: []*bill.Line{
			{
				Quantity: num.MakeAmount(10, 0),
				Item: &org.Item{
					Name:  "Service",
					Price: num.NewAmount(10000, 2),
				},
				Taxes: tax.Set{
					{Category: "VAT", Rate: "standard"},
				},
			},
		},
		Payment: &bill.PaymentDetails{
			Terms: &pay.Terms{
				Key: pay.TermKeyDueDate,
				DueDates: []*pay.DueDate{
					{
						Date:    cal.NewDate(2024, 7, 13),
						Percent: num.NewPercentage(100, 3),
					},
				},
			},
			Instructions: &pay.Instructions{
				Key: pay.MeansKeyCreditTransfer,
				CreditTransfer: []*pay.CreditTransfer{
					{
						IBAN: "FR7630006000011234567890189",
						Name: "Supplier SARL",
					},
				},
			},
		},
		Notes: []*org.Note{
			{
				Key:  org.NoteKeyPayment,
				Text: "Conditions.",
				Ext:  tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTextSubject: "PMT"}),
			},
			{
				Key:  org.NoteKeyPaymentMethod,
				Text: "Penalties.",
				Ext:  tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTextSubject: "PMD"}),
			},
			{
				Key:  org.NoteKeyPaymentTerm,
				Text: "No early discount.",
				Ext:  tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTextSubject: "AAB"}),
			},
		},
	}
}

func TestInvoiceB2BHappyPath(t *testing.T) {
	inv := testInvoiceB2BStandard(t)
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
}

// A French invoice whose parties carry only the canonical endpoint and no
// legacy inbox — e.g. one parsed from UBL/CII — satisfies the electronic
// address rules (BR-FR-13/21/22), which are bound to BT-34 / BT-49.
func TestInvoiceB2BEndpointOnlyParties(t *testing.T) {
	inv := testInvoiceB2BStandard(t)
	for _, p := range []*org.Party{inv.Supplier, inv.Customer} {
		siren := p.Inboxes[0].Code.String()
		p.Inboxes = nil
		p.Endpoints = []*org.Endpoint{
			{URI: cbc.URI("iso6523-actorid-upis::0225:" + siren)},
		}
	}
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
}

// A party expressed the older way, with a Peppol inbox and no endpoint, is
// migrated forward so the same rules pass. en16931 cannot do this migration
// here: it normalizes before this addon, and the peppol key it looks for is
// only assigned by normalizeInboxes.
func TestInvoiceB2BInboxOnlyPartiesMigrateToEndpoint(t *testing.T) {
	inv := testInvoiceB2BStandard(t)
	for _, p := range []*org.Party{inv.Supplier, inv.Customer} {
		p.Endpoints = nil
		p.Inboxes = []*org.Inbox{
			{Scheme: inboxSchemeSIREN, Code: p.Inboxes[0].Code}, // no peppol key
		}
	}
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
	require.Len(t, inv.Supplier.Endpoints, 1)
	assert.Equal(t, cbc.URI("iso6523-actorid-upis::0225:356000000"), inv.Supplier.Endpoints[0].URI)
}

// BR-FR-21 constrains the buyer's electronic address (BT-49) on a normal B2B
// invoice; BR-FR-22 constrains the seller's (BT-34) when the document is
// self-billed. Which party carries the SIREN-matching endpoint therefore
// swaps with the document type.
func TestInvoiceSIRENEndpointFollowsDocumentType(t *testing.T) {
	// An endpoint that is present and well-formed, but whose code does not
	// start with the party's SIREN.
	mismatch := func(p *org.Party) {
		p.Inboxes = nil
		p.Endpoints = []*org.Endpoint{{URI: "iso6523-actorid-upis::0225:999999999"}}
	}
	// The self-billed tag drives the scenario that sets document type 389;
	// Calculate re-derives the ext, so setting it directly would not survive.
	selfBilled := func(inv *bill.Invoice) {
		inv.Tags = tax.WithTags(tax.TagSelfBilled)
		inv.Tax.Ext = tax.ExtensionsOf(cbc.CodeMap{
			dgfip.ExtKeyBillingMode: dgfip.BillingModeS1,
		})
	}

	t.Run("standard invoice checks the customer (BR-FR-21)", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		mismatch(inv.Customer)
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "customer must have a Peppol endpoint")
	})

	t.Run("standard invoice leaves the supplier's SIREN unchecked", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		mismatch(inv.Supplier)
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("self-billed invoice checks the supplier (BR-FR-22)", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		selfBilled(inv)
		mismatch(inv.Supplier)
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "supplier must have a Peppol endpoint")
	})

	t.Run("self-billed invoice leaves the customer's SIREN unchecked", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		selfBilled(inv)
		mismatch(inv.Customer)
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})
}

// The electronic address format rules reach endpoint-only parties: BR-FR-23
// constrains the charset of a 0225 address, BR-FR-25 its length.
func TestInvoiceEndpointAddressFormat(t *testing.T) {
	t.Run("charset (BR-FR-23)", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		inv.Supplier.Inboxes = nil
		inv.Supplier.Endpoints = []*org.Endpoint{{URI: "iso6523-actorid-upis::0225:356000000/x"}}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "BR-FR-23")
	})
	t.Run("length (BR-FR-25)", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		inv.Supplier.Inboxes = nil
		inv.Supplier.Endpoints = []*org.Endpoint{
			{URI: cbc.URI("iso6523-actorid-upis::0225:356000000" + strings.Repeat("A", 120))},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "BR-FR-25")
	})
}

func TestInvoiceCodeFormatRejectsBadChars(t *testing.T) {
	inv := testInvoiceB2BStandard(t)
	inv.Code = "INVALID CODE WITH SPACE"
	assert.Error(t, rules.Validate(inv))
}

func TestInvoiceMissingNotesFails(t *testing.T) {
	inv := testInvoiceB2BStandard(t)
	inv.Notes = nil
	assert.Error(t, rules.Validate(inv))
}

func TestInvoiceMissingBillingModeFails(t *testing.T) {
	inv := testInvoiceB2BStandard(t)
	inv.Tax.Ext = inv.Tax.Ext.Delete(dgfip.ExtKeyBillingMode)
	assert.Error(t, rules.Validate(inv))
}

func TestInvoiceInvalidBillingModeFails(t *testing.T) {
	// GOBL does not enforce an extension's code list automatically; rule 09 must
	// reject a value that is not a DGFiP billing-mode code (e.g. the "b2b" that
	// reached PPF as an invalid BT-23 ProfileID).
	inv := testInvoiceB2BStandard(t)
	require.NoError(t, inv.Calculate())
	inv.Tax.Ext = inv.Tax.Ext.Merge(tax.ExtensionsOf(cbc.CodeMap{dgfip.ExtKeyBillingMode: "b2b"}))
	assert.ErrorContains(t, rules.Validate(inv), "must be a valid billing-mode code")
}

func TestNormalizeAddsRequiredNotes(t *testing.T) {
	inv := testInvoiceB2BStandard(t)
	inv.Notes = nil
	norm.Normalize(inv, tax.AddonContext(V1))
	assert.GreaterOrEqual(t, len(inv.Notes), 3)
}

func TestInvoiceAttachmentDescription(t *testing.T) {
	attachment := func(desc string) *org.Attachment {
		return &org.Attachment{
			Code:        "PJ-001",
			Name:        "facture.pdf",
			Description: desc,
			URL:         "https://example.com/facture.pdf",
		}
	}

	t.Run("accepts a missing description", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		inv.Attachments = []*org.Attachment{attachment("")}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})

	t.Run("accepts an allowed description", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		inv.Attachments = []*org.Attachment{attachment(attachmentFormatLisible)}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})

	t.Run("rejects an unknown description", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		inv.Attachments = []*org.Attachment{attachment("UNEXPECTED")}
		require.NoError(t, inv.Calculate())
		assert.Error(t, rules.Validate(inv))
	})
}

func TestIdentitySIRENIsNineDigits(t *testing.T) {
	stcIdentity := func(code string) *org.Identity {
		return &org.Identity{
			Code: cbc.Code(code),
			Ext:  tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDSTC}),
		}
	}

	t.Run("rejects a SIRET under scheme 0002", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		inv.Supplier.Identities[0].Code = "73282932000074"
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "9 digits")
	})

	t.Run("rejects a SIRET under scheme 0231", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		inv.Customer.Identities = append(inv.Customer.Identities, stcIdentity("73282932000074"))
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "9 digits")
	})

	t.Run("accepts a SIREN under scheme 0231", func(t *testing.T) {
		inv := testInvoiceB2BStandard(t)
		inv.Customer.Identities = append(inv.Customer.Identities, stcIdentity("356000000"))
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})
}
