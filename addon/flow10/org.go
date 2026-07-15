package flow10

import (
	"slices"
	"strings"

	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/fr"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// Identity scheme constants used by Flow 10 reporting.
const (
	// identitySchemeIDSIREN is the ISO scheme ID for SIREN identities.
	identitySchemeIDSIREN = "0002"
	// identitySchemeIDSIRET is the ISO scheme ID for SIRET identities.
	identitySchemeIDSIRET = "0009"
	// identitySchemeIDEUVAT is the ISO scheme ID for EU (non-French)
	// intra-community VAT.
	identitySchemeIDEUVAT = "0223"
	// identitySchemeIDNonEU is the ISO scheme ID for non-EU party
	// identifiers.
	identitySchemeIDNonEU = "0227"
	// identitySchemeIDRIDET is the ISO scheme ID for New Caledonia
	// RIDET.
	identitySchemeIDRIDET = "0228"
	// identitySchemeIDTAHITI is the ISO scheme ID for French Polynesia
	// TAHITI.
	identitySchemeIDTAHITI = "0229"
)

// allowedPartySchemeIDs lists the scheme IDs permitted for the legal
// identity of a Flow 10 B2B party (supplier or customer), per G2.19.
var allowedPartySchemeIDs = []string{
	identitySchemeIDSIREN,
	identitySchemeIDEUVAT,
	identitySchemeIDNonEU,
	identitySchemeIDRIDET,
	identitySchemeIDTAHITI,
}

// schemeIDsRequiringVAT are the scheme IDs for which party.TaxID must
// also be present (G2.33): SIREN (French) and EU non-French VAT.
var schemeIDsRequiringVAT = []string{
	identitySchemeIDSIREN,
	identitySchemeIDEUVAT,
}

func normalizeParty(party *org.Party) {
	if party == nil {
		return
	}
	normalizePartyFromTaxID(party)
	normalizeIdentities(party)
}

// normalizePartyFromTaxID derives a legal identity from the party's
// TaxID when no matching identity is present. The identity scheme is
// chosen from the TaxID country:
//   - France           → SIREN (0002)
//   - other EU          → EU-VAT (0223), country code + VAT number
//   - New Caledonia     → RIDET (0228)
//   - French Polynesia  → TAHITI (0229)
//   - any other country → HORS_UE (0227), country code + first 16
//     characters of the party name (incl. Wallis & Futuna)
func normalizePartyFromTaxID(party *org.Party) {
	if party.TaxID == nil {
		return
	}
	country := l10n.Code(party.TaxID.Country)
	code := string(party.TaxID.Code)
	// Derivation keys off the TaxID country, not the code: HORS_UE is
	// built from country + name and needs no code at all, and many
	// parties carry a country without a tax number. Schemes that do
	// need the code guard for it individually below.
	if country == "" {
		return
	}
	switch {
	case country == l10n.FR:
		ensureIdentity(party, fr.IdentityTypeSIREN, cbc.Code(sirenFromFrenchTaxID(code, party)), identitySchemeIDSIREN)
	case isEUNonFrance(country):
		// EU VAT is the country code + VAT number, so the code is required.
		if code != "" {
			ensureIdentity(party, "", cbc.Code(country.String()+code), identitySchemeIDEUVAT)
		}
	case country == l10n.NC:
		ensureIdentity(party, "", cbc.Code(code), identitySchemeIDRIDET)
	case country == l10n.PF:
		ensureIdentity(party, "", cbc.Code(code), identitySchemeIDTAHITI)
	default:
		if id := nonEUIdentityCode(country, party.Name); id != "" {
			ensureIdentity(party, "", cbc.Code(id), identitySchemeIDNonEU)
		}
	}
}

// nonEUIdentityCode builds the 18-character HORS_UE (0227) identifier:
// the 2-letter ISO country code followed by up to the first 16
// alphanumeric characters of the party name. Returns an empty string
// (so no identity is created) when the country or name is missing.
func nonEUIdentityCode(country l10n.Code, name string) string {
	if country == "" {
		return ""
	}
	clean := cbc.NormalizeAlphanumericalCode(cbc.Code(name)).String()
	if clean == "" {
		return ""
	}
	if len(clean) > 16 {
		clean = clean[:16]
	}
	return country.String() + clean
}

// sirenFromFrenchTaxID extracts the 9-digit SIREN from a French TaxID.
// Prefers the first 9 digits of a present SIRET identity, otherwise
// falls back to the last 9 digits of the TaxID code.
func sirenFromFrenchTaxID(taxCode string, party *org.Party) string {
	for _, id := range party.Identities {
		if id != nil && id.Type == fr.IdentityTypeSIRET {
			s := string(id.Code)
			if len(s) == 14 {
				return s[:9]
			}
		}
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, taxCode)
	if len(digits) >= 9 {
		return digits[len(digits)-9:]
	}
	return digits
}

// ensureIdentity adds an identity matching the given scheme ID if none
// is already present.
func ensureIdentity(party *org.Party, typ cbc.Code, code cbc.Code, schemeID string) {
	if code == "" {
		return
	}
	for _, id := range party.Identities {
		if id != nil && !id.Ext.IsZero() && id.Ext.Get(iso.ExtKeySchemeID).String() == schemeID {
			return
		}
	}
	party.Identities = append(party.Identities, &org.Identity{
		Type: typ,
		Code: code,
		Ext: tax.ExtensionsOf(cbc.CodeMap{
			iso.ExtKeySchemeID: cbc.Code(schemeID),
		}),
		Scope: org.IdentityScopeLegal,
	})
}

func normalizeIdentities(party *org.Party) {
	if party == nil || len(party.Identities) == 0 {
		return
	}
	var siret, siren *org.Identity
	hasLegalScope := false
	for _, id := range party.Identities {
		if id == nil {
			continue
		}
		normalizeIdentity(id)
		if id.Type == fr.IdentityTypeSIRET {
			siret = id
		}
		if id.Type == fr.IdentityTypeSIREN {
			siren = id
		}
		if id.Scope == org.IdentityScopeLegal {
			hasLegalScope = true
		}
	}
	// Generate SIREN from SIRET if needed.
	if siret != nil && siren == nil {
		siretCode := string(siret.Code)
		if len(siretCode) == 14 {
			sirenCode := siretCode[:9]
			siren = &org.Identity{
				Type: fr.IdentityTypeSIREN,
				Code: cbc.Code(sirenCode),
				Ext: tax.ExtensionsOf(cbc.CodeMap{
					iso.ExtKeySchemeID: identitySchemeIDSIREN,
				}),
			}
			party.Identities = append(party.Identities, siren)
		}
	}
	if siren != nil && !hasLegalScope {
		siren.Scope = org.IdentityScopeLegal
	}
}

func normalizeIdentity(id *org.Identity) {
	if id == nil {
		return
	}
	if id.Type == fr.IdentityTypeSIREN && id.Ext.Get(iso.ExtKeySchemeID) == "" {
		id.Ext = id.Ext.Set(iso.ExtKeySchemeID, identitySchemeIDSIREN)
	}
	if id.Type == fr.IdentityTypeSIRET && id.Ext.Get(iso.ExtKeySchemeID) == "" {
		id.Ext = id.Ext.Set(iso.ExtKeySchemeID, identitySchemeIDSIRET)
	}
}

func isEUNonFrance(c l10n.Code) bool {
	if c == l10n.FR || c == "" {
		return false
	}
	eu := l10n.Union(l10n.EU)
	return eu != nil && eu.HasMember(c)
}

// partyLegalSchemeID returns the ICD 6523 scheme ID of the identity
// the party presents as its legal identifier.
func partyLegalSchemeID(party *org.Party) string {
	if party == nil {
		return ""
	}
	var fallback string
	for _, id := range party.Identities {
		if id == nil || id.Ext.IsZero() {
			continue
		}
		scheme := id.Ext.Get(iso.ExtKeySchemeID).String()
		if scheme == "" {
			continue
		}
		if id.Scope == org.IdentityScopeLegal {
			return scheme
		}
		if fallback == "" && slices.Contains(allowedPartySchemeIDs, scheme) {
			fallback = scheme
		}
	}
	return fallback
}

// partyHasSIREN reports whether the party carries a SIREN-scheme
// (0002) identity.
func partyHasSIREN(v any) bool {
	party, ok := v.(*org.Party)
	if !ok || party == nil {
		return false
	}
	for _, id := range party.Identities {
		if id == nil {
			continue
		}
		if id.Type == fr.IdentityTypeSIREN {
			return true
		}
		if !id.Ext.IsZero() && id.Ext.Get(iso.ExtKeySchemeID).String() == identitySchemeIDSIREN {
			return true
		}
	}
	return false
}

func partyHasAllowedLegalScheme(v any) bool {
	party, ok := v.(*org.Party)
	if !ok || party == nil {
		return false
	}
	return slices.Contains(allowedPartySchemeIDs, partyLegalSchemeID(party))
}

func partyHasTaxIDWhenRequired(v any) bool {
	party, ok := v.(*org.Party)
	if !ok || party == nil {
		return true
	}
	scheme := partyLegalSchemeID(party)
	if !slices.Contains(schemeIDsRequiringVAT, scheme) {
		return true
	}
	return party.TaxID != nil && party.TaxID.Code != ""
}

func partyHasVATCode(p *org.Party) bool {
	return p != nil && p.TaxID != nil && p.TaxID.Code != ""
}

func orgPartyRules() *rules.Set {
	return rules.For(new(org.Party),
		rules.Field("identities",
			rules.Assert("01", "party identities must not duplicate iso-scheme-id values (BR-FR-CO-10)",
				is.Func("unique iso-scheme-id", identitiesSchemesUnique),
			),
			rules.Assert("03", "only one identity may have the legal scope",
				is.Func("single legal-scope identity", identitiesSingleLegalScope),
			),
			rules.Each(
				rules.Field("ext",
					rules.Assert("02", "party identity ext iso-scheme-id is required (BR-FR-CO-10)",
						tax.ExtensionsRequire(iso.ExtKeySchemeID),
					),
				),
			),
		),
	)
}

// orgIdentityRules validates the code of the France-specific identity
// schemes against the lengths and format required by the CTC spec (G1.73).
func orgIdentityRules() *rules.Set {
	return rules.For(new(org.Identity),
		// 0227 HORS_UE: up to 18 uppercase alphanumeric characters
		// (2-letter country code + up to 16 name characters).
		rules.When(
			is.Func("scheme 0227", identitySchemeIs(identitySchemeIDNonEU)),
			rules.Field("code",
				rules.Assert("01", "HORS_UE (0227) identity must be no more than 18 characters (G1.73)",
					is.Length(0, 18),
				),
				rules.Assert("02", "HORS_UE (0227) identity must be uppercase alphanumeric (G1.73)",
					is.Matches(`^[A-Z0-9]+$`),
				),
			),
		),
		// 0228 RIDET: 9 or 10 characters.
		rules.When(
			is.Func("scheme 0228", identitySchemeIs(identitySchemeIDRIDET)),
			rules.Field("code",
				rules.Assert("03", "RIDET (0228) identity must be 9 or 10 characters (G1.73)",
					is.Length(9, 10),
				),
			),
		),
		// 0229 TAHITI: 9 characters.
		rules.When(
			is.Func("scheme 0229", identitySchemeIs(identitySchemeIDTAHITI)),
			rules.Field("code",
				rules.Assert("04", "TAHITI (0229) identity must be 9 characters (G1.73)",
					is.Length(9, 9),
				),
			),
		),
	)
}

// identitySchemeIs returns a test that passes when the identity carries the
// given ISO 6523 scheme ID in its extensions.
func identitySchemeIs(scheme string) func(any) bool {
	return func(val any) bool {
		id, ok := val.(*org.Identity)
		return ok && id != nil && !id.Ext.IsZero() &&
			id.Ext.Get(iso.ExtKeySchemeID).String() == scheme
	}
}

// identitiesSingleLegalScope reports whether at most one identity carries
// the legal scope.
func identitiesSingleLegalScope(val any) bool {
	identities, ok := val.([]*org.Identity)
	if !ok {
		return true
	}
	legal := 0
	for _, id := range identities {
		if id != nil && id.Scope.Has(org.IdentityScopeLegal) {
			legal++
		}
	}
	return legal <= 1
}

// identitiesSchemesUnique reports whether the slice contains at most
// one identity per iso-scheme-id value. Empty extensions are ignored
// (the per-identity rule covers them).
func identitiesSchemesUnique(val any) bool {
	identities, ok := val.([]*org.Identity)
	if !ok || len(identities) == 0 {
		return true
	}
	seen := make(map[cbc.Code]bool, len(identities))
	for _, id := range identities {
		if id == nil {
			continue
		}
		schemeID := id.Ext.Get(iso.ExtKeySchemeID)
		if schemeID == cbc.CodeEmpty {
			continue
		}
		if seen[schemeID] {
			return false
		}
		seen[schemeID] = true
	}
	return true
}
