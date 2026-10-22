package facturx_test

import (
	"testing"

	"github.com/invopop/gobl.fr.ctc/addon/facturx"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddon(t *testing.T) {
	ad := tax.AddonForKey(facturx.V1)
	require.NotNil(t, ad)
	assert.Equal(t, "French Factur-X v1", ad.Name.String())
	assert.Equal(t, en16931.V2017, ad.Requires[0])
}
