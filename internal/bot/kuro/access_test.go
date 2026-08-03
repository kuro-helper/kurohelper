package kuro

import "testing"

func TestParseIDSet(t *testing.T) {
	ids := ParseIDSet(" 1,2,1, ")
	if len(ids) != 2 {
		t.Fatalf("ids = %#v", ids)
	}
	if !isAllowed(ids, "1") || isAllowed(ids, "3") {
		t.Fatalf("unexpected allow-list behavior: %#v", ids)
	}
}
