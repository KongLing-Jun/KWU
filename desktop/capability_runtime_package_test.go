package main

import (
	"strings"
	"testing"
)

func TestCapabilityRuntimeResolvesEmbeddedPackage(t *testing.T) {
	pkg, found, err := resolveCapabilityPackage(crossborderCommerceCapabilityID)
	if err != nil || !found {
		t.Fatalf("resolve package: found=%v err=%v", found, err)
	}
	if pkg.Manifest.Version != "1.1.0" || pkg.InstructionHash == "" || !pkg.AllowsHostAction("dashboard.add_widget") {
		t.Fatalf("unexpected package: %#v", pkg)
	}
	input, err := capabilitySubmitContract(crossborderCommerceCapabilityID, "find products")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"package-version=\"1.1.0\"", "instruction-hash=\"", "dashboard_add_widget", "find products", "<capability-skills>", "capability_load_skill", "crossborder-research-plan"} {
		if !strings.Contains(input, want) {
			t.Fatalf("contract missing %q: %s", want, input)
		}
	}
	if len(pkg.Skills) != 1 || strings.Contains(input, pkg.Skills[0].Body) {
		t.Fatal("initial contract must contain the index, not the Skill body")
	}
}

func TestCapabilityRuntimeLeavesOrdinaryInputUntouched(t *testing.T) {
	input, err := capabilitySubmitContract("ordinary-chat", "hello")
	if err != nil || input != "hello" {
		t.Fatalf("ordinary contract = %q, err=%v", input, err)
	}
}
