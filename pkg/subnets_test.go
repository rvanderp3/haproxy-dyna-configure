package pkg

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func TestAbs(t *testing.T) {
	monitorRanges, err := parseSubnetsJson()

	if err != nil {
		t.Errorf("failed: %s", err)
	}
	spew.Dump(monitorRanges)
}
