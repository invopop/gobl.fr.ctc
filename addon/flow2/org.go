package flow2

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
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
	identitySchemeIDSTC     cbc.Code = "0231"
	identityKeyPrivateID    cbc.Key  = "private-id"
)

// The opaque part of an ISO 6523 endpoint on the French 0225 scheme, and
// BR-FR-23's charset for its address. The charset deliberately differs from
// sirenInboxFormatRegex on `.` and `/`.
var (
	endpointSIRENScheme  = fmt.Sprintf(`^:%s:`, inboxSchemeSIREN)
	endpointSIRENAddress = fmt.Sprintf(`^:%s:[A-Za-z0-9+\-_.]+$`, inboxSchemeSIREN)
)

// sirenInboxFormatRegex enforces the alphanumeric + `-+_/` format required of
// private-id identity codes.
var sirenInboxFormatRegex = regexp.MustCompile(`^[A-Za-z0-9+\-_/]+$`)

func normalizeParty(party *org.Party) {
	if party == nil {
		return
	}
	normalizePartyFromTaxID(party)
	normalizeIdentities(party)
	normalizeInboxes(party)
	ensureEndpointFromInbox(party)
}

// ensureEndpointFromInbox migrates a deprecated Peppol inbox to the canonical
// endpoint. en16931 normalizes before this addon, so it misses the peppol key
// that normalizeInboxes assigns above.
func ensureEndpointFromInbox(party *org.Party) {
	if party == nil || party.Endpoint(iso.ActorIDScheme) != nil {
		return
	}
	for _, inbox := range party.Inboxes {
		if inbox == nil || inbox.Key != org.InboxKeyPeppol {
			continue
		}
		if inbox.Scheme == cbc.CodeEmpty || inbox.Code == cbc.CodeEmpty {
			continue
		}
		party.Endpoints = append(party.Endpoints, &org.Endpoint{
			Label: inbox.Label,
			URI:   cbc.URI(iso.ActorIDScheme + "::" + inbox.Scheme.String() + ":" + inbox.Code.String()),
		})
		return
	}
}

// splitPeppolEndpoint splits ":<scheme>:<code>", the form URI parsing exposes
// the opaque part of "iso6523-actorid-upis::<scheme>:<code>" in.
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
		if id == nil {
			continue
		}
		// The scheme is only set later by normalizeIdentity.
		if id.Type == fr.IdentityTypeSIREN || id.Ext.Get(iso.ExtKeySchemeID) == identitySchemeIDSIREN {
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
	assignSIRENLegalScope(party.Identities)
}

// assignSIRENLegalScope gives the legal scope to an unscoped SIREN when no
// identity claims it yet. Anything else is left for the rules to reject.
func assignSIRENLegalScope(identities []*org.Identity) {
	for _, id := range identities {
		if id != nil && id.Scope.Has(org.IdentityScopeLegal) {
			return
		}
	}
	for _, id := range identities {
		if id != nil && id.Type == fr.IdentityTypeSIREN && id.Scope == cbc.KeyEmpty {
			id.Scope = org.IdentityScopeLegal
			return
		}
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
			if code := id.Ext.Get(iso.ExtKeySchemeID); code == identitySchemeIDSTC {
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

// partyHasSIRENEndpoint reports whether the party's Peppol endpoint is on
// scheme 0225 with a code starting with its SIREN (BR-FR-21/22).
func partyHasSIRENEndpoint(val any) bool {
	party, ok := val.(*org.Party)
	if !ok || party == nil {
		return true
	}
	siren := getPartySIREN(party)
	if siren == "" {
		return true
	}
	ep := party.Endpoint(iso.ActorIDScheme)
	if ep == nil {
		return false
	}
	scheme, code, ok := splitPeppolEndpoint(ep.URI.Opaque())
	if !ok {
		return false
	}
	return cbc.Code(scheme) == inboxSchemeSIREN && strings.HasPrefix(code, siren)
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
	)
}

func orgIdentityRules() *rules.Set {
	return rules.For(new(org.Identity),
		rules.When(
			is.Func("scheme 0224", identitySchemeIs0224),
			rules.Field("code",
				rules.Assert("01", "identity code must be no more than 100 characters long",
					is.Length(0, 100),
				),
				rules.Assert("02", "identity code must be in a valid format",
					is.Matches(`^[A-Za-z0-9\-\+_/]+$`),
				),
			),
		),
		rules.When(
			is.Func("scheme 0002 or 0231", identitySchemeIsSIRENBased),
			rules.Field("code",
				rules.Assert("03", "identity code must be exactly 9 digits (BR-FR-32)",
					is.Matches(`^\d{9}$`),
				),
			),
		),
	)
}

// orgEndpointRules constrains the ISO 6523 endpoint, which is the one the
// electronic address terms BT-34/BT-49 carry. Other schemes, such as a
// mailto: or gobl: routing address, are outside the French rules.
func orgEndpointRules() *rules.Set {
	return rules.For(new(org.Endpoint),
		rules.Field("uri",
			rules.When(cbc.URISchemeIn(iso.ActorIDScheme),
				rules.When(cbc.URIOpaqueMatches(endpointSIRENScheme),
					rules.Assert("01", "endpoint address on scheme 0225 must contain only alphanumeric characters and +, -, _, . (BR-FR-23)",
						cbc.URIOpaqueMatches(endpointSIRENAddress),
					),
				),
				rules.Assert("02", "endpoint address must not exceed 125 characters (BR-FR-25)",
					is.Func("address within 125 characters", endpointAddressLengthValid),
				),
			),
		),
	)
}

func endpointAddressLengthValid(val any) bool {
	uri, ok := val.(cbc.URI)
	if !ok {
		return true
	}
	// A malformed pair has no code to measure, so cap the whole opaque part.
	value := uri.Opaque()
	if _, code, ok := splitPeppolEndpoint(value); ok {
		value = code
	}
	return len(value) <= 125
}

// orgNoteRules relaxes GOBL-ORG-NOTE-01, which requires text on every note. A
// Flow 2 note may instead carry only its UNTDID 4451 subject code, which is
// the content in that case, so one or the other is required.
func orgNoteRules() *rules.Set {
	return rules.For(new(org.Note),
		rules.Ignore("GOBL-ORG-NOTE-01"),
		rules.Object(
			rules.Assert("01", "note must carry either text or an untdid-text-subject extension",
				is.Func("text or subject", noteHasTextOrSubject),
			),
		),
	)
}

func noteHasTextOrSubject(val any) bool {
	note, ok := val.(*org.Note)
	if !ok || note == nil {
		return true
	}
	return note.Text != "" || note.Ext.Get(untdid.ExtKeyTextSubject) != cbc.CodeEmpty
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
		// BR-FR-CO-10 is bound to GlobalID, so the legal (BT-30) and tax
		// (BT-32) registrations are out of its scope.
		if id == nil || id.Scope.Has(org.IdentityScopeLegal) || id.Scope.Has(org.IdentityScopeTax) {
			continue
		}
		schemeID := id.Ext.Get(iso.ExtKeySchemeID)
		if schemeID == cbc.CodeEmpty {
			return errors.New("all party identifiers must have an ISO scheme ID defined in extensions BR-FR-CO-10")
		}
		if schemes[schemeID] {
			return fmt.Errorf("duplicate party identifiers with ISO scheme ID '%s' are not allowed (BR-FR-CO-10)", schemeID)
		}
		schemes[schemeID] = true
		if schemeID == identitySchemeIDPrivate {
			code := string(id.Code)
			if code == "" {
				continue
			}
			if len(code) > 100 {
				return errors.New("identity with ISO scheme ID 0224 (private-id) must not exceed 100 characters (BR-FR-26)")
			}
			if !sirenInboxFormatRegex.MatchString(code) {
				return errors.New("identity with ISO scheme ID 0224 (private-id) must contain only alphanumeric characters and +, -, _, / (BR-FR-24)")
			}
		}
	}
	return nil
}

func identitySchemeIs0224(val any) bool {
	id, ok := val.(*org.Identity)
	return ok && id != nil && !id.Ext.IsZero() && id.Ext.Get(iso.ExtKeySchemeID) == identitySchemeIDPrivate
}

// identitySchemeIsSIRENBased reports whether the identity carries a
// scheme whose code must be a SIREN.
func identitySchemeIsSIRENBased(val any) bool {
	id, ok := val.(*org.Identity)
	if !ok || id == nil || id.Ext.IsZero() {
		return false
	}
	scheme := id.Ext.Get(iso.ExtKeySchemeID)
	return scheme == identitySchemeIDSIREN || scheme == identitySchemeIDSTC
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
