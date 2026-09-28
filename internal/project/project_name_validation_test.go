package project

import (
	"strings"
	"testing"
)

func TestProjectNameMaximumCharacters(t *testing.T) {
	if err := validateCreateProjectInput(CreateProjectInput{Name: strings.Repeat("界", 15)}); err != nil {
		t.Fatalf("15-character project name rejected: %v", err)
	}
	if err := validateCreateProjectInput(CreateProjectInput{Name: strings.Repeat("界", 16)}); err == nil {
		t.Fatal("16-character project name accepted")
	}
}
