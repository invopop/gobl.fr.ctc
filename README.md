# GOBL ➡️ French CTC

French Continuous Transaction Control (CTC) addon for [GOBL](https://github.com/invopop/gobl).

Released under the Apache 2.0 [LICENSE](https://github.com/invopop/gobl.fr.ctc/blob/main/LICENSE), Copyright 2026 [Invopop S.L.](https://invopop.com).

[![Lint](https://github.com/invopop/gobl.fr.ctc/actions/workflows/lint.yaml/badge.svg)](https://github.com/invopop/gobl.fr.ctc/actions/workflows/lint.yaml)
[![Test Go](https://github.com/invopop/gobl.fr.ctc/actions/workflows/test.yaml/badge.svg)](https://github.com/invopop/gobl.fr.ctc/actions/workflows/test.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/invopop/gobl.fr.ctc)](https://goreportcard.com/report/github.com/invopop/gobl.fr.ctc)
[![codecov](https://codecov.io/gh/invopop/gobl.fr.ctc/graph/badge.svg)](https://codecov.io/gh/invopop/gobl.fr.ctc)
[![GoDoc](https://godoc.org/github.com/invopop/gobl.fr.ctc?status.svg)](https://godoc.org/github.com/invopop/gobl.fr.ctc)
![Latest Tag](https://img.shields.io/github/v/tag/invopop/gobl.fr.ctc)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/invopop/gobl.fr.ctc)

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

## Layout

- `addon/` — the GOBL addon: extensions, normalizers, scenarios, and validation
  rules that register into GOBL on import (`addon/flow2`, `addon/flow6`,
  `addon/flow10` plus the `fr-ctc-v1` meta-addon). This package is kept dependency-
  light so importing it never pulls in conversion tooling.
- the module root (and future subpackages) is reserved for converters and other
  CTC logic that build on the addon.

## Usage

Add a blank import of the **addon** so it registers itself, then use GOBL as
normal:

```go
import (
	"github.com/invopop/gobl"
	_ "github.com/invopop/gobl.fr.ctc/addon"
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

The addon builds on core GOBL features (the `bill` lifecycle and the approved
external-addon registry) that are not yet in a tagged release. The `go.mod`
therefore pins `github.com/invopop/gobl` to a commit on the core branch (a
pseudo-version); bump it to the release tag once core is published.

```sh
go test ./...
```

### Examples

`examples/` holds sample documents for each flow, with their expected JSON
envelopes under `examples/out/`. They are verified via GOBL's shared
`pkg/examples` helpers. Regenerate the golden output after intentional changes
with:

```sh
go test . -run TestExamples -update
```

## License

Apache 2.0 — see [LICENSE](./LICENSE).
