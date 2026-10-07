package cdar_test

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/invopop/gobl"
	cii "github.com/invopop/gobl.cii"
	frcdar "github.com/invopop/gobl.fr.ctc/cdar"
	"github.com/invopop/gobl/bill"
)

// updateOut is a flag that can be set to update example files
var updateOut = flag.Bool("update", false, "Update the example files in testdata")

// validate is a flag that enables schematron validation against phorm
var validate = flag.Bool("validate", false, "Run phorm schematron validation on generated XML")

func getConvertPath() string {
	return filepath.Join("testdata", "convert")
}

func getParsePath() string {
	return filepath.Join("testdata", "parse")
}

// decodeCDAR decodes raw CDAR XML.
func decodeCDAR(data []byte) (*cii.CDAR, error) {
	doc, err := cii.Decode(data)
	if err != nil {
		return nil, err
	}
	cdar, ok := doc.(*cii.CDAR)
	if !ok {
		return nil, cii.ErrUnsupportedDocumentType
	}
	return cdar, nil
}

// parseStatus decodes raw CDAR XML into a status, without an envelope.
func parseStatus(data []byte) (*bill.Status, error) {
	cdar, err := decodeCDAR(data)
	if err != nil {
		return nil, err
	}
	return frcdar.ImportStatus(cdar)
}

// parsePayment decodes raw CDAR XML into a payment, without an envelope.
func parsePayment(data []byte) (*bill.Payment, error) {
	cdar, err := decodeCDAR(data)
	if err != nil {
		return nil, err
	}
	return frcdar.ImportPayment(cdar)
}

// parseCDAR decodes and imports raw CDAR XML into an envelope.
func parseCDAR(data []byte, opts ...frcdar.Option) (*gobl.Envelope, error) {
	cdar, err := decodeCDAR(data)
	if err != nil {
		return nil, err
	}
	return frcdar.Import(cdar, opts...)
}

// readParse reads a fixture from testdata/parse.
func readParse(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(getParsePath(), name))
}
