// Package addon is a convenience aggregator that registers all of the French
// CTC flow-specific addons:
//
//   - fr-ctc-flow2-v1  (domestic B2B clearance)
//   - fr-ctc-flow6-v1  (lifecycle status messages)
//   - fr-ctc-flow10-v1 (B2C / cross-border B2B e-reporting and payment receipts)
//
// Import it for its side effects to make all of them available, or import the
// individual flow subpackages directly. Each document must declare the flow
// addon that applies to it; there is no auto-dispatch.
package addon

import (
	_ "github.com/invopop/gobl.fr.ctc/addon/flow10"
	_ "github.com/invopop/gobl.fr.ctc/addon/flow2"
	_ "github.com/invopop/gobl.fr.ctc/addon/flow6"
)
