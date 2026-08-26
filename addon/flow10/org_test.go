package flow10

import (
	"slices"
	"strings"
	"testing"

	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/fr"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func legalIdentity(scheme cbc.Code, code string) *org.Identity {
	return &org.Identity{
		Code:  cbc.Code(code),
		Scope: org.IdentityScopeLegal,
		Ext:   tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: scheme}),
	}
}

func TestNormalizeParty(t *testing.T) {
	t.Run("nil safe", func(t *testing.T) {
		assert.NotPanics(t, func() { normalizeParty(nil) })
	})

	t.Run("French tax ID derives SIREN", func(t *testing.T) {
		p := &org.Party{TaxID: &tax.Identity{Country: "FR", Code: "732829320"}}
		normalizeParty(p)
		require.Len(t, p.Identities, 1)
		assert.Equal(t, cbc.Code(identitySchemeIDSIREN), p.Identities[0].Ext.Get(iso.ExtKeySchemeID))
	})

	t.Run("EU non-France tax ID derives EU VAT identity", func(t *testing.T) {
		p := &org.Party{TaxID: &tax.Identity{Country: "DE", Code: "111111125"}}
		normalizeParty(p)
		require.Len(t, p.Identities, 1)
		assert.Equal(t, cbc.Code(identitySchemeIDEUVAT), p.Identities[0].Ext.Get(iso.ExtKeySchemeID))
		assert.Equal(t, cbc.Code("DE111111125"), p.Identities[0].Code)
	})

	t.Run("empty code is a no-op", func(t *testing.T) {
		p := &org.Party{TaxID: &tax.Identity{Country: "FR"}}
		normalizeParty(p)
		assert.Empty(t, p.Identities)
	})

	t.Run("nil tax ID is a no-op", func(t *testing.T) {
		p := &org.Party{}
		normalizeParty(p)
		assert.Empty(t, p.Identities)
	})

	t.Run("generates SIREN from SIRET", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{
			{Type: fr.IdentityTypeSIRET, Code: "73282932000074"},
		}}
		normalizeParty(p)
		var siren *org.Identity
		for _, id := range p.Identities {
			if id.Type == fr.IdentityTypeSIREN {
				siren = id
			}
		}
		require.NotNil(t, siren)
		assert.Equal(t, cbc.Code("732829320"), siren.Code)
		assert.Equal(t, org.IdentityScopeLegal, siren.Scope)
	})

	t.Run("New Caledonia tax ID derives RIDET identity", func(t *testing.T) {
		p := &org.Party{TaxID: &tax.Identity{Country: "NC", Code: "123456789"}}
		normalizeParty(p)
		require.Len(t, p.Identities, 1)
		assert.Equal(t, cbc.Code(identitySchemeIDRIDET), p.Identities[0].Ext.Get(iso.ExtKeySchemeID))
		assert.Equal(t, cbc.Code("123456789"), p.Identities[0].Code)
		assert.Equal(t, org.IdentityScopeLegal, p.Identities[0].Scope)
	})

	t.Run("French Polynesia tax ID derives TAHITI identity", func(t *testing.T) {
		p := &org.Party{TaxID: &tax.Identity{Country: "PF", Code: "123456789"}}
		normalizeParty(p)
		require.Len(t, p.Identities, 1)
		assert.Equal(t, cbc.Code(identitySchemeIDTAHITI), p.Identities[0].Ext.Get(iso.ExtKeySchemeID))
		assert.Equal(t, cbc.Code("123456789"), p.Identities[0].Code)
	})

	t.Run("non-EU tax ID derives HORS_UE identity from name", func(t *testing.T) {
		p := &org.Party{
			Name:  "Global Trading",
			TaxID: &tax.Identity{Country: "US", Code: "TAX-123"},
		}
		normalizeParty(p)
		require.Len(t, p.Identities, 1)
		assert.Equal(t, cbc.Code(identitySchemeIDNonEU), p.Identities[0].Ext.Get(iso.ExtKeySchemeID))
		assert.Equal(t, cbc.Code("USGLOBALTRADING"), p.Identities[0].Code)
	})

	t.Run("Wallis & Futuna tax ID is treated as HORS_UE", func(t *testing.T) {
		p := &org.Party{
			Name:  "Island Co",
			TaxID: &tax.Identity{Country: "WF", Code: "X"},
		}
		normalizeParty(p)
		require.Len(t, p.Identities, 1)
		assert.Equal(t, cbc.Code(identitySchemeIDNonEU), p.Identities[0].Ext.Get(iso.ExtKeySchemeID))
		assert.Equal(t, cbc.Code("WFISLANDCO"), p.Identities[0].Code)
	})

	t.Run("non-EU tax ID without a name is a no-op", func(t *testing.T) {
		p := &org.Party{TaxID: &tax.Identity{Country: "US", Code: "TAX-123"}}
		normalizeParty(p)
		assert.Empty(t, p.Identities)
	})

	t.Run("non-EU tax ID without a code still derives HORS_UE from name", func(t *testing.T) {
		p := &org.Party{
			Name:  "Global Trading",
			TaxID: &tax.Identity{Country: "US"},
		}
		normalizeParty(p)
		require.Len(t, p.Identities, 1)
		assert.Equal(t, cbc.Code(identitySchemeIDNonEU), p.Identities[0].Ext.Get(iso.ExtKeySchemeID))
		assert.Equal(t, cbc.Code("USGLOBALTRADING"), p.Identities[0].Code)
	})

	t.Run("EU non-France tax ID without a code is a no-op", func(t *testing.T) {
		p := &org.Party{Name: "Muster GmbH", TaxID: &tax.Identity{Country: "DE"}}
		normalizeParty(p)
		assert.Empty(t, p.Identities)
	})

	t.Run("tax ID without a country is a no-op", func(t *testing.T) {
		p := &org.Party{Name: "No Country Co", TaxID: &tax.Identity{Code: "123"}}
		normalizeParty(p)
		assert.Empty(t, p.Identities)
	})

	t.Run("non-FR tax ID with an existing SIREN keeps the SIREN, no HORS_UE added", func(t *testing.T) {
		p := &org.Party{
			Name:       "Monaco Branch",
			TaxID:      &tax.Identity{Country: "MC", Code: "12345678900011"},
			Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "123456789")},
		}
		normalizeParty(p)
		require.Len(t, p.Identities, 1)
		assert.Equal(t, cbc.Code(identitySchemeIDSIREN), p.Identities[0].Ext.Get(iso.ExtKeySchemeID))
	})

	t.Run("normalization is idempotent", func(t *testing.T) {
		parties := []*org.Party{
			{TaxID: &tax.Identity{Country: "FR", Code: "732829320"}},
			{TaxID: &tax.Identity{Country: "DE", Code: "111111125"}},
			{Name: "Global Trading", TaxID: &tax.Identity{Country: "US", Code: "TAX-123"}},
			{
				TaxID:      &tax.Identity{Country: "MC", Code: "12345678900011"},
				Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "123456789")},
			},
		}
		for _, p := range parties {
			normalizeParty(p)
			want := slices.Clone(p.Identities)
			normalizeParty(p)
			assert.Equal(t, want, p.Identities)
		}
	})
}

func TestNonEUIdentityCode(t *testing.T) {
	// missing country or name → empty (no identity built)
	assert.Equal(t, "", nonEUIdentityCode("", "Some Name"))
	assert.Equal(t, "", nonEUIdentityCode("US", ""))
	assert.Equal(t, "", nonEUIdentityCode("US", "!!! ---"))
	// country code + cleaned, uppercased name
	assert.Equal(t, "USGLOBALTRADING", nonEUIdentityCode("US", "Global Trading"))
	// uppercased alphanumeric: digits kept, spaces/punctuation dropped
	assert.Equal(t, "USABC123DEF", nonEUIdentityCode("US", "abc 123-def!"))
	// name truncated to 16 characters (18 total with country code)
	got := nonEUIdentityCode("US", "AbcdefghijklmnopqrsTuv")
	assert.Equal(t, "USABCDEFGHIJKLMNOP", got)
	assert.Len(t, got, 18)
}

func TestSirenFromFrenchTaxID(t *testing.T) {
	p := &org.Party{Identities: []*org.Identity{{Type: fr.IdentityTypeSIRET, Code: "73282932000074"}}}
	assert.Equal(t, "732829320", sirenFromFrenchTaxID("x", p))
	assert.Equal(t, "732829320", sirenFromFrenchTaxID("FR44732829320", &org.Party{}))
	assert.Equal(t, "12", sirenFromFrenchTaxID("FR12", &org.Party{}))
}

func TestEnsureIdentity(t *testing.T) {
	t.Run("empty code no-op", func(t *testing.T) {
		p := &org.Party{}
		ensureIdentity(p, fr.IdentityTypeSIREN, "", identitySchemeIDSIREN)
		assert.Empty(t, p.Identities)
	})
	t.Run("skips when scheme present", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "1")}}
		ensureIdentity(p, fr.IdentityTypeSIREN, "2", identitySchemeIDSIREN)
		assert.Len(t, p.Identities, 1)
	})
}

func TestIsEUNonFrance(t *testing.T) {
	assert.False(t, isEUNonFrance("FR"))
	assert.False(t, isEUNonFrance(""))
	assert.True(t, isEUNonFrance("DE"))
	assert.False(t, isEUNonFrance("US"))
}

func TestPartyLegalSchemeID(t *testing.T) {
	assert.Equal(t, "", partyLegalSchemeID(nil))
	assert.Equal(t, "", partyLegalSchemeID(&org.Party{}))

	t.Run("legal-scope identity wins", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "1")}}
		assert.Equal(t, identitySchemeIDSIREN, partyLegalSchemeID(p))
	})
	t.Run("falls back to allowed scheme without legal scope", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{
			{Code: "1", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDNonEU})},
		}}
		assert.Equal(t, identitySchemeIDNonEU, partyLegalSchemeID(p))
	})
	t.Run("ignores identities with no scheme", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{{Code: "1"}}}
		assert.Equal(t, "", partyLegalSchemeID(p))
	})
}

func TestPartyHasSIREN(t *testing.T) {
	assert.False(t, partyHasSIREN("wrong-type"))
	assert.False(t, partyHasSIREN((*org.Party)(nil)))
	assert.False(t, partyHasSIREN(&org.Party{}))
	assert.True(t, partyHasSIREN(&org.Party{Identities: []*org.Identity{{Type: fr.IdentityTypeSIREN, Code: "1"}}}))
	assert.True(t, partyHasSIREN(&org.Party{Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "1")}}))
	assert.False(t, partyHasSIREN(&org.Party{Identities: []*org.Identity{nil, legalIdentity(identitySchemeIDNonEU, "1")}}))
}

func TestPartyHasAllowedLegalScheme(t *testing.T) {
	assert.False(t, partyHasAllowedLegalScheme("wrong-type"))
	assert.False(t, partyHasAllowedLegalScheme((*org.Party)(nil)))
	assert.True(t, partyHasAllowedLegalScheme(&org.Party{Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "1")}}))
	assert.False(t, partyHasAllowedLegalScheme(&org.Party{Identities: []*org.Identity{legalIdentity("9999", "1")}}))
}

func TestPartyHasTaxIDWhenRequired(t *testing.T) {
	assert.True(t, partyHasTaxIDWhenRequired("wrong-type"))
	assert.True(t, partyHasTaxIDWhenRequired((*org.Party)(nil)))

	t.Run("non-VAT-requiring scheme passes without tax ID", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{legalIdentity(identitySchemeIDNonEU, "1")}}
		assert.True(t, partyHasTaxIDWhenRequired(p))
	})
	t.Run("SIREN scheme requires tax ID", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "1")}}
		assert.False(t, partyHasTaxIDWhenRequired(p))
		p.TaxID = &tax.Identity{Country: "FR", Code: "44732829320"}
		assert.True(t, partyHasTaxIDWhenRequired(p))
	})
}

func TestIdentitiesSchemesUnique(t *testing.T) {
	assert.True(t, identitiesSchemesUnique("wrong-type"))
	assert.True(t, identitiesSchemesUnique([]*org.Identity{}))
	// nil and empty-ext entries skipped
	assert.True(t, identitiesSchemesUnique([]*org.Identity{nil, {Code: "x"}}))
	unique := []*org.Identity{legalIdentity(identitySchemeIDSIREN, "1"), legalIdentity(identitySchemeIDNonEU, "2")}
	assert.True(t, identitiesSchemesUnique(unique))
	dup := []*org.Identity{legalIdentity(identitySchemeIDSIREN, "1"), legalIdentity(identitySchemeIDSIREN, "2")}
	assert.False(t, identitiesSchemesUnique(dup))
}

func TestIdentitiesSingleLegalScope(t *testing.T) {
	assert.True(t, identitiesSingleLegalScope("wrong-type"))
	assert.True(t, identitiesSingleLegalScope([]*org.Identity{}))
	// nil entries and non-legal identities are ignored
	assert.True(t, identitiesSingleLegalScope([]*org.Identity{nil, {Code: "x"}}))
	// exactly one legal identity is fine
	assert.True(t, identitiesSingleLegalScope([]*org.Identity{
		legalIdentity(identitySchemeIDSIREN, "1"),
		{Code: "2", Scope: org.IdentityScopeTax},
	}))
	// two legal identities are rejected
	assert.False(t, identitiesSingleLegalScope([]*org.Identity{
		legalIdentity(identitySchemeIDSIREN, "1"),
		legalIdentity(identitySchemeIDEUVAT, "2"),
	}))
}

func TestIdentitySchemeIs(t *testing.T) {
	fn := identitySchemeIs(identitySchemeIDNonEU)
	assert.False(t, fn("wrong-type"))
	assert.False(t, fn((*org.Identity)(nil)))
	assert.False(t, fn(&org.Identity{Code: "x"}))
	assert.True(t, fn(&org.Identity{
		Code: "x", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDNonEU}),
	}))
}

func TestOrgIdentityValidate(t *testing.T) {
	ctx := tax.AddonContext(V1)
	ident := func(scheme, code string) *org.Identity {
		return &org.Identity{
			Code: cbc.Code(code),
			Ext:  tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: cbc.Code(scheme)}),
		}
	}

	t.Run("valid HORS_UE", func(t *testing.T) {
		assert.NoError(t, rules.Validate(ident(identitySchemeIDNonEU, "USGLOBALTRADING"), ctx))
	})
	t.Run("HORS_UE too long", func(t *testing.T) {
		err := rules.Validate(ident(identitySchemeIDNonEU, strings.Repeat("A", 19)), ctx)
		assert.ErrorContains(t, err, "18 characters")
	})
	t.Run("HORS_UE not uppercase alphanumeric", func(t *testing.T) {
		err := rules.Validate(ident(identitySchemeIDNonEU, "us-abc"), ctx)
		assert.ErrorContains(t, err, "uppercase alphanumeric")
	})

	t.Run("valid RIDET (9 and 10)", func(t *testing.T) {
		assert.NoError(t, rules.Validate(ident(identitySchemeIDRIDET, "123456789"), ctx))
		assert.NoError(t, rules.Validate(ident(identitySchemeIDRIDET, "1234567890"), ctx))
	})
	t.Run("RIDET wrong length", func(t *testing.T) {
		err := rules.Validate(ident(identitySchemeIDRIDET, "12345"), ctx)
		assert.ErrorContains(t, err, "9 or 10 characters")
	})

	t.Run("valid TAHITI", func(t *testing.T) {
		assert.NoError(t, rules.Validate(ident(identitySchemeIDTAHITI, "123456789"), ctx))
	})
	t.Run("TAHITI wrong length", func(t *testing.T) {
		err := rules.Validate(ident(identitySchemeIDTAHITI, "12345678"), ctx)
		assert.ErrorContains(t, err, "9 characters")
	})
}

func TestOrgPartyValidate(t *testing.T) {
	ctx := tax.AddonContext(V1)

	t.Run("SIREN with a VAT number passes", func(t *testing.T) {
		p := &org.Party{
			Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "1")},
			TaxID:      &tax.Identity{Country: "FR", Code: "44732829320"},
		}
		assert.NoError(t, rules.Validate(p, ctx))
	})
	t.Run("SIREN without a TaxID fails", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "1")}}
		assert.ErrorContains(t, rules.Validate(p, ctx), "VAT number")
	})
	t.Run("SIREN with an empty TaxID code fails", func(t *testing.T) {
		p := &org.Party{
			Identities: []*org.Identity{legalIdentity(identitySchemeIDSIREN, "1")},
			TaxID:      &tax.Identity{Country: "FR"},
		}
		assert.ErrorContains(t, rules.Validate(p, ctx), "VAT number")
	})
	t.Run("EU VAT without a VAT number fails", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{legalIdentity(identitySchemeIDEUVAT, "DE111111125")}}
		assert.ErrorContains(t, rules.Validate(p, ctx), "VAT number")
	})
	t.Run("HORS_UE without a VAT number is unaffected", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{legalIdentity(identitySchemeIDNonEU, "USGLOBALTRADING")}}
		assert.NoError(t, rules.Validate(p, ctx))
	})
}

func TestNormalizeIdentityFlow10(t *testing.T) {
	assert.NotPanics(t, func() { normalizeIdentity(nil) })
	siren := &org.Identity{Type: fr.IdentityTypeSIREN, Code: "1"}
	normalizeIdentity(siren)
	assert.Equal(t, cbc.Code(identitySchemeIDSIREN), siren.Ext.Get(iso.ExtKeySchemeID))
	siret := &org.Identity{Type: fr.IdentityTypeSIRET, Code: "1"}
	normalizeIdentity(siret)
	assert.Equal(t, cbc.Code(identitySchemeIDSIRET), siret.Ext.Get(iso.ExtKeySchemeID))
}
