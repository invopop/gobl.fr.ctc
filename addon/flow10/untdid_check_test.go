package flow10

import (
	"testing"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckUNTDIDDocTypeSetByScenario(t *testing.T) {
	inv := testInvoiceB2BCrossBorder(t)
	require.NoError(t, inv.Calculate())
	t.Logf("untdid-document-type after Calculate: %q", inv.Tax.Ext.Get(untdid.ExtKeyDocumentType))
	assert.Equal(t, "380", inv.Tax.Ext.Get(untdid.ExtKeyDocumentType).String())
}

func TestCheckUNTDIDDocTypeAbsent(t *testing.T) {
	inv := testInvoiceB2BCrossBorder(t)
	require.NoError(t, inv.Calculate())
	inv.Tax.Ext = inv.Tax.Ext.Delete(untdid.ExtKeyDocumentType)
	err := rules.Validate(inv)
	t.Logf("validation with untdid-document-type ABSENT: %v", err)
}

func TestCheckUNTDIDDocTypeInvalid(t *testing.T) {
	inv := testInvoiceB2BCrossBorder(t)
	require.NoError(t, inv.Calculate())
	inv.Tax.Ext = inv.Tax.Ext.Set(untdid.ExtKeyDocumentType, "326")
	err := rules.Validate(inv)
	t.Logf("validation with untdid-document-type=326 (partial invoice, not whitelisted): %v", err)
	assert.Error(t, err)
}

func TestCheckUNTDIDDocTypeDebitNote(t *testing.T) {
	inv := testInvoiceB2BCrossBorder(t)
	inv.Type = bill.InvoiceTypeDebitNote
	inv.Preceding = []*org.DocumentRef{{Code: "INV-2026-000"}}
	require.NoError(t, inv.Calculate())
	t.Logf("debit-note untdid-document-type after Calculate: %q", inv.Tax.Ext.Get(untdid.ExtKeyDocumentType))
	err := rules.Validate(inv)
	t.Logf("debit-note validation: %v", err)
}
