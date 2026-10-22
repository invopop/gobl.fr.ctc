package cii

import (
	"strings"
	"testing"

	"github.com/invopop/gobl/addons/fr/choruspro"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/fr"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chorusProLegalOrgID picks the legal identifier Chorus Pro expects, which
// depends entirely on the scheme the party declares.
func TestChorusProLegalOrgID(t *testing.T) {
	withScheme := func(scheme string, f func(p *org.Party)) *org.Party {
		p := &org.Party{
			Name: "Acme Industries Limited",
			Ext:  tax.ExtensionsOf(cbc.CodeMap{choruspro.ExtKeyScheme: cbc.Code(scheme)}),
		}
		if f != nil {
			f(p)
		}
		return p
	}

	t.Run("scheme 1 takes the SIRET identity", func(t *testing.T) {
		pid := chorusProLegalOrgID(withScheme("1", func(p *org.Party) {
			p.Identities = []*org.Identity{
				{Type: "OTHER", Code: "ignored"},
				{Type: fr.IdentityTypeSIRET, Code: "73282932000074"},
			}
		}))
		assert.Equal(t, "1", pid.SchemeID)
		assert.Equal(t, "73282932000074", pid.Value)
	})

	t.Run("scheme 1 without a SIRET leaves the value empty", func(t *testing.T) {
		pid := chorusProLegalOrgID(withScheme("1", nil))
		assert.Equal(t, "1", pid.SchemeID)
		assert.Empty(t, pid.Value)
	})

	t.Run("scheme 2 takes the VAT number", func(t *testing.T) {
		pid := chorusProLegalOrgID(withScheme("2", func(p *org.Party) {
			p.TaxID = &tax.Identity{Country: "FR", Code: "44732829320"}
		}))
		assert.Equal(t, "FR44732829320", pid.Value)
	})

	t.Run("scheme 2 without a tax identity leaves the value empty", func(t *testing.T) {
		assert.Empty(t, chorusProLegalOrgID(withScheme("2", nil)).Value)
	})

	for _, scheme := range []string{"3", "6"} {
		t.Run("scheme "+scheme+" takes the country and a truncated name", func(t *testing.T) {
			pid := chorusProLegalOrgID(withScheme(scheme, func(p *org.Party) {
				p.TaxID = &tax.Identity{Country: "FR", Code: "44732829320"}
			}))
			// "Acme Industries Limited" is longer than the 16 characters the
			// scheme allows, and the cut lands on a space, which is kept.
			assert.Equal(t, "FRAcme Industries ", pid.Value)
			assert.Len(t, []rune(pid.Value), 18) // 2 for the country + 16
		})
	}

	t.Run("a short name is not padded", func(t *testing.T) {
		pid := chorusProLegalOrgID(withScheme("3", func(p *org.Party) {
			p.Name = "Acme"
			p.TaxID = &tax.Identity{Country: "FR"}
		}))
		assert.Equal(t, "FRAcme", pid.Value)
	})

	t.Run("the name is truncated by runes, not bytes", func(t *testing.T) {
		// Slicing bytes would cut a multi-byte character in half and produce
		// invalid UTF-8.
		pid := chorusProLegalOrgID(withScheme("3", func(p *org.Party) {
			p.Name = strings.Repeat("é", 20)
			p.TaxID = &tax.Identity{Country: "FR"}
		}))
		assert.Equal(t, "FR"+strings.Repeat("é", 16), pid.Value)
	})

	t.Run("scheme 3 without a country leaves the value empty", func(t *testing.T) {
		assert.Empty(t, chorusProLegalOrgID(withScheme("3", nil)).Value)
	})

	for _, scheme := range []string{"4", "5"} {
		t.Run("scheme "+scheme+" takes the legal identity", func(t *testing.T) {
			pid := chorusProLegalOrgID(withScheme(scheme, func(p *org.Party) {
				p.Identities = []*org.Identity{
					{Code: "not-legal"},
					{Scope: org.IdentityScopeLegal, Code: "RIDET-1"},
				}
			}))
			assert.Equal(t, "RIDET-1", pid.Value)
		})
	}

	t.Run("an unknown scheme yields the scheme alone", func(t *testing.T) {
		pid := chorusProLegalOrgID(withScheme("99", nil))
		assert.Equal(t, "99", pid.SchemeID)
		assert.Empty(t, pid.Value)
	})

	t.Run("no scheme at all", func(t *testing.T) {
		pid := chorusProLegalOrgID(&org.Party{Name: "Acme"})
		require.NotNil(t, pid)
		assert.Empty(t, pid.SchemeID)
		assert.Empty(t, pid.Value)
	})
}
