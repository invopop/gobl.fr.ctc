package ubl_test

import (
	"testing"

	"github.com/invopop/gobl.fr.ctc/addon/flow2"
	frubl "github.com/invopop/gobl.fr.ctc/ubl"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContexts(t *testing.T) {
	for _, k := range []cbc.Key{frubl.KeyCIUS, frubl.KeyExtended} {
		ctx := convert.ContextFor(k)
		require.NotNil(t, ctx, k)
		assert.Equal(t, cbc.Key("ubl"), ctx.Syntax)
		assert.Contains(t, ctx.Addons, flow2.V1)
	}

	keys := make([]cbc.Key, 0)
	for _, ctx := range convert.ContextsFor("FR") {
		keys = append(keys, ctx.Key)
	}
	assert.Contains(t, keys, frubl.KeyCIUS)
	assert.Contains(t, keys, ubl.ContextEN16931.Key, "base contexts from gobl.ubl")
}

func TestDetect(t *testing.T) {
	tests := []struct {
		file string
		key  cbc.Key
	}{
		{"france-cius/b2b-reg.xml", frubl.KeyCIUS},
		{"france-extended/b2g-invoice.xml", frubl.KeyExtended},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := testLoadXML(tt.file)
			require.NoError(t, err)
			ctx, err := convert.Detect(data)
			require.NoError(t, err)
			assert.Equal(t, tt.key, ctx.Key)
		})
	}
}

func TestImport(t *testing.T) {
	data, err := testLoadXML("france-cius/b2b-reg.xml")
	require.NoError(t, err)
	env, err := convert.Import(data)
	require.NoError(t, err)
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	assert.Contains(t, inv.GetAddons(), flow2.V1)
}

func TestExport(t *testing.T) {
	t.Run("cius", func(t *testing.T) {
		env := loadTestEnvelope(t, "france-cius/invoice-standard.json")
		out, err := convert.Export(env, frubl.KeyCIUS, ubl.ContextEN16931.Key)
		require.NoError(t, err)
		assert.Equal(t, frubl.KeyCIUS, out.Context.Key)

		ctx, err := convert.Detect(out.Data)
		require.NoError(t, err)
		assert.Equal(t, frubl.KeyCIUS, ctx.Key, "detected again")
	})
	t.Run("extended", func(t *testing.T) {
		env := loadTestEnvelope(t, "france-extended/invoice-standard.json")
		out, err := convert.Export(env, frubl.KeyExtended)
		require.NoError(t, err)
		assert.Contains(t, string(out.Data), frubl.ContextExtended.OutputCustomizationID)

		ctx, err := convert.Detect(out.Data)
		require.NoError(t, err)
		assert.Equal(t, frubl.KeyExtended, ctx.Key, "detected again")
	})
	t.Run("addon missing", func(t *testing.T) {
		env := loadTestEnvelope(t, "invoice-minimal.json")
		_, err := convert.Export(env, frubl.KeyCIUS, frubl.KeyExtended)
		assert.ErrorIs(t, err, convert.ErrNotSupported)
	})
}
