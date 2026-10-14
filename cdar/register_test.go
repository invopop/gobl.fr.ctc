package cdar_test

import (
	"testing"

	frcdar "github.com/invopop/gobl.fr.ctc/cdar"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/convert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertRegister(t *testing.T) {
	t.Run("formats", func(t *testing.T) {
		for _, k := range []cbc.Key{frcdar.KeyCDV, frcdar.KeyCDVPPF} {
			f := convert.FormatFor(k)
			require.NotNil(t, f, k)
			assert.Equal(t, cbc.Key("cdar"), f.Syntax)
		}
	})

	t.Run("detect", func(t *testing.T) {
		for file, key := range map[string]cbc.Key{
			"UC1_F202500003_04-CDV-204_Prise_en_charge.xml":  frcdar.KeyCDV,
			"UC1_F202500003_01-CDV-200_Deposee_POUR_PPF.xml": frcdar.KeyCDVPPF,
		} {
			data, err := readParse(file)
			require.NoError(t, err)
			f, err := convert.Detect(data)
			require.NoError(t, err)
			assert.Equal(t, key, f.Key, file)
		}
	})

	t.Run("import and export", func(t *testing.T) {
		data, err := readParse("UC4_F202500006_04-CDV-207_En_litige.xml")
		require.NoError(t, err)
		env, err := convert.Import(data)
		require.NoError(t, err)

		out, err := convert.Export(env, frcdar.KeyCDV)
		require.NoError(t, err)
		f, err := convert.Detect(out.Data)
		require.NoError(t, err)
		assert.Equal(t, frcdar.KeyCDV, f.Key, "detected again")
	})
}
