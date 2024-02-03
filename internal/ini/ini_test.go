package ini

import (
	"strings"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestIniRead(t *testing.T) {
	iniContent := `
[rom]

[data]
E0BB = init_tests
E12F = ;) NMI period is too short/3)too long
E481 = wait_vbl
`

	reader := strings.NewReader(iniContent)
	_, err := Read(reader)
	assert.NoError(t, err)
}
