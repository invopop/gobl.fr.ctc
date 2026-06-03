// Package dgfip defines the French tax authority (Direction Générale des
// Finances Publiques) billing-mode code list used by the French CTC
// (Continuous Transaction Control) reform. It is shared across the
// flow-specific addons (Flow 2 clearance and Flow 10 e-reporting), so it lives
// in its own leaf package that those addons embed in their extension lists.
package dgfip

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
)

// ExtKeyBillingMode is the DGFiP "Cadre de Facturation" code that describes
// the nature of the document (Biens / Services / Mixte) and the payment
// context. Required on Flow 2 clearance invoices and Flow 10 B2B reporting
// invoices.
const ExtKeyBillingMode cbc.Key = "fr-ctc-billing-mode"

// Billing mode codes. The prefix denotes invoice nature (B = goods, S =
// services, M = mixed); the numeric suffix encodes the payment context
// (1 = deposit, 2 = already paid, 4 = final after down payment,
// 5 = subcontractor, 6 = co-contractor, 7 = e-reporting).
const (
	BillingModeB1 cbc.Code = "B1"
	BillingModeB2 cbc.Code = "B2"
	BillingModeB4 cbc.Code = "B4"
	BillingModeB7 cbc.Code = "B7"
	BillingModeS1 cbc.Code = "S1"
	BillingModeS2 cbc.Code = "S2"
	BillingModeS4 cbc.Code = "S4"
	BillingModeS5 cbc.Code = "S5"
	BillingModeS6 cbc.Code = "S6"
	BillingModeS7 cbc.Code = "S7"
	BillingModeM1 cbc.Code = "M1"
	BillingModeM2 cbc.Code = "M2"
	BillingModeM4 cbc.Code = "M4"
)

// ExtBillingMode is the shared billing-mode extension definition. Flow addons
// that use it include it in their AddonDef.Extensions list; registration is
// idempotent by key, so it is safe for more than one addon to declare it.
var ExtBillingMode = &cbc.Definition{
	Key: ExtKeyBillingMode,
	Name: i18n.String{
		i18n.EN: "Billing Mode",
		i18n.FR: "Cadre de Facturation",
	},
	Desc: i18n.String{
		i18n.EN: here.Doc(`
			Code used to describe the billing framework of the invoice. The
			billing mode indicates the nature of goods/services and the payment
			context.

			Code prefixes indicate the invoice nature:
			- "B": Goods invoice (Biens)
			- "S": Services invoice
			- "M": Mixed/dual invoice (goods and services that are not accessory
			  to each other)

			The numeric suffix indicates the payment type (1=deposit,
			2=already paid, 4=final after down payment, 5=subcontractor,
			6=co-contractor, 7=e-reporting).
		`),
		i18n.FR: here.Doc(`
			Code utilisé pour décrire le cadre de facturation de la facture. Le
			mode de facturation indique la nature des biens/services et le
			contexte de paiement.

			Les préfixes de code indiquent la nature de la facture :
			- "B" : Facture de biens
			- "S" : Facture de services
			- "M" : Facture mixte (biens et services qui ne sont pas accessoires
			  l'un de l'autre)

			Le suffixe numérique indique le type de paiement (1=dépôt,
			2=déjà payée, 4=définitive après acompte, 5=sous-traitant,
			6=cotraitant, 7=e-reporting).
		`),
	},
	Values: []*cbc.Definition{
		{Code: BillingModeB1, Name: i18n.String{i18n.EN: "Goods - Deposit invoice", i18n.FR: "Biens - Facture de dépôt"}},
		{Code: BillingModeB2, Name: i18n.String{i18n.EN: "Goods - Already paid invoice", i18n.FR: "Biens - Facture déjà payée"}},
		{Code: BillingModeB4, Name: i18n.String{i18n.EN: "Goods - Final invoice (after down payment)", i18n.FR: "Biens - Facture définitive (après acompte)"}},
		{Code: BillingModeB7, Name: i18n.String{i18n.EN: "Goods - E-reporting (VAT already collected)", i18n.FR: "Biens - E-reporting (TVA déjà collectée)"}},
		{Code: BillingModeS1, Name: i18n.String{i18n.EN: "Services - Deposit invoice", i18n.FR: "Services - Facture de dépôt"}},
		{Code: BillingModeS2, Name: i18n.String{i18n.EN: "Services - Already paid invoice", i18n.FR: "Services - Facture déjà payée"}},
		{Code: BillingModeS4, Name: i18n.String{i18n.EN: "Services - Final invoice (after down payment)", i18n.FR: "Services - Facture définitive (après acompte)"}},
		{Code: BillingModeS5, Name: i18n.String{i18n.EN: "Services - Subcontractor invoice", i18n.FR: "Services - Facture de sous-traitance"}},
		{Code: BillingModeS6, Name: i18n.String{i18n.EN: "Services - Co-contractor invoice", i18n.FR: "Services - Facture de cotraitance"}},
		{Code: BillingModeS7, Name: i18n.String{i18n.EN: "Services - E-reporting (VAT already collected)", i18n.FR: "Services - E-reporting (TVA déjà collectée)"}},
		{Code: BillingModeM1, Name: i18n.String{i18n.EN: "Mixed - Deposit invoice", i18n.FR: "Mixte - Facture de dépôt"}},
		{Code: BillingModeM2, Name: i18n.String{i18n.EN: "Mixed - Already paid invoice", i18n.FR: "Mixte - Facture déjà payée"}},
		{Code: BillingModeM4, Name: i18n.String{i18n.EN: "Mixed - Final invoice (after down payment)", i18n.FR: "Mixte - Facture définitive (après acompte)"}},
	},
}
