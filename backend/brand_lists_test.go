package main

import (
	"reflect"
	"testing"
)

func TestRejoinSplitItems(t *testing.T) {
	cases := []struct {
		in, want []string
	}{
		{[]string{"Starter Kit (monthly plan", "50 seats)", "Coaching"}, []string{"Starter Kit (monthly plan, 50 seats)", "Coaching"}},
		{[]string{"A (x", "y", "z)", "B"}, []string{"A (x, y, z)", "B"}},
		{[]string{"Plan [beta", "v2]"}, []string{"Plan [beta, v2]"}},
		{[]string{"Unclosed (typo", "Other", "Third"}, []string{"Unclosed (typo", "Other", "Third"}},
		{[]string{"Fine (ok)", "Also fine"}, []string{"Fine (ok)", "Also fine"}},
		{nil, []string{}},
	}
	for _, c := range cases {
		if got := rejoinSplitItems(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("rejoinSplitItems(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
