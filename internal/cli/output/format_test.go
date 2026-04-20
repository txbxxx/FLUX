package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintJSON(t *testing.T) {
	data := map[string]interface{}{
		"name":  "test",
		"count": 42,
	}

	var buf bytes.Buffer
	err := printJSON(&buf, data)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), `"name": "test"`)
	assert.Contains(t, buf.String(), `"count": 42`)
}

func TestPrintYAML(t *testing.T) {
	data := map[string]interface{}{
		"name":  "test",
		"count": 42,
	}

	var buf bytes.Buffer
	err := printYAML(&buf, data)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "name: test")
	assert.Contains(t, buf.String(), "count: 42")
}

func TestPrintFormat(t *testing.T) {
	data := map[string]string{"name": "test"}
	table := &Table{
		Columns: []Column{{Title: "Name"}},
		Rows:    []Row{{Cells: []string{"test"}}},
	}

	// Test JSON
	var bufJSON bytes.Buffer
	err := Print(&bufJSON, FormatJSON, nil, data)
	assert.NoError(t, err)
	assert.Contains(t, bufJSON.String(), `"name": "test"`)

	// Test YAML
	var bufYAML bytes.Buffer
	err = Print(&bufYAML, FormatYAML, nil, data)
	assert.NoError(t, err)
	assert.Contains(t, bufYAML.String(), "name: test")

	// Test Table
	var bufTable bytes.Buffer
	err = Print(&bufTable, FormatTable, table, nil)
	assert.NoError(t, err)
	assert.Contains(t, bufTable.String(), "Name")
}
