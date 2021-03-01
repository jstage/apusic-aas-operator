package resources

import (
	appsv1 "k8s.io/api/apps/v1"
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
	acp := NewAcp(nil)
	t.Logf("%d,%d,%d,%d,%d,", SVCNAME, STATEFULNAME, PVCNAME, DEPLOYNAME, HEADLESS)
	fun := acp.ResTypeFuncs[HEADLESS]
	t.Logf("result %s", fun("aaa"))
	var deploy *appsv1.Deployment
	t.Log(deploy)
}
