package flow2

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
)

// ExtKeyDeliveryTerms is the delivery terms code (EXT-FR-FE-185) of the
// French CTC Extended profile.
const ExtKeyDeliveryTerms cbc.Key = "fr-ctc-delivery-terms"

var extDeliveryTerms = &cbc.Definition{
	Key: ExtKeyDeliveryTerms,
	Name: i18n.String{
		i18n.EN: "Incoterm Delivery Terms",
		i18n.FR: "Conditions de livraison (Incoterms)",
	},
	Desc: i18n.String{
		i18n.EN: here.Doc(`
			Code for the delivery or transport terms agreed between the buyer and
			the seller (EXT-FR-FE-185), restricted to the Incoterms and UNTDID 4053
			codes the French CTC Extended rules accept (BR-FREXT-CL-27).
		`),
		i18n.FR: here.Doc(`
			Code des conditions de livraison ou de transport convenues entre
			l'acheteur et le vendeur (EXT-FR-FE-185), limité aux codes Incoterms et
			UNTDID 4053 acceptés par les règles CTC Extended (BR-FREXT-CL-27).
		`),
	},
	Values: []*cbc.Definition{
		{Code: "1", Name: i18n.String{i18n.EN: "Delivery arranged by the supplier", i18n.FR: "Livraison organisée par le fournisseur"}},
		{Code: "2", Name: i18n.String{i18n.EN: "Delivery arranged by logistic service provider", i18n.FR: "Livraison organisée par le prestataire logistique"}},
		{Code: "CFR", Name: i18n.String{i18n.EN: "Cost and Freight", i18n.FR: "Coût et fret"}},
		{Code: "CIF", Name: i18n.String{i18n.EN: "Cost, Insurance and Freight", i18n.FR: "Coût, assurance et fret"}},
		{Code: "CIP", Name: i18n.String{i18n.EN: "Carriage and Insurance Paid to", i18n.FR: "Port payé, assurance comprise, jusqu'à"}},
		{Code: "CPT", Name: i18n.String{i18n.EN: "Carriage Paid To", i18n.FR: "Port payé jusqu'à"}},
		{Code: "DAP", Name: i18n.String{i18n.EN: "Delivered At Place", i18n.FR: "Rendu au lieu de destination"}},
		{Code: "DDP", Name: i18n.String{i18n.EN: "Delivered Duty Paid", i18n.FR: "Rendu droits acquittés"}},
		{Code: "DPU", Name: i18n.String{i18n.EN: "Delivered at Place Unloaded", i18n.FR: "Rendu au lieu de destination déchargé"}},
		{Code: "EXW", Name: i18n.String{i18n.EN: "Ex Works", i18n.FR: "À l'usine"}},
		{Code: "FAS", Name: i18n.String{i18n.EN: "Free Alongside Ship", i18n.FR: "Franco le long du navire"}},
		{Code: "FCA", Name: i18n.String{i18n.EN: "Free Carrier", i18n.FR: "Franco transporteur"}},
		{Code: "FOB", Name: i18n.String{i18n.EN: "Free On Board", i18n.FR: "Franco à bord"}},
	},
}
