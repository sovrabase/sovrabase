package connections

import "testing"

func TestValidateProjectID(t *testing.T) {
	valid := []string{"ab", "proj_1", "PROJ-1", "a_b-c123"}
	for _, value := range valid {
		if err := ValidateProjectID(value); err != nil {
			t.Fatalf("ValidateProjectID(%q) unexpected error: %v", value, err)
		}
	}

	invalid := []string{"", "a", "with space", "x$", "this-project-id-is-way-too-long-to-be-valid-because-it-exceeds-sixty-four-characters"}
	for _, value := range invalid {
		if err := ValidateProjectID(value); err == nil {
			t.Fatalf("ValidateProjectID(%q) expected error, got nil", value)
		}
	}
}

func TestValidateSlug(t *testing.T) {
	valid := []string{"ab", "my-db", "proj_001", "x1"}
	for _, value := range valid {
		if err := ValidateSlug(value); err != nil {
			t.Fatalf("ValidateSlug(%q) unexpected error: %v", value, err)
		}
	}

	invalid := []string{"", "A1", " space", "a", "a*", "-bad", "x", "UPPER"}
	for _, value := range invalid {
		if err := ValidateSlug(value); err == nil {
			t.Fatalf("ValidateSlug(%q) expected error, got nil", value)
		}
	}
}

func TestConnectionEngineValidate(t *testing.T) {
	if err := ConnectionEnginePostgres.Validate(); err != nil {
		t.Fatalf("postgres engine validation error: %v", err)
	}
	if err := ConnectionEngineMongo.Validate(); err != nil {
		t.Fatalf("mongo engine validation error: %v", err)
	}
	if err := ConnectionEngine("mysql").Validate(); err == nil {
		t.Fatalf("mysql engine validation expected error, got nil")
	}
}
