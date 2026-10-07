package cdar_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl"
	cii "github.com/invopop/gobl.cii"
	frcdar "github.com/invopop/gobl.fr.ctc/cdar"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/phorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pathPatternJSON = "*.json"
	pathOut         = "out"
	dirCDAR         = "cdar"
	dirCDARPPF      = "cdar-PPF"

	staticUUID uuid.UUID = "0195ce71-dc9c-72c8-bf2c-9890a4a9f0a2"
)

// loadEnvelope returns a calculated and validated GOBL envelope from a file
// in testdata/convert.
func loadEnvelope(t *testing.T, name string) *gobl.Envelope {
	t.Helper()
	path := filepath.Join(getConvertPath(), name)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	env := new(gobl.Envelope)
	require.NoError(t, json.Unmarshal(data, env))

	// Clear the IDs
	env.Head.UUID = staticUUID
	require.NoError(t, env.Calculate())
	require.NoError(t, env.Validate())

	if *updateOut {
		data, err := json.MarshalIndent(env, "", "\t")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0644))
	}
	return env
}

// cdarConvertContexts pins each fixture directory to the CDAR format it
// represents. The test loop and the fixture regenerator both read the format
// from this map — never from bill.Status.Type.
var cdarConvertContexts = []struct {
	name   string
	dir    string
	format frcdar.Format
}{
	{"CDARFlow6", dirCDAR, frcdar.FormatFlow6},
	{"CDARFlow6PPF", dirCDARPPF, frcdar.FormatFlow6PPF},
}

// cdarConvertFixtures lists the synthetic CDAR status fixtures used to
// drive TestConvertCDAR. Each entry names the destination directory; that
// directory's Context (in cdarConvertContexts) is the only signal for how
// the fixture is shaped — process code and bill.Status.Type are irrelevant.
var cdarConvertFixtures = []struct {
	processCode string
	fixtureName string
	dir         string
	payment     bool
}{
	// Buyer-issued ack 23 — Issuer = Customer, Recipient = Supplier
	{"204", "status-204-prise-en-charge.json", dirCDAR, false},
	{"205", "status-205-approuvee.json", dirCDAR, false},
	{"206", "status-206-approuvee-partiellement.json", dirCDAR, false},
	{"207", "status-207-litige.json", dirCDAR, false},
	{"208", "status-208-suspendue.json", dirCDAR, false},
	{"210", "status-210-refusee.json", dirCDAR, false},
	// Platform-issued ack 23
	{"213", "status-213-rejetee-semantique.json", dirCDAR, false},
	// Seller-issued payment receipt (212 Encaissée) — bill.Payment
	{"212", "payment-212-encaissee.json", dirCDAR, true},
	// PPF transmissions (305)
	{"200", "status-200-deposee.json", dirCDARPPF, false},
	{"212", "payment-212-encaissee.json", dirCDARPPF, true},
	// NOTE: 209 (Complétée) has no flow6 (Type, Key) mapping yet and 211
	// (Paiement transmis) is out of the mandatory-status scope; both are
	// omitted until needed.
}

// regenerateCDARFixtures rebuilds the JSON fixtures for TestConvertCDAR from
// the synthetic-status / synthetic-payment factories. Only runs under
// -update. PPF (305) wire shape — WK issuer, PPF recipient — is derived
// by the converter from the Context, so the fixtures carry only the
// business parties.
func regenerateCDARFixtures(t *testing.T) {
	t.Helper()
	for _, f := range cdarConvertFixtures {
		var env *gobl.Envelope
		var err error
		if f.payment {
			pmt := buildSyntheticPayment(t, num.MakeAmount(25000, 2))
			env, err = gobl.Envelop(pmt)
			require.NoError(t, err, "envelope for %s", f.processCode)
			pmt.UUID = staticUUID
		} else {
			st := buildSyntheticStatus(t, f.processCode)
			env, err = gobl.Envelop(st)
			require.NoError(t, err, "envelope for %s", f.processCode)
			st.UUID = staticUUID
		}
		env.Head.UUID = staticUUID
		require.NoError(t, env.Calculate())
		require.NoError(t, env.Validate())

		dir := filepath.Join(getConvertPath(), f.dir)
		require.NoError(t, os.MkdirAll(dir, 0755))
		data, err := json.MarshalIndent(env, "", "\t")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, f.fixtureName), data, 0644))
	}
}

func TestConvertCDAR(t *testing.T) {
	if *updateOut {
		regenerateCDARFixtures(t)
	}

	var pc *phorm.Client
	if *validate {
		pc = phormClient(t)
	}

	for _, ctx := range cdarConvertContexts {
		t.Run(ctx.name, func(t *testing.T) {
			examples, err := filepath.Glob(filepath.Join(getConvertPath(), ctx.dir, pathPatternJSON))
			require.NoError(t, err)
			if len(examples) == 0 {
				t.Skip("No examples found for context")
			}

			for _, example := range examples {
				inName := filepath.Base(example)
				outName := strings.Replace(inName, ".json", ".xml", 1)

				t.Run(inName, func(t *testing.T) {
					env := loadEnvelope(t, filepath.Join(ctx.dir, inName))
					cdar, err := frcdar.Export(env, ctx.format)
					require.NoError(t, err)

					data, err := cii.Encode(cdar)
					require.NoError(t, err)

					outPath := filepath.Join(getConvertPath(), ctx.dir, pathOut, outName)
					if *updateOut {
						outDir := filepath.Join(getConvertPath(), ctx.dir, pathOut)
						require.NoError(t, os.MkdirAll(outDir, 0755))
						require.NoError(t, os.WriteFile(outPath, data, 0644))
					}

					if *validate && ctx.format.VESID != "" {
						validateXML(t, pc, ctx.format.VESID, data)
					}

					expected, err := os.ReadFile(outPath)
					assert.NoError(t, err)
					assert.Equal(t, string(expected), string(data), "Output should match the expected XML. Update with --update flag.")
				})
			}
		})
	}
}

// cdarFixturesNeedingRouting names the CDV fixtures whose issuer party omits
// its electronic address from the document body (as a conformant CDV may): the
// parser must hydrate the missing inbox from the transport routing. The other
// fixtures already carry both party inboxes.
var cdarFixturesNeedingRouting = map[string]bool{
	"UC1_F202500003_04-CDV-204_Prise_en_charge.xml":    true,
	"UC1_F202500003_05-CDV-205_Approuvee.xml":          true,
	"UC1_F202500003_06-CDV-211_Paiement_transmis.xml":  true,
	"UC1_F202500003_07-CDV-212_Encaissee.xml":          true,
	"UC1_F202500003_07-CDV-212_Encaissee_POUR_PPF.xml": true,
	"UC4_F202500006_04-CDV-207_En_litige.xml":          true,
	"UC5_F202500007_04-CDV-207_En_litige.xml":          true,
}

func TestParseCDAR(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join(getParsePath(), "UC*.xml"))
	require.NoError(t, err)
	require.NotEmpty(t, examples, "expected UC*.xml CDAR fixtures")

	for _, example := range examples {
		inName := filepath.Base(example)
		outName := strings.Replace(inName, ".xml", ".json", 1)

		t.Run(inName, func(t *testing.T) {
			xmlData, err := os.ReadFile(example)
			require.NoError(t, err)

			// A conformant CDV need not repeat a party's electronic address in
			// the body (it travels at the SBD layer). For the fixtures whose
			// issuer omits it, supply the transport routing the Peppol layer
			// would provide — the two business participants — so the parser
			// hydrates the missing party inbox (BR-FR-CDV-08). Fixtures that
			// already carry both inboxes are parsed without routing.
			var parseOpts []frcdar.Option
			if cdarFixturesNeedingRouting[inName] {
				parseOpts = append(parseOpts, frcdar.WithRouting(
					"iso6523-actorid-upis::0225:100000009_STATUTS",
					"iso6523-actorid-upis::0225:200000008_STATUTS",
				))
			}
			env, err := parseCDAR(xmlData, parseOpts...)
			require.NoError(t, err)

			env.Head.UUID = staticUUID
			// Parse dispatches on the ProcessConditionCode: statuses and
			// payments (211 / 212) come back as different documents.
			switch doc := env.Extract().(type) {
			case *bill.Status:
				doc.UUID = staticUUID
			case *bill.Payment:
				doc.UUID = staticUUID
			default:
				t.Fatalf("parsed document should be a status or payment, got %T", doc)
			}
			require.NoError(t, env.Calculate())
			// PPF transmission copies (305) only carry the platform-level
			// parties (WK / DFH) plus the seller SIREN — they cannot
			// satisfy the full B2B document rules and are never parsed
			// into bill documents in production (the PPF leg stays raw
			// CDAR). The B2B fixtures must validate cleanly.
			if !strings.Contains(inName, "POUR_PPF") {
				require.NoError(t, env.Validate(), "parsed envelope must satisfy GOBL validation")
			}

			outPath := filepath.Join(getParsePath(), pathOut, outName)
			if *updateOut {
				outDir := filepath.Join(getParsePath(), pathOut)
				require.NoError(t, os.MkdirAll(outDir, 0755))
				data, err := json.MarshalIndent(env, "", "\t")
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(outPath, data, 0644))
			}

			data, err := json.MarshalIndent(env.Extract(), "", "\t")
			require.NoError(t, err)

			expectedRaw, err := os.ReadFile(outPath)
			assert.NoError(t, err)

			var expectedEnv gobl.Envelope
			require.NoError(t, json.Unmarshal(expectedRaw, &expectedEnv))
			expectedData, err := json.MarshalIndent(expectedEnv.Extract(), "", "\t")
			require.NoError(t, err)

			assert.JSONEq(t, string(expectedData), string(data), "Document should match the expected JSON. Update with --update flag.")
		})
	}
}

// TestParseRoutingFromArgs verifies that a received CDAR takes its transport
// Head.From/To from the routing args (who routed it to us), overriding GOBL's
// document-derived, outgoing (supplier->customer) assumption. The 212's
// outgoing From would be the supplier (100000009); the args here are the
// REVERSE, so seeing 200000008 as From proves the args win.
func TestParseRoutingFromArgs(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(getParsePath(), "UC1_F202500003_07-CDV-212_Encaissee.xml"))
	require.NoError(t, err)

	t.Run("routing args are recorded verbatim on Head.From/To", func(t *testing.T) {
		// The args are the fully-qualified participant URIs the Peppol layer
		// supplies, REVERSED vs the 212's outgoing direction (whose From would
		// be the supplier) — proving the args win over GOBL's derivation.
		env, err := parseCDAR(data, frcdar.WithRouting(
			"iso6523-actorid-upis::0225:200000008_STATUTS",
			"iso6523-actorid-upis::0225:100000009_STATUTS",
		))
		require.NoError(t, err)
		assert.Equal(t, "iso6523-actorid-upis::0225:200000008_STATUTS", string(env.Head.From))
		assert.Equal(t, "iso6523-actorid-upis::0225:100000009_STATUTS", string(env.Head.To))

		// The contract is that a later calculation must NOT overwrite the
		// routing: normalizeRouting only fills empty fields, so the transport
		// direction survives instead of being re-derived from the document.
		require.NoError(t, env.Calculate())
		assert.Equal(t, "iso6523-actorid-upis::0225:200000008_STATUTS", string(env.Head.From))
		assert.Equal(t, "iso6523-actorid-upis::0225:100000009_STATUTS", string(env.Head.To))
	})
}
