# GOBL ↔ CDAR (Flow 6) mapping reference

How GOBL documents with the `fr-ctc-flow6-v1` addon round-trip to and from
the French CDAR (*Compte rendu d'Avis et de Réception*, XP Z12-012 v1.4)
lifecycle messages (CDV) exchanged between *Plateformes Agréées* and the
PPF.

Two GOBL documents carry a CDV:

| GOBL document | Process codes (MDT-105) |
|---|---|
| `bill.Status` | 200–210, 213 |
| `bill.Payment` | 211 (Paiement transmis), 212 (Encaissée) |

The `cdar` package converts both ways, on top of the CDAR XML structures in
`gobl.cii`. Importing the package registers it with the GOBL `convert`
register:

```go
import _ "github.com/invopop/gobl.fr.ctc/cdar"
```

---

## 1. API and formats

```go
cdar.Export(env, cdar.FormatCDV, opts...)  // envelope → *cii.CDAR
cdar.ExportStatus(st, format, opts...)
cdar.ExportPayment(pmt, format, opts...)

cdar.Import(doc, opts...)                   // *cii.CDAR → envelope
cdar.ImportStatus(doc)
cdar.ImportPayment(doc)
```

`Import` returns a `bill.Payment` when the process code is 211 or 212, and a
`bill.Status` otherwise. `ImportStatus` and `ImportPayment` return the
document with only the CDAR codes on its extensions: calculate it (or insert
it in an envelope) to let the flow6 normalizer derive the GOBL keys.

| Format | Key | XML GuidelineParameter (MDT-3) | BusinessProcess (MDT-2) | Recipient |
|---|---|---|---|---|
| `FormatCDV` | `cdar+fr-ctc-cdv-v1` | `urn.cpro.gouv.fr:1p0:CDV:invoice` | `REGULATED` | the counterparty |
| `FormatCDVPPF` | `cdar+fr-ctc-cdv-v1+ppf` | `urn.cpro.gouv.fr:1p0:CDV:einvoicingF2` | none | the PPF |

`FormatCDV` is identified on the Peppol network by the document type
`urn:peppol:france:billing:cdv:1.0` and the process
`urn:peppol:france:billing:regulated`; the XML keeps the CDV guideline.
Both formats validate against the `fr.ctc:cdar:1.4.0-03` schematron.

On import, a CDAR with the PPF guideline is detected as `FormatCDVPPF`, and
any other CDAR as `FormatCDV`.

Options:

| Option | Effect |
|---|---|
| `WithSender(p)` | Export: the `SenderTradeParty` (MDT-21). Defaults to a bare `WK` party. Use `cdar.PPFPlatformParty(matricule)` for copies to the PPF, which rejects a sender without a GlobalID. |
| `WithRouting(from, to)` | Import: the Peppol SBD addresses the document arrived with. Recorded on `Head.From` / `Head.To`, and used to fill a party inbox the CDV body omits (see §2). |

On export, the envelope's `Head.From` / `Head.To` can also pick the issuer
and recipient slots (see §2).

### Ack type (MDT-77)

The acknowledgement `TypeCode` follows the process code, not the format:

| Process codes | Phase | `TypeCode` |
|---|---|---|
| 200, 201, 202, 203, 213 | transmission (platform-issued) | 305 |
| 204–212 | treatment (business-issued) | 23 |

---

## 2. Parties

GOBL holds only the two business parties, `Supplier` (role `SE`) and
`Customer` (role `BY`). The platform (`WK`) and PPF (`DFH`) parties are
transport detail: the exporter adds them and the importer drops them.

### Export

| CDAR slot | Source |
|---|---|
| `SenderTradeParty` (MDT-21) | `WithSender`, or a bare `WK` party |
| `IssuerTradeParty` (MDG-16) | by process code, below |
| `RecipientTradeParty` (MDG-23) | by process code, below; always the PPF under `FormatCDVPPF` |
| referenced `IssuerTradeParty/GlobalID` (MDT-129) | the line document's SIREN identity, else the `Supplier`'s SIREN |

| Document | Issuer | Recipient |
|---|---|---|
| Status 204–208, 210 (declared by the buyer) | `Customer` | `Supplier` |
| Status 209 (declared by the seller) | `Supplier` | `Customer` |
| Status 200–203, 213 (platform phase) | the sender | `Supplier` |
| Payment `receipt` (212) | `Supplier` | `Customer` |
| Payment `advice` (211) | `Customer` | `Supplier` |

For treatment-phase codes, an envelope `Head.From` / `Head.To` that matches
one of the business parties' endpoints overrides these defaults.

The PPF party is fixed: GlobalID `0000` under scheme `0238`, role `DFH`.
MDT-129 always names the invoice's seller, also for self-billed invoices
(confirmed against PPF).

Each party is written as:

| CDAR field | `org.Party` |
|---|---|
| `Name` | `Name` |
| `RoleCode` | `ext[fr-ctc-flow6-role]` |
| `GlobalID[@schemeID]` | each identity with an `iso-scheme-id` extension |
| `URIUniversalCommunication/URIID[@schemeID]` | first inbox (`scheme`, `code`) |

### Import

Trade parties are assigned by role: `SE` → `Supplier`, `BY` → `Customer`.
Other roles are dropped.

- With no `SE` party, the MDT-129 issuer becomes the `Supplier`; an `SE`
  party without a SIREN gets the MDT-129 one.
- The MDT-129 SIREN is also kept on the line document's identities, so the
  invoice resolves by its seller even when the buyer issued the status.
- A CDV need not repeat a party's electronic address, which travels in the
  SBD. With `WithRouting`, each address fills the inbox of the party whose
  SIREN it carries, then any party still missing one, so the result meets
  BR-FR-CDV-08.

### Normalizer defaults

The flow6 normalizer sets `fr-ctc-flow6-role` to `SE` on the `Supplier` and
`BY` on the `Customer` when it is empty.

---

## 3. Process codes

The process code lives on `fr-ctc-flow6-status`: on the status line for a
`bill.Status`, and on the payment for a `bill.Payment`. The normalizer
derives it from the GOBL keys, or derives the keys from it when only the
code is set (as after an import).

### `bill.Status`

| `Status.Type` | `StatusLine.Key` | Code | Label (MDT-106) |
|---|---|---|---|
| `update` | `issued` | **200** | Déposée |
| `response` | `issued` | **201** | Émise par la plateforme |
| `response` | `acknowledged` | **202** | Reçue |
| `response` | `other` | **203** | Mise à disposition |
| `response` | `processing` | **204** | Prise en charge |
| `response` | `accepted` | **205** | Approuvée |
| `response` | `rejected` | **206** | Approuvée partiellement |
| `response` | `rejected` | **207** | En litige |
| `response` | `querying` | **208** | Suspendue |
| `update` | `other` | **209** | Complétée |
| `response` | `rejected` | **210** (default for `rejected`) | Refusée |
| `response` | `error` | **213** | Rejetée |

`rejected` covers 206, 207 and 210: set the code explicitly for 206 or 207.
With no `Type` and no code, `issued` defaults to `update`, and the other
keys to `response`.

### `bill.Payment`

| `Payment.Type` | Code | Condition (MDT-207) default |
|---|---|---|
| `advice` | **211** | `MPA` |
| `receipt` | **212** | `MEN` |

`request` is not a Flow 6 event.

### Referenced document status (MDT-88)

Written only for the pairings attested in the official corpus:

| Code | 200 | 202 | 203 | 204 | 205 | 207 | 211 | 212 |
|---|---|---|---|---|---|---|---|---|
| `StatusCode` | 10 | 43 | 48 | 45 | 1 | 46 | 47 | 47 |

---

## 4. Status line content

Each `AcknowledgementDocument/ReferenceReferencedDocument` maps to one
status line.

| CDAR field | `bill.StatusLine` |
|---|---|
| `ProcessConditionCode` (MDT-105) | `ext[fr-ctc-flow6-status]` |
| `ProcessCondition` (MDT-106) | label from §3, written on export |
| `IssuerAssignedID` (MDT-87) | `Doc.Series` + `Doc.Code` |
| `FormattedIssueDateTime` (MDT-100) | `Doc.IssueDate` |
| `TypeCode` (MDT-91) | `Doc.ext[untdid-document-type]` |
| `ReceiptDateTime` (MDT-95) | `Date` |
| `SpecifiedDocumentStatus[]` | `Reasons` × `Actions`, below |

`SpecifiedDocumentStatus` entries pair every reason with every action; a
line with only reasons or only actions gets one entry each.

| CDAR field | GOBL |
|---|---|
| `ReasonCode` (MDT-113) | `Reason.ext[fr-ctc-flow6-reason]` |
| `Reason` (MDT-114) | `Reason.Description` |
| `IncludedNote/Content` (MDT-126) | `Reason.Description`, written on export only |
| `SpecifiedDocumentCharacteristic[]` | `Reason.Faults` |
| `RequestedActionCode` (MDT-121) | `Action.ext[fr-ctc-flow6-action]` |
| `RequestedAction` | `Action.Description` |

Each fault is one `SpecifiedDocumentCharacteristic`:

| CDAR field | `bill.Fault` |
|---|---|
| `TypeCode` (MDT-207) | `Code`, when it is a status condition code (§5) |
| `Name` (MDT-211) | `Message` (or the `Code`, when it is not a condition code and the message is empty) |
| `Location` (MDT-213) | first `Paths` entry |

On import, the characteristic's data name, business-term ID and typed value
compose the fault message, e.g. `Taux TVA (BT-152): 10.00%`. A
characteristic in an entry with no `ReasonCode` is attached to a reason
with key `other`.

PPF requires a free-text comment (MDT-126) on 208 and 210, so those need a
reason description.

---

## 5. Reasons, actions and conditions

### Reasons

`fr-ctc-flow6-reason` holds the exact CDAR reason code. Each
`bill.Reason.Key` covers a set of codes: the normalizer defaults to the first
one and keeps a caller-set code from the same set. On import, the key is
derived from the code.

| `Reason.Key` | CDAR reason codes (first is the default) |
|---|---|
| `finance-terms` | `COORD_BANC_ERR` |
| `other` | `AUTRE` |
| `legal` | `NON_CONFORME`, `TX_TVA_ERR`, `REJ_SEMAN`, `REJ_COH`, `REJ_CONT_B2G`, `IRR_VIDE_F`, `IRR_TYPE_F`, `IRR_SYNTAX`, `IRR_TAILLE_PJ`, `IRR_NOM_PJ`, `IRR_VID_PJ`, `IRR_EXT_DOC`, `IRR_TAILLE_F`, `IRR_ANTIVIRUS` |
| `not-recognized` | `DOUBLON`, `TRANSAC_INC`, `EMMET_INC`, `CONTRAT_TERM`, `DOUBLE_FACT`, `REJ_UNI` |
| `unknown-receiver` | `DEST_INC`, `NON_TRANSMISE`, `ROUTAGE_ERR` |
| `references` | `CMD_ERR`, `JUSTIF_ABS`, `DEST_ERR`, `ADR_ERR`, `SIRET_ERR`, `CODE_ROUTAGE_ERR`, `REF_CT_ABSENT`, `REF_ERR`, `REJ_ADR`, `REJ_REF_PJ`, `REJ_ASS_PJ` |
| `prices` | `PU_ERR`, `MONTANTTOTAL_ERR`, `CALCUL_ERR`, `REM_ERR` |
| `quantity` | `QTE_ERR` |
| `items` | `ART_ERR` |
| `payment-terms` | `MODPAI_ERR` |
| `quality` | `QUALITE_ERR` |
| `delivery` | `LIVR_INCOMP` |

Allowed reason codes per process code (BR-FR-CDV-CL-09); other codes don't
restrict them:

| Code | Allowed `fr-ctc-flow6-reason` |
|---|---|
| 200 | `NON_TRANSMISE` |
| 206 | `AUTRE`, `CMD_ERR`, `SIRET_ERR`, `CODE_ROUTAGE_ERR`, `REF_CT_ABSENT`, `REF_ERR`, `PU_ERR`, `REM_ERR`, `QTE_ERR`, `ART_ERR`, `MODPAI_ERR`, `QUALITE_ERR`, `LIVR_INCOMP` |
| 207 | `AUTRE`, `COORD_BANC_ERR`, `TX_TVA_ERR`, `MONTANTTOTAL_ERR`, `CALCUL_ERR`, `NON_CONFORME`, `DOUBLON`, `DEST_INC`, `DEST_ERR`, `TRANSAC_INC`, `EMMET_INC`, `CONTRAT_TERM`, `DOUBLE_FACT`, `CMD_ERR`, `ADR_ERR`, `SIRET_ERR`, `CODE_ROUTAGE_ERR`, `REF_CT_ABSENT`, `REF_ERR`, `PU_ERR`, `REM_ERR`, `QTE_ERR`, `ART_ERR`, `MODPAI_ERR`, `QUALITE_ERR`, `LIVR_INCOMP` |
| 208 | `JUSTIF_ABS`, `COORD_BANC_ERR`, `CMD_ERR`, `SIRET_ERR`, `CODE_ROUTAGE_ERR`, `REF_CT_ABSENT`, `REF_ERR` |
| 210 | `TX_TVA_ERR`, `MONTANTTOTAL_ERR`, `CALCUL_ERR`, `NON_CONFORME`, `DOUBLON`, `DEST_ERR`, `TRANSAC_INC`, `EMMET_INC`, `CONTRAT_TERM`, `DOUBLE_FACT`, `CMD_ERR`, `ADR_ERR`, `REF_CT_ABSENT` |
| 213 | `MONTANTTOTAL_ERR`, `CALCUL_ERR`, `DOUBLON`, `DEST_INC`, `ADR_ERR`, `REJ_SEMAN`, `REJ_UNI`, `REJ_COH`, `REJ_ADR`, `REJ_CONT_B2G`, `REJ_REF_PJ`, `REJ_ASS_PJ` |

### Actions (MDT-121)

| `Action.Key` | `fr-ctc-flow6-action` |
|---|---|
| `none` | `NOA` |
| `provide` | `PIN` |
| `reissue` | `NIN` |
| `credit-full` | `CNF` |
| `credit-partial` | `CNP` |
| `credit-amount` | `CNA` |
| `other` | `OTH` |

### Conditions (MDT-207)

`fr-ctc-flow6-condition` holds a CDAR characteristic type code.

| Where | Codes |
|---|---|
| `bill.Reason` under a status | `CBB`, `DIV`, `DVA`, `MAJ` (field corrections), `MAP`, `MAPTTC`, `MNA`, `MNATTC` (approved / rejected amounts), `ESC`, `RAB`, `REM` (discounts) |
| `bill.Payment` | `MEN` (encaissé), `MPA` (payé), `RAP` (reste à payer) |

On a reason, the condition classifies its main characteristic, defaulting
to `CBB` for `finance-terms` and `DIV` otherwise. The characteristics
themselves are the reason's faults (§4): a `DIV` fault with the wrong value
and a `DVA` fault with the expected one, for example.

---

## 6. Payments

A `bill.Payment` has one line, for the invoice it settles. Its amount is one
`SpecifiedDocumentStatus` whose characteristics carry the payment's
condition (`MEN` by default):

| `bill.PaymentLine` | CDAR |
|---|---|
| `Document` | referenced document, as in §4 |
| `Tax` VAT rates | one characteristic per rate: `ValueAmount` = rate base + tax (MDT-215), `ValuePercent` (MDT-224) |
| `Amount`, when there is no tax breakdown | one characteristic with `ValueAmount` only |
| `Refund` | amounts negated |
| `Due` | not exported |

PPF rejects a cashed amount that isn't split by VAT rate
(`REJ_ENCAISSEMENT`), so a `receipt` must carry the VAT breakdown. An exempt
rate is written as `0.00`, like a zero rate, since the CDV can't tell them
apart.

On import, `MEN` / `MPA` amounts become the line `Amount` (a negative one
sets `Refund`), `RAP` becomes `Due`, and the condition found is set on the
payment. A CDV carries no payment method, so the importer adds a bare
record for the line amount.

---

## 7. PPF copies

Under `FormatCDVPPF`, the exporter also applies the corrections PPF enforces
beyond the schematron:

- `ReferenceTypeCode` (MDT-97) set to the einvoicingF2 guideline on every
  referenced document.
- `ExchangedDocument/Name` (MDT-5) defaults to the status label.
- `FormattedIssueDateTime` (MDT-100) written in format 204 (`CCYYMMDDHHMMSS`).
- `ReceiptDateTime` (MDG-34) defaults to the acknowledgement's issue date.

---

## 8. Validation

Rules run when the document declares `fr-ctc-flow6-v1`. Their codes are
prefixed with `GOBL-FR-CTC-FLOW6-`, e.g. `GOBL-FR-CTC-FLOW6-BILL-STATUS-03`.
The converter's `Accepts` rejects a document without the addon.

### `BILL-STATUS`

| Rule | Spec | Checks |
|---|---|---|
| 01 | | `Type` is `response` or `update` |
| 02 | | `fr-ctc-flow6-status` on the status is a status code (not 211/212) |
| 26 | BR-FR-CDV-03 | `Code` present (MDT-4) |
| 03 | BR-FR-CDV-13 | `Supplier` present |
| 04 / 08 | BR-FR-CDV-08 | `Supplier` / `Customer` has an inbox unless its role is `WK` or `DFH` |
| 05 / 09 | BR-FR-CDV-CL-03/04 | `Supplier` / `Customer` has a role |
| 06 | | `Supplier` has a SIREN identity (0002) |
| 07 | BR-FR-CDV-CL-04 | `Customer` present on 204–210 |
| 10 | | `Customer` has an identity in the Flow 6 scheme list |
| 11 | | exactly one line |
| 12–14 | BR-FR-CDV-10/11 | line `Doc` with code and issue date |
| 25 | MDT-91 | line `Doc` has a valid `untdid-document-type` |
| 15 | | line key is a Flow 6 event |
| 16 | BR-FR-CDV-15 | reasons present for `rejected`, `querying`, `error` |
| 17–22 | BR-FR-CDV-CL-09 | reason codes allowed for 200, 206, 207, 208, 210, 213 |
| 27 | MDT-126 | reason description present on 208 and 210 |
| 23 / 24 | | line key consistent with `response` / `update` |

### `BILL-PAYMENT`

| Rule | Spec | Checks |
|---|---|---|
| 01 | | `Type` is `advice` or `receipt` |
| 19 | BR-FR-CDV-03 | `Code` present (MDT-4) |
| 02 / 05 | BR-FR-CDV-13, CL-04 | `Supplier` / `Customer` present |
| 03 / 06 | | `Supplier` / `Customer` has a SIREN identity (0002) |
| 04 / 07 / 08 | | supplier, customer and payee identities in the Flow 6 scheme list |
| 09 | | exactly one line |
| 10–12 | BR-FR-CDV-10/11 | line document with code and issue date |
| 17 | MDT-91 | line document has a valid `untdid-document-type` |
| 18 | P1.18 | a `receipt` line has a VAT breakdown (exempt allowed) |
| 13 / 15 / 16 | | `fr-ctc-flow6-status` is 211 for `advice`, 212 for `receipt` |
| 14 | | `fr-ctc-flow6-condition` is `MEN`, `MPA` or `RAP` |

### Reasons, actions, parties

| Rule | Spec | Checks |
|---|---|---|
| `BILL-REASON-01` | | `fr-ctc-flow6-reason` is a known code |
| `BILL-REASON-02` | | `fr-ctc-flow6-condition` is a status condition code |
| `BILL-ACTION-01` | MDT-121 | `fr-ctc-flow6-action` is a known code |
| `ORG-PARTY-01` | | `fr-ctc-flow6-role` is in the UNCL 3035 subset (`SE`, `BY`, `WK`, `DFH`, `AB`, `SR`, `DL`, `PE`, `PR`, `II`, `IV`) |
| `ORG-PARTY-02` | BR-FR-09/10 | SIRET and SIREN identities agree |
| `ORG-PARTY-03`–`05` | BR-FR-CO-10 | identities have a unique `iso-scheme-id` from 0002, 0009, 0223, 0224, 0226–0229, 0238 |
| `ORG-PARTY-06` / `07` | BR-FR-26 / 24 | private ID (0224) is at most 100 characters of `A-Z a-z 0-9 + - _ /` |
| `ORG-PARTY-08` | BR-FR-23 | 0225 inbox code format |

---

## 9. Examples

`cdar/testdata/convert` holds a GOBL input and the exported XML for each
case: `cdar/` for end-party copies (204–208, 210, 212, 213) and `cdar-PPF/`
for PPF copies (200, 212).

A buyer disputing an invoice's VAT rate (207), from
`cdar/status-207-litige.json`:

```json
{
  "$schema": "https://gobl.org/draft-0/bill/status",
  "$addons": ["fr-ctc-flow6-v1"],
  "type": "response",
  "issue_date": "2025-07-01",
  "code": "STATUS-207",
  "supplier": {
    "name": "VENDEUR",
    "identities": [{ "code": "100000009", "ext": { "iso-scheme-id": "0002" } }],
    "inboxes": [{ "key": "peppol", "scheme": "0225", "code": "100000009_PEP" }],
    "ext": { "fr-ctc-flow6-role": "SE" }
  },
  "customer": {
    "name": "ACHETEUR",
    "identities": [{ "code": "200000008", "ext": { "iso-scheme-id": "0002" } }],
    "inboxes": [{ "key": "peppol", "scheme": "0225", "code": "200000008_PEP" }],
    "ext": { "fr-ctc-flow6-role": "BY" }
  },
  "lines": [
    {
      "key": "rejected",
      "date": "2025-07-01",
      "doc": {
        "code": "F202500003",
        "issue_date": "2025-07-01",
        "ext": { "untdid-document-type": "380" }
      },
      "reasons": [
        {
          "key": "legal",
          "description": "Motif TX_TVA_ERR",
          "faults": [
            { "code": "DIV", "message": "Taux TVA (BT-152): 10.00%", "paths": ["/rsm:CrossIndustryInvoice/ram:ApplicableTradeTax/ram:RateApplicablePercent"] },
            { "code": "DVA", "message": "Taux TVA (BT-152): 20.00%", "paths": ["/rsm:CrossIndustryInvoice/ram:ApplicableTradeTax/ram:RateApplicablePercent"] }
          ],
          "ext": { "fr-ctc-flow6-reason": "TX_TVA_ERR" }
        }
      ],
      "actions": [{ "key": "reissue", "description": "Créer une facture rectificative" }],
      "ext": { "fr-ctc-flow6-status": "207" }
    }
  ]
}
```

It exports with ack `TypeCode` 23, the buyer as issuer and the seller as
recipient, and one `SpecifiedDocumentStatus` carrying `TX_TVA_ERR`, `NIN`
and the `DIV` / `DVA` characteristics.

A seller confirming payment (212) is a `bill.Payment` of type `receipt`
with the invoice on its single line; see `cdar/payment-212-encaissee.json`.
