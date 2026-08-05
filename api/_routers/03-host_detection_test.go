package _routers

import "testing"

func TestHostFromRemoteAddress(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		expected    string
		expectError bool
	}{
		{
			name:     "IPv4 with port",
			address:  "127.0.0.1:1234",
			expected: "127.0.0.1",
		},
		{
			name:     "bare IPv4",
			address:  "192.168.99.22",
			expected: "192.168.99.22",
		},
		{
			name:     "IPv6 with port",
			address:  "[2001:db8::1]:1234",
			expected: "2001:db8::1",
		},
		{
			name:     "bare IPv6",
			address:  "2001:db8::1",
			expected: "2001:db8::1",
		},
		{
			name:     "forwarded IPv4 chain",
			address:  "203.0.113.10, 192.168.99.22, 127.0.0.1",
			expected: "203.0.113.10",
		},
		{
			name:     "forwarded IPv6 chain",
			address:  "[2001:db8::1]:1234, 192.168.99.22",
			expected: "2001:db8::1",
		},
		{
			name:        "invalid address",
			address:     "not-an-address",
			expected:    "not-an-address",
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			host, err := hostFromRemoteAddress(test.address)
			if (err != nil) != test.expectError {
				t.Fatalf("expected error=%t, got %v", test.expectError, err)
			}
			if host != test.expected {
				t.Fatalf("expected host %q, got %q", test.expected, host)
			}
		})
	}
}
