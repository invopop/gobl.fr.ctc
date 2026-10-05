// Package ubl registers the French CTC UBL contexts with the GOBL convert
// register. Import it for its side effects:
//
//	import _ "github.com/invopop/gobl.fr.ctc/ubl"
package ubl

import (
	"github.com/invopop/gobl"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/schema"
)

// Context keys.
const (
	KeyCIUS     cbc.Key = "ubl+peppol-fr-cius-v1"
	KeyExtended cbc.Key = "ubl+peppol-fr-extended-v1"
)

var invoiceSchema = schema.Lookup(bill.Invoice{})

var contexts = []*convert.Context{
	newContext(KeyCIUS, goblubl.ContextPeppolFranceCIUS, "UBL Peppol France CIUS"),
	newContext(KeyExtended, goblubl.ContextPeppolFranceExtended, "UBL Peppol France Extended"),
}

func newContext(key cbc.Key, c goblubl.Context, name string) *convert.Context {
	return &convert.Context{
		Key:       key,
		Name:      i18n.NewString(name),
		MIME:      "application/xml",
		Syntax:    "ubl",
		Countries: []l10n.Code{"FR"},
		Addons:    c.Addons,
		Import:    []schema.ID{invoiceSchema},
		Export:    []schema.ID{invoiceSchema},
	}
}

func init() {
	convert.Register(converter{})
}

// converter implements convert.Converter for the French UBL contexts,
// using gobl.ubl for the conversion itself.
type converter struct{}

func (converter) Contexts() []*convert.Context {
	return contexts
}

func (converter) Detect(in *convert.Input) cbc.Key {
	dc := goblubl.ReadDocumentContext(in)
	if dc.Err != nil {
		return cbc.KeyEmpty
	}
	if dc.Namespace != goblubl.NamespaceUBLInvoice && dc.Namespace != goblubl.NamespaceUBLCreditNote {
		return cbc.KeyEmpty
	}
	ctx := goblubl.FindContext(dc.CustomizationID, dc.ProfileID)
	if ctx == nil {
		return cbc.KeyEmpty
	}
	switch {
	case ctx.Is(goblubl.ContextPeppolFranceCIUS):
		return KeyCIUS
	case ctx.Is(goblubl.ContextPeppolFranceExtended):
		return KeyExtended
	}
	return cbc.KeyEmpty
}

func (converter) Import(_ cbc.Key, data []byte) (*gobl.Envelope, error) {
	doc, err := goblubl.Parse(data)
	if err != nil {
		return nil, err
	}
	inv, ok := doc.(*goblubl.Invoice)
	if !ok {
		return nil, goblubl.ErrUnsupportedDocumentType
	}
	return inv.Convert()
}

func (converter) Accepts(_ cbc.Key, _ *gobl.Envelope) bool {
	return true
}

func (converter) Export(key cbc.Key, env *gobl.Envelope) ([]byte, error) {
	ctx := goblubl.ContextPeppolFranceCIUS
	if key == KeyExtended {
		ctx = goblubl.ContextPeppolFranceExtended
	}
	doc, err := goblubl.Convert(env, goblubl.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return goblubl.Bytes(doc)
}
