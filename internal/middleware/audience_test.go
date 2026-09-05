package middleware

import "testing"

// TestAudienceContains covers the audienceContains helper for every shape
// the "aud" claim can take per RFC 7519 §4.1.3: a single string, a []string,
// or a []any (as produced by jwt.MapClaims JSON unmarshaling).
func TestAudienceContains(t *testing.T) {
	const expected = "statistiloto-ui"

	tests := []struct {
		name string
		aud  any
		want bool
	}{
		{
			name: "string match",
			aud:  "statistiloto-ui",
			want: true,
		},
		{
			name: "string no-match",
			aud:  "some-other-client",
			want: false,
		},
		{
			name: "[]string match",
			aud:  []string{"account", "statistiloto-ui"},
			want: true,
		},
		{
			name: "[]string no-match",
			aud:  []string{"account", "other-client"},
			want: false,
		},
		{
			name: "[]any match",
			aud:  []any{"account", "statistiloto-ui"},
			want: true,
		},
		{
			name: "[]any no-match",
			aud:  []any{"account", "other-client"},
			want: false,
		},
		{
			name: "nil input",
			aud:  nil,
			want: false,
		},
		{
			name: "empty string input",
			aud:  "",
			want: false,
		},
		{
			name: "empty []string input",
			aud:  []string{},
			want: false,
		},
		{
			name: "empty []any input",
			aud:  []any{},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := audienceContains(tc.aud, expected)
			if got != tc.want {
				t.Fatalf("audienceContains(%#v, %q) = %v, want %v", tc.aud, expected, got, tc.want)
			}
		})
	}
}
