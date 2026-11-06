package tunnel

import "testing"

func TestForwardedTail(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{"invoke", "/http/in-process/9080/invoke/Greeter/Greet", "/invoke/Greeter/Greet", true},
		{"discover", "/http/in-process/9080/discover", "/discover", true},
		{"root tail", "/http/in-process/9080/", "/", true},
		{"with query", "/http/in-process/9080/invoke/S/h?foo=bar", "/invoke/S/h?foo=bar", true},
		{"real host", "/https/svc.ns.svc.cluster.local/9080/invoke/S/h", "/invoke/S/h", true},
		{"non-numeric port not a prefix", "/invoke/Svc/handler", "", false},
		{"too few segments", "/discover", "", false},
		{"empty scheme", "//host/9080/x", "", false},
		{"empty host", "/http//9080/x", "", false},
		{"root", "/", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := forwardedTail(tc.in)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("forwardedTail(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestComputeDeploymentURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://proxy.restate.cloud", "https://proxy.restate.cloud:9080/http/in-process/9080/"},
		{"https://proxy.restate.cloud:9080", "https://proxy.restate.cloud:9080/http/in-process/9080/"},
		{"https://proxy.restate.cloud:8080/", "https://proxy.restate.cloud:8080/http/in-process/9080/"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := computeDeploymentURL(tc.in); got != tc.want {
			t.Fatalf("computeDeploymentURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
