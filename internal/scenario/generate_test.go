package scenario

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/B42Labs/openstack-tester/internal/plan"
)

var update = flag.Bool("update", false, "update golden files")

// smallScenario is a compact scenario that still exercises every resource kind
// (pools, IPv4/IPv6/pool subnets, router interfaces, rules, multi-SG ports). It
// backs the golden test that locks byte-stability across runs and Go versions.
func smallScenario() Scenario {
	return Scenario{
		Name: "small",
		Seed: 42,
		Resources: Resources{
			SubnetPools:    1,
			AddressScopes:  1,
			Networks:       3,
			Routers:        2,
			SecurityGroups: 2,
		},
		Distribution: Distribution{
			SubnetsPerNetwork:            Range{Min: 1, Max: 3},
			PortsPerNetwork:              Range{Min: 1, Max: 2},
			RulesPerSecurityGroup:        Range{Min: 1, Max: 3},
			SubnetFromPoolRatio:          0.5,
			IPv6Ratio:                    0.3,
			SubnetsAttachedToRouterRatio: 0.7,
		},
		Topology: Topology{
			RouterAttachStrategy:   "random",
			PortSecurityGroupCount: Range{Min: 1, Max: 2},
		},
	}
}

// marshal renders a plan exactly as the generate command does: indented JSON
// with a trailing newline.
func marshal(t *testing.T, p *plan.Plan) []byte {
	t.Helper()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("marshalling plan: %v", err)
	}
	return append(data, '\n')
}

func TestGenerateInvalidScenario(t *testing.T) {
	s := smallScenario()
	s.Name = "" // fails Scenario.Validate

	if _, err := s.Generate(); err == nil {
		t.Fatal("Generate() = nil error, want error for invalid scenario")
	}
}

func TestGenerateDeterministic(t *testing.T) {
	s := smallScenario()

	p1, err := s.Generate()
	if err != nil {
		t.Fatalf("first Generate(): %v", err)
	}
	p2, err := s.Generate()
	if err != nil {
		t.Fatalf("second Generate(): %v", err)
	}

	if got, want := marshal(t, p1), marshal(t, p2); !bytes.Equal(got, want) {
		t.Error("two generations of the same scenario+seed differ")
	}
}

func TestGenerateSeedChangesTopology(t *testing.T) {
	s1 := smallScenario()
	s2 := smallScenario()
	s2.Seed = s1.Seed + 1

	p1, err := s1.Generate()
	if err != nil {
		t.Fatalf("Generate(seed=%d): %v", s1.Seed, err)
	}
	p2, err := s2.Generate()
	if err != nil {
		t.Fatalf("Generate(seed=%d): %v", s2.Seed, err)
	}

	if bytes.Equal(marshal(t, p1), marshal(t, p2)) {
		t.Error("different seeds produced identical plans")
	}
}

func TestGenerateGolden(t *testing.T) {
	p, err := smallScenario().Generate()
	if err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	got := marshal(t, p)

	path := filepath.Join("testdata", "golden", "small.plan.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("generated plan differs from golden file %s; run with -update if the change is intended", path)
	}
}

func TestGenerateCounts(t *testing.T) {
	s := Scenario{
		Name: "counts",
		Seed: 7,
		Resources: Resources{
			SubnetPools:    2,
			AddressScopes:  1,
			Networks:       40,
			Routers:        5,
			SecurityGroups: 4,
		},
		Distribution: Distribution{
			SubnetsPerNetwork:            Range{Min: 1, Max: 3},
			PortsPerNetwork:              Range{Min: 0, Max: 4},
			RulesPerSecurityGroup:        Range{Min: 2, Max: 6},
			SubnetFromPoolRatio:          0.4,
			IPv6Ratio:                    0.3,
			SubnetsAttachedToRouterRatio: 0.6,
		},
		Topology: Topology{
			RouterAttachStrategy:   "random",
			PortSecurityGroupCount: Range{Min: 1, Max: 2},
		},
	}

	p, err := s.Generate()
	if err != nil {
		t.Fatalf("Generate(): %v", err)
	}

	// Fixed counts match the scenario exactly.
	if got := len(p.AddressScopes); got != s.Resources.AddressScopes {
		t.Errorf("address scopes = %d, want %d", got, s.Resources.AddressScopes)
	}
	if got := len(p.SubnetPools); got != s.Resources.SubnetPools {
		t.Errorf("subnet pools = %d, want %d", got, s.Resources.SubnetPools)
	}
	if got := len(p.Networks); got != s.Resources.Networks {
		t.Errorf("networks = %d, want %d", got, s.Resources.Networks)
	}
	if got := len(p.Routers); got != s.Resources.Routers {
		t.Errorf("routers = %d, want %d", got, s.Resources.Routers)
	}
	if got := len(p.SecurityGroups); got != s.Resources.SecurityGroups {
		t.Errorf("security groups = %d, want %d", got, s.Resources.SecurityGroups)
	}

	// Per-security-group rule counts fall within their distribution bounds.
	for _, sg := range p.SecurityGroups {
		if n := len(sg.Rules); n < s.Distribution.RulesPerSecurityGroup.Min || n > s.Distribution.RulesPerSecurityGroup.Max {
			t.Errorf("security group %q has %d rules, want within [%d,%d]", sg.Name, n,
				s.Distribution.RulesPerSecurityGroup.Min, s.Distribution.RulesPerSecurityGroup.Max)
		}
	}

	// A subnet attaches to at most one router.
	attached := make(map[string]bool)
	for _, ri := range p.RouterInterfaces {
		if attached[ri.Subnet] {
			t.Errorf("subnet %q attached to more than one router", ri.Subnet)
		}
		attached[ri.Subnet] = true
	}

	// Both IPv6 and pool-allocated subnets appear when their ratios are nonzero.
	var ipv6, pool int
	for _, sub := range p.Subnets {
		switch {
		case sub.IPVersion == 6:
			ipv6++
		case sub.SubnetPool != "":
			pool++
		}
	}
	if ipv6 == 0 {
		t.Error("no IPv6 subnets generated despite ipv6_ratio > 0")
	}
	if pool == 0 {
		t.Error("no pool-allocated subnets generated despite subnet_from_pool_ratio > 0")
	}
}
