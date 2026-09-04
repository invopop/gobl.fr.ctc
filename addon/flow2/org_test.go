package flow2

import (
	"strings"
	"testing"

	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/fr"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sirenIdentity(code string) *org.Identity {
	return &org.Identity{
		Type:  fr.IdentityTypeSIREN,
		Code:  cbc.Code(code),
		Scope: org.IdentityScopeLegal,
		Ext:   tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDSIREN}),
	}
}

// unscopedSIREN returns a SIREN identity that is not the party's legal one.
func unscopedSIREN(code string) *org.Identity {
	id := sirenIdentity(code)
	id.Scope = cbc.KeyEmpty
	return id
}

func TestNormalizeParty(t *testing.T) {
	t.Run("nil safe", func(t *testing.T) {
		assert.NotPanics(t, func() { normalizeParty(nil) })
	})

	t.Run("derives SIREN from French tax ID", func(t *testing.T) {
		p := &org.Party{TaxID: &tax.Identity{Country: "FR", Code: "732829320"}}
		normalizeParty(p)
		require.Len(t, p.Identities, 1)
		assert.Equal(t, fr.IdentityTypeSIREN, p.Identities[0].Type)
		assert.Equal(t, cbc.Code("732829320"), p.Identities[0].Code)
		assert.Equal(t, org.IdentityScopeLegal, p.Identities[0].Scope)
	})

	t.Run("non-French tax ID leaves identities untouched", func(t *testing.T) {
		p := &org.Party{TaxID: &tax.Identity{Country: "ES", Code: "B98602642"}}
		normalizeParty(p)
		assert.Empty(t, p.Identities)
	})

	t.Run("empty tax ID code is a no-op", func(t *testing.T) {
		p := &org.Party{TaxID: &tax.Identity{Country: "FR"}}
		normalizeParty(p)
		assert.Empty(t, p.Identities)
	})

	t.Run("derives SIREN prefix from SIRET", func(t *testing.T) {
		p := &org.Party{
			TaxID: &tax.Identity{Country: "FR", Code: "FR12"},
			Identities: []*org.Identity{
				{Type: fr.IdentityTypeSIRET, Code: "73282932000074"},
			},
		}
		normalizeParty(p)
		// SIRET gets the 0009 scheme; a SIREN (first 9 digits) is generated.
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

	t.Run("tags private-id identity with scheme 0224", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{
			{Key: identityKeyPrivateID, Code: "ABC123"},
		}}
		normalizeParty(p)
		assert.Equal(t, identitySchemeIDPrivate, p.Identities[0].Ext.Get(iso.ExtKeySchemeID))
	})

	t.Run("flags SIREN-scope inbox as peppol", func(t *testing.T) {
		p := &org.Party{Inboxes: []*org.Inbox{
			{Scheme: inboxSchemeSIREN, Code: "732829320_PEP"},
		}}
		normalizeParty(p)
		assert.Equal(t, org.InboxKeyPeppol, p.Inboxes[0].Key)
	})

	t.Run("does not override existing peppol inbox", func(t *testing.T) {
		p := &org.Party{Inboxes: []*org.Inbox{
			{Key: org.InboxKeyPeppol, Scheme: "9999", Code: "X"},
			{Scheme: inboxSchemeSIREN, Code: "Y"},
		}}
		normalizeParty(p)
		assert.Equal(t, cbc.Key(""), p.Inboxes[1].Key)
	})

	t.Run("migrates a peppol inbox to the canonical endpoint", func(t *testing.T) {
		p := &org.Party{Inboxes: []*org.Inbox{
			{Key: org.InboxKeyPeppol, Scheme: inboxSchemeSIREN, Code: "732829320_PEP"},
		}}
		normalizeParty(p)
		require.Len(t, p.Endpoints, 1)
		assert.Equal(t, cbc.URI("iso6523-actorid-upis::0225:732829320_PEP"), p.Endpoints[0].URI)
	})

	t.Run("migrates a 0225 inbox that arrives without the peppol key", func(t *testing.T) {
		// normalizeInboxes assigns the key, which is why this addon has to do
		// the migration itself — en16931 has already normalized by then.
		p := &org.Party{Inboxes: []*org.Inbox{
			{Scheme: inboxSchemeSIREN, Code: "732829320_PEP"},
		}}
		normalizeParty(p)
		require.Len(t, p.Endpoints, 1)
		assert.Equal(t, cbc.URI("iso6523-actorid-upis::0225:732829320_PEP"), p.Endpoints[0].URI)
	})

	t.Run("does not duplicate an existing peppol endpoint", func(t *testing.T) {
		p := &org.Party{
			Inboxes:   []*org.Inbox{{Key: org.InboxKeyPeppol, Scheme: inboxSchemeSIREN, Code: "732829320_PEP"}},
			Endpoints: []*org.Endpoint{{URI: "iso6523-actorid-upis::0225:keep-me"}},
		}
		normalizeParty(p)
		require.Len(t, p.Endpoints, 1)
		assert.Equal(t, cbc.URI("iso6523-actorid-upis::0225:keep-me"), p.Endpoints[0].URI)
	})

	t.Run("leaves a party with no inbox alone", func(t *testing.T) {
		p := &org.Party{Endpoints: []*org.Endpoint{{URI: "mailto:billing@example.com"}}}
		normalizeParty(p)
		require.Len(t, p.Endpoints, 1)
		assert.Equal(t, cbc.URI("mailto:billing@example.com"), p.Endpoints[0].URI)
	})

	t.Run("only the first SIREN keeps the legal scope", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{
			{Type: fr.IdentityTypeSIREN, Code: "732829320", Scope: org.IdentityScopeLegal},
			{Type: fr.IdentityTypeSIREN, Code: "732829320"},
		}}
		normalizeParty(p)
		require.Len(t, p.Identities, 2)
		assert.Equal(t, org.IdentityScopeLegal, p.Identities[0].Scope)
		assert.Equal(t, cbc.KeyEmpty, p.Identities[1].Scope)
	})

	t.Run("legal scope follows the SIREN that already carries it", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{
			{Type: fr.IdentityTypeSIREN, Code: "732829320"},
			{Type: fr.IdentityTypeSIREN, Code: "732829320", Scope: org.IdentityScopeLegal},
		}}
		normalizeParty(p)
		assert.Equal(t, cbc.KeyEmpty, p.Identities[0].Scope)
		assert.Equal(t, org.IdentityScopeLegal, p.Identities[1].Scope)
	})

	t.Run("does not duplicate a SIREN that lacks the ISO scheme", func(t *testing.T) {
		p := &org.Party{
			TaxID:      &tax.Identity{Country: "FR", Code: "44732829320"},
			Identities: []*org.Identity{{Type: fr.IdentityTypeSIREN, Code: "732829320"}},
		}
		normalizeParty(p)
		require.Len(t, p.Identities, 1)
		assert.Equal(t, identitySchemeIDSIREN, p.Identities[0].Ext.Get(iso.ExtKeySchemeID))
		assert.Equal(t, org.IdentityScopeLegal, p.Identities[0].Scope)
	})

	t.Run("SIREN always gets legal scope even when another identity has it", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{
			{Type: fr.IdentityTypeSIREN, Code: "732829320"},
			{Key: identityKeyPrivateID, Code: "ABC123", Scope: org.IdentityScopeLegal},
		}}
		normalizeParty(p)
		var siren *org.Identity
		for _, id := range p.Identities {
			if id.Type == fr.IdentityTypeSIREN {
				siren = id
			}
		}
		require.NotNil(t, siren)
		assert.Equal(t, org.IdentityScopeLegal, siren.Scope)
	})
}

func TestSirenFromFrenchTaxID(t *testing.T) {
	t.Run("from SIRET identity", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{
			{Type: fr.IdentityTypeSIRET, Code: "73282932000074"},
		}}
		assert.Equal(t, "732829320", sirenFromFrenchTaxID("anything", p))
	})
	t.Run("strips non-digits and takes last 9", func(t *testing.T) {
		assert.Equal(t, "732829320", sirenFromFrenchTaxID("FR44732829320", &org.Party{}))
	})
	t.Run("short code returned as-is", func(t *testing.T) {
		assert.Equal(t, "1234", sirenFromFrenchTaxID("FR1234", &org.Party{}))
	})
}

func TestEnsureSIRENIdentity(t *testing.T) {
	t.Run("empty code is a no-op", func(t *testing.T) {
		p := &org.Party{}
		ensureSIRENIdentity(p, "")
		assert.Empty(t, p.Identities)
	})
	t.Run("skips when scheme already present", func(t *testing.T) {
		p := &org.Party{Identities: []*org.Identity{sirenIdentity("111111111")}}
		ensureSIRENIdentity(p, "222222222")
		assert.Len(t, p.Identities, 1)
		assert.Equal(t, cbc.Code("111111111"), p.Identities[0].Code)
	})
	t.Run("appends when missing", func(t *testing.T) {
		p := &org.Party{}
		ensureSIRENIdentity(p, "333333333")
		require.Len(t, p.Identities, 1)
		assert.Equal(t, cbc.Code("333333333"), p.Identities[0].Code)
	})
}

func TestGetPartySIREN(t *testing.T) {
	assert.Equal(t, "", getPartySIREN(nil))
	assert.Equal(t, "", getPartySIREN(&org.Party{}))
	assert.Equal(t, "732829320", getPartySIREN(&org.Party{Identities: []*org.Identity{sirenIdentity("732829320")}}))
	// matches via ext scheme even without SIREN type
	p := &org.Party{Identities: []*org.Identity{
		{Code: "999", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDSIREN})},
	}}
	assert.Equal(t, "999", getPartySIREN(p))
}

func TestIsPartyIdentitySTC(t *testing.T) {
	assert.False(t, isPartyIdentitySTC(nil))
	assert.False(t, isPartyIdentitySTC(&org.Party{}))
	assert.False(t, isPartyIdentitySTC(&org.Party{Identities: []*org.Identity{sirenIdentity("1")}}))
	stc := &org.Party{Identities: []*org.Identity{
		{Code: "1", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: "0231"})},
	}}
	assert.True(t, isPartyIdentitySTC(stc))
}

func TestLegalIdentity(t *testing.T) {
	assert.Nil(t, legalIdentity(nil))
	assert.Nil(t, legalIdentity([]*org.Identity{nil, {Code: "1"}}))
	siren := sirenIdentity("732829320")
	assert.Same(t, siren, legalIdentity([]*org.Identity{{Code: "x"}, siren}))
}

func TestIdentitiesSingleLegalScope(t *testing.T) {
	assert.True(t, identitiesSingleLegalScope("wrong-type"))
	assert.True(t, identitiesSingleLegalScope([]*org.Identity{}))
	assert.True(t, identitiesSingleLegalScope([]*org.Identity{nil, sirenIdentity("1")}))
	assert.True(t, identitiesSingleLegalScope([]*org.Identity{
		sirenIdentity("732829320"), unscopedSIREN("732829320"),
	}))
	assert.False(t, identitiesSingleLegalScope([]*org.Identity{
		sirenIdentity("1"), sirenIdentity("2"),
	}))
}

func TestIdentitiesLegalIsSIREN(t *testing.T) {
	// non-slice value passes the guard
	assert.True(t, identitiesLegalIsSIREN("wrong-type"))
	// no legal identity present
	assert.False(t, identitiesLegalIsSIREN([]*org.Identity{}))
	// legal SIREN present
	assert.True(t, identitiesLegalIsSIREN([]*org.Identity{sirenIdentity("1")}))
	// SIREN scheme but no legal scope → no legal identity present
	nonLegal := []*org.Identity{
		{Code: "1", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDSIREN})},
	}
	assert.False(t, identitiesLegalIsSIREN(nonLegal))
	// legal scope on a non-SIREN identity → rejected
	legalNonSIREN := []*org.Identity{
		{
			Key:   identityKeyPrivateID,
			Code:  "ABC123",
			Scope: org.IdentityScopeLegal,
			Ext:   tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDPrivate}),
		},
	}
	assert.False(t, identitiesLegalIsSIREN(legalNonSIREN))
}

func TestPartyHasSIRENEndpoint(t *testing.T) {
	assert.True(t, partyHasSIRENEndpoint("wrong-type"))
	assert.True(t, partyHasSIRENEndpoint((*org.Party)(nil)))
	// no SIREN at all → passes
	assert.True(t, partyHasSIRENEndpoint(&org.Party{}))
	// SIREN present, matching endpoint (starts-with, per BR-FR-21/22)
	ok := &org.Party{
		Identities: []*org.Identity{sirenIdentity("732829320")},
		Endpoints:  []*org.Endpoint{{URI: "iso6523-actorid-upis::0225:732829320_PEP"}},
	}
	assert.True(t, partyHasSIRENEndpoint(ok))
	// SIREN present, wrong endpoint scheme
	badScheme := &org.Party{
		Identities: []*org.Identity{sirenIdentity("732829320")},
		Endpoints:  []*org.Endpoint{{URI: "iso6523-actorid-upis::9999:732829320"}},
	}
	assert.False(t, partyHasSIRENEndpoint(badScheme))
	// SIREN present, code does not start with it
	badCode := &org.Party{
		Identities: []*org.Identity{sirenIdentity("732829320")},
		Endpoints:  []*org.Endpoint{{URI: "iso6523-actorid-upis::0225:999999999"}},
	}
	assert.False(t, partyHasSIRENEndpoint(badCode))
	// SIREN present, no peppol endpoint at all
	noEndpoint := &org.Party{
		Identities: []*org.Identity{sirenIdentity("732829320")},
		Endpoints:  []*org.Endpoint{{URI: "mailto:billing@example.com"}},
	}
	assert.False(t, partyHasSIRENEndpoint(noEndpoint))
}

func TestEndpointAddressFormatValid(t *testing.T) {
	// out of the rule's reach: non-URI values and non-peppol endpoints
	assert.True(t, endpointAddressFormatValid("wrong-type"))
	assert.True(t, endpointAddressFormatValid(cbc.URI("mailto:billing@example.com")))
	// BR-FR-23 constrains addresses carrying scheme 0225
	assert.True(t, endpointAddressFormatValid(cbc.URI("iso6523-actorid-upis::0225:732829320_PEP")))
	assert.True(t, endpointAddressFormatValid(cbc.URI("iso6523-actorid-upis::0225:a.b-c+d_e")))
	assert.False(t, endpointAddressFormatValid(cbc.URI("iso6523-actorid-upis::0225:has/slash")))
	assert.False(t, endpointAddressFormatValid(cbc.URI("iso6523-actorid-upis::0225:has space")))
	// other ISO schemes are outside BR-FR-23
	assert.True(t, endpointAddressFormatValid(cbc.URI("iso6523-actorid-upis::0002:has/slash")))
}

func TestEndpointAddressLengthValid(t *testing.T) {
	assert.True(t, endpointAddressLengthValid("wrong-type"))
	assert.True(t, endpointAddressLengthValid(cbc.URI("iso6523-actorid-upis::0225:"+strings.Repeat("A", 125))))
	assert.False(t, endpointAddressLengthValid(cbc.URI("iso6523-actorid-upis::0225:"+strings.Repeat("A", 126))))
	// BR-FR-25 carries no scheme predicate, so it caps any address
	assert.False(t, endpointAddressLengthValid(cbc.URI("mailto:"+strings.Repeat("a", 126))))
}

func TestSplitPeppolEndpoint(t *testing.T) {
	scheme, code, ok := splitPeppolEndpoint(":0225:356000000")
	assert.True(t, ok)
	assert.Equal(t, "0225", scheme)
	assert.Equal(t, "356000000", code)
	_, _, ok = splitPeppolEndpoint(":0225:")
	assert.False(t, ok)
	_, _, ok = splitPeppolEndpoint("356000000")
	assert.False(t, ok)
}

func TestIdentitiesSIRETSIRENCoherent(t *testing.T) {
	assert.True(t, identitiesSIRETSIRENCoherent("wrong-type"))
	assert.True(t, identitiesSIRETSIRENCoherent([]*org.Identity{}))
	coherent := []*org.Identity{
		{Type: fr.IdentityTypeSIRET, Code: "73282932000074"},
		{Type: fr.IdentityTypeSIREN, Code: "732829320"},
	}
	assert.True(t, identitiesSIRETSIRENCoherent(coherent))
	incoherent := []*org.Identity{
		{Type: fr.IdentityTypeSIRET, Code: "73282932000074"},
		{Type: fr.IdentityTypeSIREN, Code: "999999999"},
	}
	assert.False(t, identitiesSIRETSIRENCoherent(incoherent))
}

func TestIdentitiesSchemeFormatValid(t *testing.T) {
	assert.NoError(t, identitiesSchemeFormatValid("wrong-type"))
	assert.NoError(t, identitiesSchemeFormatValid([]*org.Identity{}))

	t.Run("missing scheme errors", func(t *testing.T) {
		err := identitiesSchemeFormatValid([]*org.Identity{{Code: "1"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ISO scheme ID")
	})
	t.Run("duplicate scheme errors", func(t *testing.T) {
		ids := []*org.Identity{unscopedSIREN("1"), unscopedSIREN("2")}
		err := identitiesSchemeFormatValid(ids)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate")
	})
	t.Run("legal identity excluded from the uniqueness check", func(t *testing.T) {
		ids := []*org.Identity{sirenIdentity("732829320"), unscopedSIREN("732829320")}
		assert.NoError(t, identitiesSchemeFormatValid(ids))
	})
	t.Run("tax registration without scheme allowed", func(t *testing.T) {
		// BT-32 lands in `ram:SpecifiedTaxRegistration` / `cac:PartyTaxScheme`,
		// which BR-FR-CO-10 does not reach.
		ids := []*org.Identity{{Code: "828701557", Scope: org.IdentityScopeTax}}
		assert.NoError(t, identitiesSchemeFormatValid(ids))
	})
	t.Run("legal registration without scheme allowed", func(t *testing.T) {
		// BT-30 lands in `ram:SpecifiedLegalOrganization` / `cac:PartyLegalEntity`.
		ids := []*org.Identity{{Code: "356000000", Scope: org.IdentityScopeLegal}}
		assert.NoError(t, identitiesSchemeFormatValid(ids))
	})
	t.Run("scoped identity does not collide with party identifier", func(t *testing.T) {
		ids := []*org.Identity{
			sirenIdentity("356000000"),
			{Code: "356000000", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDSIREN})},
		}
		assert.NoError(t, identitiesSchemeFormatValid(ids))
	})
	t.Run("valid private-id", func(t *testing.T) {
		ids := []*org.Identity{
			{Code: "ABC-123", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDPrivate})},
		}
		assert.NoError(t, identitiesSchemeFormatValid(ids))
	})
	t.Run("empty private-id code allowed", func(t *testing.T) {
		ids := []*org.Identity{
			{Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDPrivate})},
		}
		assert.NoError(t, identitiesSchemeFormatValid(ids))
	})
	t.Run("private-id too long errors", func(t *testing.T) {
		ids := []*org.Identity{
			{Code: cbc.Code(strings.Repeat("A", 101)), Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDPrivate})},
		}
		err := identitiesSchemeFormatValid(ids)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "100 characters")
	})
	t.Run("private-id bad format errors", func(t *testing.T) {
		ids := []*org.Identity{
			{Code: "bad code!", Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDPrivate})},
		}
		err := identitiesSchemeFormatValid(ids)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "alphanumeric")
	})
}

func TestInboxCodeValid(t *testing.T) {
	assert.True(t, inboxCodeValid("wrong-type"))
	assert.True(t, inboxCodeValid((*org.Inbox)(nil)))
	// non-SIREN scheme passes regardless
	assert.True(t, inboxCodeValid(&org.Inbox{Scheme: "9999", Code: "anything goes"}))
	// SIREN scheme empty code passes
	assert.True(t, inboxCodeValid(&org.Inbox{Scheme: inboxSchemeSIREN}))
	// SIREN scheme valid code
	assert.True(t, inboxCodeValid(&org.Inbox{Scheme: inboxSchemeSIREN, Code: "732829320_PEP"}))
	// SIREN scheme too long
	assert.False(t, inboxCodeValid(&org.Inbox{Scheme: inboxSchemeSIREN, Code: cbc.Code(strings.Repeat("A", 126))}))
	// SIREN scheme bad format
	assert.False(t, inboxCodeValid(&org.Inbox{Scheme: inboxSchemeSIREN, Code: "bad code"}))
}

func TestSchemeGuards(t *testing.T) {
	assert.False(t, identitySchemeIs0224("wrong-type"))
	assert.False(t, identitySchemeIs0224(&org.Identity{}))
	assert.True(t, identitySchemeIs0224(&org.Identity{Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDPrivate})}))

	assert.False(t, identitySchemeIsSIRENBased("wrong-type"))
	assert.False(t, identitySchemeIsSIRENBased((*org.Identity)(nil)))
	assert.False(t, identitySchemeIsSIRENBased(&org.Identity{}))
	assert.False(t, identitySchemeIsSIRENBased(&org.Identity{Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDSIRET})}))
	assert.True(t, identitySchemeIsSIRENBased(&org.Identity{Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDSIREN})}))
	assert.True(t, identitySchemeIsSIRENBased(&org.Identity{Ext: tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: identitySchemeIDSTC})}))

	assert.False(t, inboxSchemeIs0225("wrong-type"))
	assert.False(t, inboxSchemeIs0225(&org.Inbox{Scheme: "9999"}))
	assert.True(t, inboxSchemeIs0225(&org.Inbox{Scheme: inboxSchemeSIREN}))
}

func TestMetaNoBlankValues(t *testing.T) {
	assert.NoError(t, metaNoBlankValues("wrong-type"))
	assert.NoError(t, metaNoBlankValues(cbc.Meta{}))
	assert.NoError(t, metaNoBlankValues(cbc.Meta{"k": "v"}))
	err := metaNoBlankValues(cbc.Meta{"k": "   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be blank")
}
