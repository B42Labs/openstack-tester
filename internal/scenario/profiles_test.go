package scenario

import (
	"testing"

	"github.com/B42Labs/openstack-tester/scenarios"
)

// profileNames are the built-in scenario profiles shipped under scenarios/.
var profileNames = []string{"small", "medium", "large"}

// readProfile reads and parses a shipped profile by name from the embedded
// scenarios filesystem, so the test does not depend on the process working
// directory.
func readProfile(t *testing.T, name string) Scenario {
	t.Helper()
	data, err := scenarios.Files.ReadFile(name + ".yaml")
	if err != nil {
		t.Fatalf("reading profile %s.yaml: %v", name, err)
	}
	s, err := Parse(data)
	if err != nil {
		t.Fatalf("parsing profile %s.yaml: %v", name, err)
	}
	return s
}

// TestProfilesGenerateValidPlans locks the core acceptance criterion: every
// shipped profile parses, names itself after its file, validates, and expands
// into a valid plan.
func TestProfilesGenerateValidPlans(t *testing.T) {
	for _, name := range profileNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := readProfile(t, name)
			if s.Name != name {
				t.Errorf("profile %s.yaml has name %q, want %q", name, s.Name, name)
			}
			if err := s.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if _, err := s.Generate(); err != nil {
				t.Fatalf("Generate() = %v, want nil", err)
			}
		})
	}
}

// TestMediumProfileMatchesReadmeExample asserts the shipped medium profile is
// the README §6 example, by comparing it to the in-package fixture that the
// README-example parse test also pins.
func TestMediumProfileMatchesReadmeExample(t *testing.T) {
	if got, want := readProfile(t, "medium"), mediumScenario(); got != want {
		t.Errorf("scenarios/medium.yaml = %+v, want %+v", got, want)
	}
}

// TestSmallProfileMatchesGoldenFixture asserts the shipped small profile equals
// the smallScenario fixture, tying it transitively to the golden plan locked by
// TestGenerateGolden.
func TestSmallProfileMatchesGoldenFixture(t *testing.T) {
	if got, want := readProfile(t, "small"), smallScenario(); got != want {
		t.Errorf("scenarios/small.yaml = %+v, want %+v", got, want)
	}
}

// TestLargeProfileHitsHeadlineScale asserts the large profile expands to at
// least the headline 20-routers / 100-networks / 200-subnets target and stays
// strictly larger than medium, so the two profiles never collapse into the same
// scale.
func TestLargeProfileHitsHeadlineScale(t *testing.T) {
	large, err := readProfile(t, "large").Generate()
	if err != nil {
		t.Fatalf("Generate(large): %v", err)
	}

	if got := len(large.Routers); got < 20 {
		t.Errorf("large routers = %d, want >= 20", got)
	}
	if got := len(large.Networks); got < 100 {
		t.Errorf("large networks = %d, want >= 100", got)
	}
	if got := len(large.Subnets); got < 200 {
		t.Errorf("large subnets = %d, want >= 200", got)
	}

	medium, err := readProfile(t, "medium").Generate()
	if err != nil {
		t.Fatalf("Generate(medium): %v", err)
	}
	if len(large.Networks) <= len(medium.Networks) {
		t.Errorf("large networks (%d) must exceed medium networks (%d)",
			len(large.Networks), len(medium.Networks))
	}
}
