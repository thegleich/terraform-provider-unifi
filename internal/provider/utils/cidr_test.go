package utils

import (
	"testing"
)

func TestCIDRValidate(t *testing.T) {
	for _, c := range []struct {
		expectedError string
		cidr          string
	}{
		{"invalid CIDR address: ", ""},
		{"invalid CIDR address: abc", "abc"},
		{"invalid CIDR address: 192.1.2.3", "192.1.2.3"},
		{"invalid CIDR address: 500.1.2.3/20", "500.1.2.3/20"},
		{"invalid CIDR address: 192.1.2.3/500", "192.1.2.3/500"},

		{"", "192.1.2.1/20"},
	} {
		t.Run(c.cidr, func(t *testing.T) {
			_, actualErrs := CidrValidate(c.cidr, "key")
			switch len(actualErrs) {
			case 0:
				if c.expectedError != "" {
					t.Fatalf("expected no error, got %d: %#v", len(actualErrs), actualErrs)
				}
			case 1:
				actualErr := actualErrs[0].Error()
				if actualErr != c.expectedError {
					t.Fatalf("expected %q, got %q", c.expectedError, actualErr)
				}
			default:
				t.Fatalf("expected 0 or 1 errors, got %d: %#v", len(actualErrs), actualErrs)
			}
		})
	}
}

func TestCidrGateway(t *testing.T) {
	for _, c := range []struct {
		cidr     string
		expected string
	}{
		{"192.168.100.0/24", "192.168.100.1/24"},
		{"192.168.100.1/24", "192.168.100.1/24"},
		{"192.168.100.2/24", "192.168.100.2/24"},
		{"10.0.16.0/20", "10.0.16.1/20"},
		{"10.0.20.5/20", "10.0.20.5/20"},
		{"invalid", ""},
	} {
		t.Run(c.cidr, func(t *testing.T) {
			if actual := CidrGateway(c.cidr); actual != c.expected {
				t.Fatalf("expected %q, got %q", c.expected, actual)
			}
		})
	}
}

func TestCidrFromGateway(t *testing.T) {
	for _, c := range []struct {
		cidr     string
		expected string
	}{
		{"192.168.100.1/24", "192.168.100.0/24"},
		{"192.168.100.0/24", "192.168.100.0/24"},
		{"192.168.100.2/24", "192.168.100.2/24"},
		{"10.0.0.2/32", "10.0.0.2/32"},
		{"invalid", ""},
	} {
		t.Run(c.cidr, func(t *testing.T) {
			if actual := CidrFromGateway(c.cidr); actual != c.expected {
				t.Fatalf("expected %q, got %q", c.expected, actual)
			}
		})
	}
}

func TestCidrGatewayDiffSuppress(t *testing.T) {
	for _, c := range []struct {
		old      string
		new      string
		suppress bool
	}{
		{"192.168.100.0/24", "192.168.100.0/24", true},
		{"192.168.100.0/24", "192.168.100.1/24", true},
		{"192.168.100.2/24", "192.168.100.2/24", true},
		{"192.168.100.0/24", "192.168.100.2/24", false},
		{"192.168.100.2/24", "192.168.100.3/24", false},
		{"192.168.100.0/24", "192.168.100.0/25", false},
		{"", "192.168.100.0/24", false},
		{"192.168.100.0/24", "", false},
		{"", "", false},
	} {
		t.Run(c.old+"->"+c.new, func(t *testing.T) {
			if actual := CidrGatewayDiffSuppress("subnet", c.old, c.new, nil); actual != c.suppress {
				t.Fatalf("expected %t, got %t", c.suppress, actual)
			}
		})
	}
}
