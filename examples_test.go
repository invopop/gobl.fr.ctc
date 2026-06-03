package frctc_test

import (
	"flag"
	"testing"

	// Register the French CTC addon so example documents declaring the
	// fr-ctc-* addons normalize and validate.
	_ "github.com/invopop/gobl.fr.ctc/addon"

	"github.com/invopop/gobl/pkg/examples/exampletest"
)

var update = flag.Bool("update", false, "update the example golden files")

// TestExamples converts every document under examples/ to a calculated,
// validated JSON envelope and compares it against its golden output, using the
// shared GOBL example helpers. Run with -update to (re)generate the goldens.
func TestExamples(t *testing.T) {
	exampletest.Run(t, "examples", *update)
}
