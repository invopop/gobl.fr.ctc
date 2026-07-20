package flow2

import (
	"errors"
	"fmt"
	"regexp"
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

// Inbox / identity scheme constants used across Flow 2.
const (
	inboxSchemeSIREN        cbc.Code = "0225"
	identitySchemeIDSIREN   cbc.Code = "0002"
	identitySchemeIDSIRET   cbc.Code = "0009"
	identitySchemeIDPrivate cbc.Code = "0224"
	identityKeyPrivateID    cbc.Key  = "private-id"
)

// peppolEndpointScheme is the URI scheme GOBL uses for Peppol participant
// identifier endpoints (e.g. "iso6523-actorid-upis::0225:356000000").
const peppolEndpointScheme = "iso6523-actorid-upis"

// sirenInboxFormatRegex enforces the alphanumeric + `-+_/` format
// shared by SIREN-scope inboxes and private-id identity codes.
var sirenInboxFormatRegex = regexp.MustCompile(`^[A-Za-z0-9+\-_/]+$`)

func normalizeParty(party *org.Party) {
	if party == nil {
		return
	}
	normalizePartyFromTaxID(party)
	normalizeIdentities(party)
	ensureInboxFromEndpoint(party)
	normalizeInboxes(party)
}

// ensureInboxFromEndpoint back-fills a party's Peppol inbox from its
// `iso6523-actorid-upis::<scheme>:<code>` endpoint when the party carries
// that endpoint (the canonical electronic-address model) but no inbox. It
// is the inverse of the eu/en16931 addon's inbox→endpoint migration and
// keeps the Flow 2 inbox rules (BR-FR-13/21/22) satisfied for parties
// expressed only with endpoints — e.g. invoices parsed from UBL/CII.
func ensureInboxFromEndpoint(party *org.Party) {
	if party == nil || len(party.Inboxes) > 0 {
		return
	}
	ep := party.Endpoint(peppolEndpointScheme)
	if ep == nil {
		return
	}
	scheme, code, ok := splitPeppolEndpoint(ep.URI.Opaque())
	if !ok {
		return
	}
	party.Inboxes = []*org.Inbox{{
		Key:    org.InboxKeyPeppol,
		Scheme: cbc.Code(scheme),
		Code:   cbc.Code(code),
	}}
}

// splitPeppolEndpoint splits the opaque part of an
// "iso6523-actorid-upis::<scheme>:<code>" URI (which URL parsing exposes
// as ":<scheme>:<code>") into its scheme and code components.
func splitPeppolEndpoint(opaque string) (scheme, code string, ok bool) {
	parts := strings.SplitN(strings.TrimPrefix(opaque, ":"), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// normalizePartyFromTaxID derives a legal identity from the party's
// TaxID when no matching identity is present.
func normalizePartyFromTaxID(party *org.Party) {
	if party.TaxID == nil {
		return
	}
	country := l10n.Code(party.TaxID.Country)
	code := string(party.TaxID.Code)
	if code == "" || country != l10n.FR {
		return
	}
	ensureSIRENIdentity(party, cbc.Code(sirenFromFrenchTaxID(code, party)))
}

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

// ensureSIRENIdentity appends a SIREN legal identity (ISO scheme 0002)
// when the party does not already carry one.
func ensureSIRENIdentity(party *org.Party, code cbc.Code) {
	if code == "" {
		return
	}
	for _, id := range party.Identities {
		if id != nil && !id.Ext.IsZero() && id.Ext.Get(iso.ExtKeySchemeID) == identitySchemeIDSIREN {
			return
		}
	}
	party.Identities = append(party.Identities, &org.Identity{
		Type: fr.IdentityTypeSIREN,
		Code: code,
		Ext: tax.ExtensionsOf(cbc.CodeMap{
			iso.ExtKeySchemeID: identitySchemeIDSIREN,
		}),
		Scope: org.IdentityScopeLegal,
	})
}

func normalizeIdentities(party *org.Party) {
	if party == nil || len(party.Identities) == 0 {
		return
	}
	var siret, siren *org.Identity
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
	}
	// BR-FR-09/10: Generate SIREN from SIRET if needed.
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
	// The SIREN is France's legal identifier: it must always carry the
	// legal scope (any other legal-scoped identity is rejected in the rules).
	if siren != nil {
		siren.Scope = org.IdentityScopeLegal
	}
}

func normalizeIdentity(id *org.Identity) {
	if id == nil {
		return
	}
	if id.Key == identityKeyPrivateID {
		id.Ext = id.Ext.Set(iso.ExtKeySchemeID, identitySchemeIDPrivate)
	}
	if id.Type == fr.IdentityTypeSIREN && id.Ext.Get(iso.ExtKeySchemeID) == "" {
		id.Ext = id.Ext.Set(iso.ExtKeySchemeID, identitySchemeIDSIREN)
	}
	if id.Type == fr.IdentityTypeSIRET && id.Ext.Get(iso.ExtKeySchemeID) == "" {
		id.Ext = id.Ext.Set(iso.ExtKeySchemeID, identitySchemeIDSIRET)
	}
}

func normalizeInboxes(party *org.Party) {
	if party == nil || len(party.Inboxes) == 0 {
		return
	}
	hasPeppol := false
	var sirenInbox *org.Inbox
	for _, inbox := range party.Inboxes {
		if inbox == nil {
			continue
		}
		if inbox.Key == org.InboxKeyPeppol {
			hasPeppol = true
		}
		if inbox.Scheme == inboxSchemeSIREN {
			sirenInbox = inbox
		}
	}
	if !hasPeppol && sirenInbox != nil {
		sirenInbox.Key = org.InboxKeyPeppol
	}
}

// -- Helpers --------------------------------------------------------------

// getPartySIREN returns the code of the party's SIREN identity (ISO
// scheme 0002), or "" if none is present. Identities are normalized to
// carry the scheme before validation runs.
func getPartySIREN(party *org.Party) string {
	if party == nil {
		return ""
	}
	for _, id := range party.Identities {
		if id != nil && id.Ext.Get(iso.ExtKeySchemeID) == identitySchemeIDSIREN {
			return string(id.Code)
		}
	}
	return ""
}

func isPartyIdentitySTC(party *org.Party) bool {
	if party == nil || len(party.Identities) == 0 {
		return false
	}
	for _, id := range party.Identities {
		if id != nil && !id.Ext.IsZero() {
			if code := id.Ext.Get(iso.ExtKeySchemeID); code == "0231" {
				return true
			}
		}
	}
	return false
}

// legalIdentity returns the identity carrying the legal scope, or nil if
// none is present.
func legalIdentity(identities []*org.Identity) *org.Identity {
	for _, id := range identities {
		if id != nil && id.Scope.Has(org.IdentityScopeLegal) {
			return id
		}
	}
	return nil
}

// identitiesLegalIsSIREN reports whether the party's legal identity is
// present and carries the SIREN ISO scheme (0002).
func identitiesLegalIsSIREN(val any) bool {
	identities, ok := val.([]*org.Identity)
	if !ok {
		return true
	}
	id := legalIdentity(identities)
	return id != nil && id.Ext.Get(iso.ExtKeySchemeID) == identitySchemeIDSIREN
}

func partyHasSIRENInbox(val any) bool {
	party, ok := val.(*org.Party)
	if !ok || party == nil {
		return true
	}
	siren := getPartySIREN(party)
	if siren == "" {
		return true
	}
	for _, inbox := range party.Inboxes {
		if inbox != nil && inbox.Scheme == inboxSchemeSIREN {
			return strings.HasPrefix(string(inbox.Code), siren)
		}
	}
	return false
}

// -- Rules ----------------------------------------------------------------

func orgPartyRules() *rules.Set {
	return rules.For(new(org.Party),
		rules.Field("identities",
			rules.Assert("01", "SIRET and SIREN must be coherent (BR-FR-09/10)",
				is.Func("SIRET/SIREN coherent", identitiesSIRETSIRENCoherent),
			),
			rules.Assert("02", "identity scheme format invalid (BR-FR-CO-10)",
				is.FuncError("valid scheme format", identitiesSchemeFormatValid),
			),
		),
		rules.Field("inboxes",
			rules.Each(
				rules.Assert("03", "inbox code format invalid",
					is.Func("valid inbox", inboxCodeValid),
				),
			),
		),
	)
}

func orgIdentityRules() *rules.Set {
	return rules.For(new(org.Identity),
		rules.When(
			is.Func("scheme 0224", identitySchemeIs0224),
			rules.Field("code",
				rules.Assert("01", "must be no more than 100 characters long",
					is.Length(0, 100),
				),
				rules.Assert("02", "must be in a valid format",
					is.Matches(`^[A-Za-z0-9\-\+_/]+$`),
				),
			),
		),
	)
}

func orgInboxRules() *rules.Set {
	return rules.For(new(org.Inbox),
		rules.When(
			is.Func("scheme 0225", inboxSchemeIs0225),
			rules.Field("code",
				rules.Assert("01", "the length must be between 0 and 125",
					is.Length(0, 125),
				),
				rules.Assert("02", "must be in a valid format",
					is.Matches(`^[A-Za-z0-9\-\+_/]+$`),
				),
			),
		),
	)
}

func orgItemRules() *rules.Set {
	return rules.For(new(org.Item),
		rules.Field("meta",
			rules.Assert("01", "meta values cannot be blank (BR-FR-28)",
				is.FuncError("no blank meta", metaNoBlankValues),
			),
		),
	)
}

// -- Validation helpers ---------------------------------------------------

func identitiesSIRETSIRENCoherent(val any) bool {
	identities, ok := val.([]*org.Identity)
	if !ok || len(identities) == 0 {
		return true
	}
	var siret, siren *org.Identity
	for _, id := range identities {
		if id == nil {
			continue
		}
		if id.Type == fr.IdentityTypeSIRET {
			siret = id
		}
		if id.Type == fr.IdentityTypeSIREN {
			siren = id
		}
	}
	if siret != nil && siren != nil {
		siretCode := string(siret.Code)
		sirenCode := string(siren.Code)
		if len(siretCode) == 14 && len(sirenCode) == 9 {
			if !strings.HasPrefix(siretCode, sirenCode) {
				return false
			}
		}
	}
	return true
}

func identitiesSchemeFormatValid(val any) error {
	identities, ok := val.([]*org.Identity)
	if !ok || len(identities) == 0 {
		return nil
	}
	schemes := make(map[cbc.Code]bool)
	for _, id := range identities {
		if id == nil {
			continue
		}
		schemeID := id.Ext.Get(iso.ExtKeySchemeID)
		if schemeID == cbc.CodeEmpty {
			return errors.New("all identities must have an ISO scheme ID defined in extensions BR-FR-CO-10")
		}
		if schemes[schemeID] {
			return fmt.Errorf("duplicate identities with ISO scheme ID '%s' are not allowed (BR-FR-CO-10)", schemeID)
		}
		if schemeID == identitySchemeIDPrivate {
			code := string(id.Code)
			if code == "" {
				schemes[schemeID] = true
				continue
			}
			if len(code) > 100 {
				return errors.New("identity with ISO scheme ID 0224 (private-id) must not exceed 100 characters (BR-FR-26)")
			}
			if !sirenInboxFormatRegex.MatchString(code) {
				return errors.New("identity with ISO scheme ID 0224 (private-id) must contain only alphanumeric characters and +, -, _, / (BR-FR-24)")
			}
		}
		schemes[schemeID] = true
	}
	return nil
}

func inboxCodeValid(val any) bool {
	inbox, ok := val.(*org.Inbox)
	if !ok || inbox == nil {
		return true
	}
	if inbox.Scheme != inboxSchemeSIREN {
		return true
	}
	code := string(inbox.Code)
	if code == "" {
		return true
	}
	if len(code) > 125 {
		return false
	}
	return sirenInboxFormatRegex.MatchString(code)
}

func identitySchemeIs0224(val any) bool {
	id, ok := val.(*org.Identity)
	return ok && id != nil && !id.Ext.IsZero() && id.Ext.Get(iso.ExtKeySchemeID) == identitySchemeIDPrivate
}

func inboxSchemeIs0225(val any) bool {
	inbox, ok := val.(*org.Inbox)
	return ok && inbox != nil && inbox.Scheme == inboxSchemeSIREN
}

func metaNoBlankValues(val any) error {
	meta, ok := val.(cbc.Meta)
	if !ok || len(meta) == 0 {
		return nil
	}
	for key, v := range meta {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s: value cannot be blank (BR-FR-28)", key)
		}
	}
	return nil
}
