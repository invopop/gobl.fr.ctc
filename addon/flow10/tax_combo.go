package flow10

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

func normalizeTaxCombo(tc *tax.Combo) {
	en16931.NormalizeTaxCombo(tc)
}

func taxComboRules() *rules.Set {
	return rules.For(new(tax.Combo),
		rules.Field("cat",
			rules.Assert("01", "Flux 10 e-reporting can only report French VAT; tax category must be VAT (G1.24)",
				is.In(tax.CategoryVAT),
			),
		),
		rules.Field("country",
			rules.Assert("02", "Flux 10 e-reporting can only report French VAT; tax country must not be a foreign VAT jurisdiction (G1.24)",
				is.Func("France or unset", taxComboCountryIsDomestic),
			),
		),
	)
}

// An empty Country means the document's own regime, which for Flow 10 is always France.
func taxComboCountryIsDomestic(v any) bool {
	country, ok := v.(l10n.TaxCountryCode)
	return ok && (country.Empty() || country == l10n.FR.Tax())
}
