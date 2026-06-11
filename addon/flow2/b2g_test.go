package flow2

import (
	"strings"
	"testing"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/invopop/gobl.fr.ctc/addon/dgfip"
)

// addSIRET appends a SIRET (scheme 0009) identity to the party,
// satisfying BR-FR-CPRO-03/09/10 on the B2G happy path.
func addSIRET(p *org.Party, siret string) {
	p.Identities = append(p.Identities, &org.Identity{
		Code: cbc.Code(siret),
		Ext: tax.ExtensionsOf(cbc.CodeMap{
			iso.ExtKeySchemeID: identitySchemeIDSIRET,
		}),
	})
}

// testInvoiceB2G builds a B2G-tagged invoice satisfying the BR-FR-CPRO
// rule set.
func testInvoiceB2G(t *testing.T) *bill.Invoice {
	t.Helper()
	inv := testInvoiceB2BStandard(t)
	inv.SetTags(TagB2G)
	addSIRET(inv.Supplier, "35600000000048")
	addSIRET(inv.Customer, "73282932000074")
	return inv
}

func TestInvoiceB2GHappyPath(t *testing.T) {
	inv := testInvoiceB2G(t)
	require.NoError(t, inv.Calculate())
	require.NoError(t, rules.Validate(inv))
}

func TestInvoiceB2GRulesGatedOnTag(t *testing.T) {
	// Without the b2g tag, CPRO violations must not fire.
	inv := testInvoiceB2BStandard(t)
	inv.Code = cbc.Code(strings.Repeat("F", 30)) // >20, under the 35-char core limit
	require.NoError(t, inv.Calculate())
	assert.NoError(t, rules.Validate(inv))
}

func TestInvoiceB2GViolations(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(inv *bill.Invoice)
		errLike string
	}{
		{
			name:    "cpro-02 invoice number over 20 chars",
			mutate:  func(inv *bill.Invoice) { inv.Code = cbc.Code(strings.Repeat("F", 21)) },
			errLike: "BR-FR-CPRO-02",
		},
		{
			name: "cpro-02 preceding number over 20 chars",
			mutate: func(inv *bill.Invoice) {
				inv.Preceding = []*org.DocumentRef{{Code: cbc.Code(strings.Repeat("P", 21))}}
			},
			errLike: "BR-FR-CPRO-02",
		},
		{
			name: "cpro-03 supplier without an admissible private identity",
			mutate: func(inv *bill.Invoice) {
				inv.Supplier.Identities = inv.Supplier.Identities[:1] // keep SIREN only
			},
			errLike: "BR-FR-CPRO-03",
		},
		{
			name: "cpro-04 EU VAT identity too long",
			mutate: func(inv *bill.Invoice) {
				inv.Supplier.Identities = append(inv.Supplier.Identities, &org.Identity{
					Code: cbc.Code(strings.Repeat("1", 18)),
					Ext:  tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: "0223"}),
				})
			},
			errLike: "BR-FR-CPRO-04",
		},
		{
			name: "cpro-06 RIDET identity outside 9-10 chars",
			mutate: func(inv *bill.Invoice) {
				inv.Supplier.Identities = append(inv.Supplier.Identities, &org.Identity{
					Code: "1234",
					Ext:  tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: "0228"}),
				})
			},
			errLike: "BR-FR-CPRO-04",
		},
		{
			name: "cpro-10 customer without SIRET",
			mutate: func(inv *bill.Invoice) {
				inv.Customer.Identities = inv.Customer.Identities[:1] // keep SIREN only
			},
			errLike: "BR-FR-CPRO-10",
		},
		{
			name: "cpro-20 multiple preceding references",
			mutate: func(inv *bill.Invoice) {
				inv.Preceding = []*org.DocumentRef{{Code: "A"}, {Code: "B"}}
			},
			errLike: "BR-FR-CPRO-20",
		},
		{
			name: "cpro-24 billing mode S5 forbidden",
			mutate: func(inv *bill.Invoice) {
				inv.Tax.Ext = inv.Tax.Ext.Set(dgfip.ExtKeyBillingMode, dgfip.BillingModeS5)
			},
			errLike: "BR-FR-CPRO-24",
		},
		{
			name: "cpro-26 two supplier contacts",
			mutate: func(inv *bill.Invoice) {
				inv.Supplier.People = []*org.Person{
					{Name: &org.Name{Given: "A"}},
					{Name: &org.Name{Given: "B"}},
				}
			},
			errLike: "BR-FR-CPRO-26",
		},
		{
			name: "cpro-39 payment account id over 27 chars",
			mutate: func(inv *bill.Invoice) {
				inv.Payment.Instructions.CreditTransfer[0].IBAN = strings.Repeat("F", 28)
			},
			errLike: "BR-FR-CPRO-39",
		},
		{
			name: "cpro-42 note over 1024 chars",
			mutate: func(inv *bill.Invoice) {
				inv.Notes[0].Text = strings.Repeat("x", 1025)
			},
			errLike: "BR-FR-CPRO-42",
		},
		{
			name: "cpro-44 customer name over 99 chars",
			mutate: func(inv *bill.Invoice) {
				inv.Customer.Name = strings.Repeat("C", 100)
			},
			errLike: "BR-FR-CPRO-44",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := testInvoiceB2G(t)
			tc.mutate(inv)
			require.NoError(t, inv.Calculate())
			err := rules.Validate(inv)
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.errLike)
		})
	}
}

func TestInvoiceB2GBillingModeS3Allowed(t *testing.T) {
	inv := testInvoiceB2G(t)
	inv.Tax.Ext = inv.Tax.Ext.Set(dgfip.ExtKeyBillingMode, dgfip.BillingModeS3)
	require.NoError(t, inv.Calculate())
	assert.NoError(t, rules.Validate(inv))
}
