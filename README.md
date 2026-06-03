# GOBL ➡️ French CTC

French Continuous Transaction Control (CTC) addon for [GOBL](https://github.com/invopop/gobl).

This module implements the French e-invoicing / e-reporting reform as a set of
GOBL tax addons covering the three exchange flows:

- **Flow 2** (`fr-ctc-flow2-v1`) — B2B clearance of domestic invoices between two
  French parties.
- **Flow 6** (`fr-ctc-flow6-v1`) — "Cycle de Vie" lifecycle statuses and payment
  messages (`bill.Status`, `bill.Payment`).
- **Flow 10** (`fr-ctc-flow10-v1`) — e-reporting for B2C and cross-border B2B.

A meta-addon (`fr-ctc-v1`) auto-dispatches an invoice or payment to the correct
flow based on its parties.

Unlike the format converters in the GOBL ecosystem, this is a true **addon**: it
registers extensions, normalizers, and validation rules into GOBL's global
registry. It lives in its own module so that only projects handling French CTC
documents take on its weight.

## Usage

Add a blank import so the addon registers itself, then use GOBL as normal:

```go
import (
	"github.com/invopop/gobl"
	_ "github.com/invopop/gobl.fr.ctc"
)
```

Declare the addon on a document (or let the regime/scenario add it) and
`Calculate` + `Validate` will run the full CTC normalization and rules.

> **Note**: the `fr-ctc-*` keys are listed in GOBL core's approved external-addon
> registry, so they are recognised as valid `$addons` values in the JSON Schema.
> The runtime check stays strict, however: a document declaring an `fr-ctc-*`
> addon will fail validation with `add-on must be registered` unless this module
> is imported. Any service that processes French CTC documents must import it.

## Development

The addon builds on core GOBL features (the `bill` lifecycle, the `dgfip`
catalogue, and the approved external-addon registry) that are not yet in a
tagged release. The `go.mod` therefore carries a `replace` directive pointing at
a local checkout of `../gobl`; drop it once a core release including those
features is available.

```sh
go test ./...
```

## License

Apache 2.0 — see [LICENSE](./LICENSE).
