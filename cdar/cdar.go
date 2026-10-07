// Package cdar converts French CTC Flow 6 lifecycle statuses and payments
// between GOBL and UN/CEFACT CDAR (Cross Domain Acknowledgement and
// Response) documents, using the CDAR structures from gobl.cii, and
// registers its formats with the GOBL convert register. Import it for its
// side effects:
//
//	import _ "github.com/invopop/gobl.fr.ctc/cdar"
package cdar

import (
	"github.com/invopop/gobl"
	cii "github.com/invopop/gobl.cii"
	"github.com/invopop/gobl.fr.ctc/addon/flow6"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/schema"
)

// Format keys.
const (
	KeyFlow6    cbc.Key = "cdar+peppol+fr-cdv-v1"
	KeyFlow6PPF cbc.Key = "cdar+fr-ppf-cdv-v1"
)

// ProfileIDPeppolFranceBilling is the Peppol France business process.
const ProfileIDPeppolFranceBilling = "urn:peppol:france:billing:regulated"

// Format defines a CDAR format: the identifiers its documents carry.
type Format struct {
	// Key identifies the format in the GOBL convert register.
	Key cbc.Key
	// Name of the format.
	Name i18n.String
	// GuidelineID identifies the format externally.
	GuidelineID string
	// OutputGuidelineID optionally specifies a different GuidelineID to
	// write into the CDAR XML's GuidelineParameter.
	OutputGuidelineID string
	// BusinessID identifies the business process externally.
	BusinessID string
	// OutputBusinessID optionally specifies a different BusinessID to write
	// into the CDAR XML's BusinessProcessParameter.
	OutputBusinessID string
	// VESID is the Validation Exchange Specification ID used for validation
	VESID string
}

// FormatFlow6 is used for French CTC Flow 6 CDARs addressed to an
// end-party: the GuidelineID is the "invoice" URN (BR-FR-CDV-02) with
// the REGULATED BusinessProcessParameter. The ack TypeCode (23 vs 305)
// is independent of the format — it follows the ProcessConditionCode's
// phase (see cdarAckTypeForCode).
var FormatFlow6 = Format{
	Key:  KeyFlow6,
	Name: i18n.NewString("CDAR Peppol France CDV"),
	// GuidelineID is the Peppol document-type customization used for the
	// busdox SMP lookup / SBD (the receiver registers this exact id);
	// OutputGuidelineID keeps the internal CDV guideline (BR-FR-CDV-02) in
	// the CDAR XML's GuidelineParameter. They differ: the network identifies
	// the doc by the Peppol id, the XML carries the cpro guideline.
	GuidelineID:       "urn:peppol:france:billing:cdv:1.0",
	OutputGuidelineID: CDARGuidelineInvoice,
	// BusinessID is the busdox SBD/SMP process id the receiver registers its
	// CDV service under (cenbii-procid-ubl scheme); OutputBusinessID keeps the
	// CDAR XML's BusinessProcessParameter at the CDV MDT-2 "REGULATED" value.
	BusinessID:       ProfileIDPeppolFranceBilling,
	OutputBusinessID: "REGULATED",
	VESID:            "fr.ctc:cdar:1.4.0-03",
}

// FormatFlow6PPF is used for French CTC Flow 6 CDAR copies sent to
// the PPF: the GuidelineID is the einvoicingF2 URN per BR-FR-CDV-02, no
// BusinessProcessParameter is emitted, and the single recipient is the
// PPF party (0000 / 0238 / DFH).
var FormatFlow6PPF = Format{
	Key:         KeyFlow6PPF,
	Name:        i18n.NewString("CDAR France PPF CDV"),
	GuidelineID: CDARGuidelinePPF,
	VESID:       "fr.ctc:cdar:1.4.0-03",
}

// routing carries the envelope's transport addresses — the SBD From / To the
// Peppol layer received, as fully-qualified participant URIs (e.g.
// "iso6523-actorid-upis::0225:code"). They are recorded verbatim on
// Head.From/To and are also read to hydrate a business party's inbox when the
// CDV body omits it (see hydratePartyInboxes, which also tolerates a bare
// "scheme:code").
type routing struct {
	from, to cbc.URI
}

type options struct {
	sender  *org.Party
	routing routing
}

// Option configures an export or import.
type Option func(*options)

// WithSender pins the *org.Party emitted as the CDAR
// ExchangedDocument/SenderTradeParty (MDT-21). Use this to carry the
// dematerialisation platform's identity (Name + GlobalID + Inbox +
// RoleCode) on the wire when it isn't anonymous. When unset, the writer
// emits a bare <ram:RoleCode>WK</ram:RoleCode> — matching the anonymous-
// platform pattern used throughout the official UC1 corpus.
func WithSender(p *org.Party) Option {
	return func(o *options) {
		o.sender = p
	}
}

// WithRouting supplies the transport addresses a received document was routed
// with — the Peppol SBD From / To — as fully-qualified participant URIs (e.g.
// "iso6523-actorid-upis::0225:code"). On import they are recorded verbatim on
// the envelope's Head.From / Head.To, and also populate a party inbox the CDAR
// body may omit (carried at the SBD layer instead), so the result satisfies
// BR-FR-CDV-08.
func WithRouting(from, to cbc.URI) Option {
	return func(o *options) {
		o.routing = routing{from: from, to: to}
	}
}

func newOptions(opts []Option) *options {
	o := new(options)
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

// Export converts the GOBL envelope's status or payment into a CDAR in the
// format. The envelope's Head.From / Head.To routing URIs steer the CDAR
// issuer and recipient slots.
func Export(env *gobl.Envelope, f Format, opts ...Option) (*cii.CDAR, error) {
	o := newOptions(opts)
	var from, to cbc.URI
	if env.Head != nil {
		from, to = env.Head.From, env.Head.To
	}
	switch doc := env.Extract().(type) {
	case *bill.Status:
		return newCDAR(doc, f, o.sender, from, to)
	case *bill.Payment:
		return newCDARFromPayment(doc, f, o.sender, from, to)
	}
	return nil, cii.ErrUnsupportedDocumentType
}

// ExportStatus converts the status into a CDAR in the format, without an
// envelope.
//
// The status must carry the flow6 extensions (fr-ctc-flow6-status on
// each line, fr-ctc-flow6-role on the parties) — run Calculate on the
// enclosing envelope with the fr-ctc-flow6-v1 addon declared so the
// normalizer derives them.
func ExportStatus(st *bill.Status, f Format, opts ...Option) (*cii.CDAR, error) {
	return newCDAR(st, f, newOptions(opts).sender, "", "")
}

// ExportPayment converts the payment into a CDAR in the format, without an
// envelope.
func ExportPayment(pmt *bill.Payment, f Format, opts ...Option) (*cii.CDAR, error) {
	return newCDARFromPayment(pmt, f, newOptions(opts).sender, "", "")
}

// Import converts the CDAR into a GOBL envelope holding a status or, for the
// payment lifecycle codes (211 / 212), a payment.
func Import(doc *cii.CDAR, opts ...Option) (*gobl.Envelope, error) {
	o := newOptions(opts)
	res, err := importDocument(doc, o.routing)
	if err != nil {
		return nil, err
	}

	env := gobl.NewEnvelope()
	// A parsed document is one we received: its transport addresses are the
	// ones the Peppol layer routed it with (who sent it → who received it).
	// Set Head.From / Head.To from those args verbatim, BEFORE calculation, so
	// GOBL respects them — normalizeRouting only fills empty routing fields, so
	// it won't overwrite them with the document-derived, OUTGOING-direction
	// guess (supplier → customer) that is wrong for a received document.
	env.Head.From = o.routing.from
	env.Head.To = o.routing.to
	if err := env.Insert(res); err != nil {
		return nil, err
	}
	return env, nil
}

// ImportStatus converts the CDAR into a *bill.Status without going through
// an envelope.
//
// The returned status carries the CDAR codes on the flow6 extensions
// (fr-ctc-flow6-status / -reason / -action) with the addon declared;
// the GOBL-level fields they derive (Status.Type, line and reason keys)
// are filled by the flow6 normalizer when the document is calculated —
// wrap it in an envelope (gobl.Envelop / Envelope.Insert) or run
// Calculate to complete it.
func ImportStatus(doc *cii.CDAR) (*bill.Status, error) {
	return goblStatusFromCDAR(doc, routing{})
}

// ImportPayment converts a CDAR carrying a payment lifecycle code (211 /
// 212) into a *bill.Payment without going through an envelope. Like
// ImportStatus, the returned document carries the CDAR codes on the flow6
// extensions; wrap it in an envelope or run Calculate to complete the
// derived fields.
func ImportPayment(doc *cii.CDAR) (*bill.Payment, error) {
	return goblPaymentFromCDAR(doc, routing{})
}

var (
	statusSchema  = schema.Lookup(bill.Status{})
	paymentSchema = schema.Lookup(bill.Payment{})
)

func init() {
	convert.Register(converter{})
}

// converter implements convert.Converter for the Flow 6 CDAR formats.
type converter struct{}

func (converter) Formats() []*convert.Format {
	list := make([]*convert.Format, 0, 2)
	for _, f := range []Format{FormatFlow6, FormatFlow6PPF} {
		list = append(list, &convert.Format{
			Key:       f.Key,
			Name:      f.Name,
			MIME:      "application/xml",
			Syntax:    "cdar",
			Countries: []l10n.Code{l10n.FR},
			Addons:    []cbc.Key{flow6.V1},
			Import:    []schema.ID{statusSchema, paymentSchema},
			Export:    []schema.ID{statusSchema, paymentSchema},
		})
	}
	return list
}

// Detect claims every CDAR: copies for the PPF by their guideline, and any
// other as the end-party Flow 6 format.
func (converter) Detect(in *convert.Input) cbc.Key {
	h := cii.ReadHeader(in)
	if h.Err != nil || h.Namespace != cii.NamespaceCDARRSM {
		return cbc.KeyEmpty
	}
	if h.GuidelineID == FormatFlow6PPF.GuidelineID {
		return KeyFlow6PPF
	}
	return KeyFlow6
}

func (converter) Import(_ cbc.Key, data []byte) (*gobl.Envelope, error) {
	doc, err := cii.Decode(data)
	if err != nil {
		return nil, err
	}
	cdar, ok := doc.(*cii.CDAR)
	if !ok {
		return nil, cii.ErrUnsupportedDocumentType
	}
	return Import(cdar)
}

func (converter) Accepts(_ cbc.Key, _ *gobl.Envelope) bool {
	return true
}

func (converter) Export(key cbc.Key, env *gobl.Envelope) ([]byte, error) {
	f := FormatFlow6
	if key == KeyFlow6PPF {
		f = FormatFlow6PPF
	}
	doc, err := Export(env, f)
	if err != nil {
		return nil, err
	}
	return cii.Encode(doc)
}
