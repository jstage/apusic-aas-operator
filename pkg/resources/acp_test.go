package resources

import (
	"testing"
)

type testint int

const (
	a1 testint = iota
	a2
	a3
	a4
	a5
)

func TestIota(t *testing.T) {
	t.Logf("%d,%d,%d,%d,%d,", SVCNAME, STATEFULNAME, PVCNAME, DEPLOYNAME, HEADLESS)
}
