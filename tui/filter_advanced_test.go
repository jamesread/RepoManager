package main

import (
	"testing"

	"github.com/jamesread/RepoManager/core/helpers"
)

func TestMatchFilterRuleMultiTags(t *testing.T) {
	field := helpers.Field{ID: "tags", Kind: helpers.FieldMulti}
	rule := filterRule{FieldID: "tags", Label: "Tags", Values: []string{"Git", "GitHub"}}

	both := projectItem{tags: []string{"Git", "GitHub", "Go"}}
	one := projectItem{tags: []string{"Git"}}
	none := projectItem{tags: []string{"Go"}}

	if !matchFilterRule(both, rule, field) {
		t.Fatal("expected both tags to match")
	}
	if matchFilterRule(one, rule, field) {
		t.Fatal("expected missing GitHub tag to fail")
	}
	if matchFilterRule(none, rule, field) {
		t.Fatal("expected unrelated tags to fail")
	}
}

func TestMatchFilterRuleNegatedTags(t *testing.T) {
	field := helpers.Field{ID: "tags", Kind: helpers.FieldMulti}
	rule := filterRule{FieldID: "tags", Label: "Tags", Negated: []string{"Archived"}}

	if !matchFilterRule(projectItem{tags: []string{"Git", "Go"}}, rule, field) {
		t.Fatal("expected project without negated tag to match")
	}
	if matchFilterRule(projectItem{tags: []string{"Git", "Archived"}}, rule, field) {
		t.Fatal("expected project with negated tag to fail")
	}
}

func TestMatchFilterRuleMatchAndNegate(t *testing.T) {
	field := helpers.Field{ID: "tags", Kind: helpers.FieldMulti}
	rule := filterRule{
		FieldID: "tags",
		Label:   "Tags",
		Values:  []string{"Git"},
		Negated: []string{"Archived"},
	}

	if !matchFilterRule(projectItem{tags: []string{"Git", "Go"}}, rule, field) {
		t.Fatal("expected git without archived to match")
	}
	if matchFilterRule(projectItem{tags: []string{"Git", "Archived"}}, rule, field) {
		t.Fatal("expected archived project to fail")
	}
	if matchFilterRule(projectItem{tags: []string{"Go"}}, rule, field) {
		t.Fatal("expected missing git to fail")
	}
}

func TestMatchFilterRuleSingleTagValue(t *testing.T) {
	field := helpers.Field{ID: "tags", Kind: helpers.FieldMulti}
	rule := filterRule{FieldID: "tags", Label: "Tags", Value: "Git"}

	if !matchFilterRule(projectItem{tags: []string{"Git", "GitHub"}}, rule, field) {
		t.Fatal("expected single tag match")
	}
	if matchFilterRule(projectItem{tags: []string{"GitHub"}}, rule, field) {
		t.Fatal("expected missing tag to fail")
	}
}

func TestFilterMatchedValuesSorted(t *testing.T) {
	m := model{
		advFilter: advancedFilter{
			valueStates: map[string]filterValueState{
				"GitHub": filterValueMatch,
				"Git":    filterValueMatch,
				"Rust":   filterValueNegate,
			},
		},
	}
	got := m.filterMatchedValues()
	if len(got) != 2 || got[0] != "Git" || got[1] != "GitHub" {
		t.Fatalf("matched=%v", got)
	}
	neg := m.filterNegatedValues()
	if len(neg) != 1 || neg[0] != "Rust" {
		t.Fatalf("negated=%v", neg)
	}
}

func TestCycleFilterValue(t *testing.T) {
	m := model{
		advFilter: advancedFilter{
			multiSelect: true,
			field:       helpers.Field{ID: "tags", Label: "Tags", Kind: helpers.FieldMulti},
			valueStates: map[string]filterValueState{},
			list:        newAdvancedFilter().list,
		},
	}
	m.cycleFilterValue("Git")
	if m.advFilter.valueState("Git") != filterValueMatch {
		t.Fatalf("state=%v", m.advFilter.valueState("Git"))
	}
	m.cycleFilterValue("Git")
	if m.advFilter.valueState("Git") != filterValueNegate {
		t.Fatalf("state=%v", m.advFilter.valueState("Git"))
	}
	m.cycleFilterValue("Git")
	if _, ok := m.advFilter.valueStates["Git"]; ok {
		t.Fatalf("expected ignore state to remove value, got %v", m.advFilter.valueStates)
	}
}

func TestFilterRuleShortSummaryNegated(t *testing.T) {
	r := filterRule{FieldID: "tags", Values: []string{"Git"}, Negated: []string{"Archived"}}
	if got := r.shortSummary(); got != "#=Git·#!Archived" {
		t.Fatalf("got %q", got)
	}
}
