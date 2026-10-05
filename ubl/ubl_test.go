package ubl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl"
	_ "github.com/invopop/gobl.fr.ctc/addon"
	"github.com/invopop/gobl.fr.ctc/addon/flow2"
	frubl "github.com/invopop/gobl.fr.ctc/ubl"
	goblubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadXML(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

func loadEnvelope(t *testing.T, name string) *gobl.Envelope {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "examples", "out", name))
	require.NoError(t, err)
	env := new(gobl.Envelope)
	require.NoError(t, json.Unmarshal(data, env))
	return env
}

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
	assert.Contains(t, keys, goblubl.ContextEN16931.Key, "base contexts from gobl.ubl")
}

func TestDetect(t *testing.T) {
	tests := []struct {
		file string
		key  cbc.Key
	}{
		{"b2b-reg.xml", frubl.KeyCIUS},
		{"b2g-invoice.xml", frubl.KeyExtended},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			ctx, err := convert.Detect(loadXML(t, tt.file))
			require.NoError(t, err)
			assert.Equal(t, tt.key, ctx.Key)
		})
	}
}

func TestImport(t *testing.T) {
	env, err := convert.Import(loadXML(t, "b2b-reg.xml"))
	require.NoError(t, err)
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	assert.Contains(t, inv.GetAddons(), flow2.V1)
}

func TestExport(t *testing.T) {
	t.Run("cius", func(t *testing.T) {
		env := loadEnvelope(t, "invoice-fr-fr-ctc-b2b.json")
		out, err := convert.Export(env, frubl.KeyCIUS, goblubl.ContextEN16931.Key)
		require.NoError(t, err)
		assert.Equal(t, frubl.KeyCIUS, out.Context.Key)

		ctx, err := convert.Detect(out.Data)
		require.NoError(t, err)
		assert.Equal(t, frubl.KeyCIUS, ctx.Key, "detected again")
	})
	t.Run("extended", func(t *testing.T) {
		env := loadEnvelope(t, "invoice-fr-fr-ctc-b2b.json")
		out, err := convert.Export(env, frubl.KeyExtended)
		require.NoError(t, err)
		assert.Contains(t, string(out.Data), goblubl.ContextPeppolFranceExtended.OutputCustomizationID)
	})
	t.Run("addon missing", func(t *testing.T) {
		env := loadEnvelope(t, "invoice-fr-fr-ctc-flow10-b2c.json")
		_, err := convert.Export(env, frubl.KeyCIUS, frubl.KeyExtended)
		assert.ErrorIs(t, err, convert.ErrNotSupported)
	})
}
