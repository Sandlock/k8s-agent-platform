package provider

import "testing"

func TestParseHandle(t *testing.T) {
	cases := []struct {
		ref     string
		wantNS  string
		wantN   string
		wantErr bool
	}{
		{"sandboxes/sb-abc123", "sandboxes", "sb-abc123", false},
		{"default/foo", "default", "foo", false},
		{"noslash", "", "", true},
		{"", "", "", true},
	}
	for _, tc := range cases {
		h, err := ParseHandle(tc.ref)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseHandle(%q): want error, got nil", tc.ref)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseHandle(%q): unexpected error: %v", tc.ref, err)
			continue
		}
		if h.Namespace != tc.wantNS || h.Name != tc.wantN {
			t.Errorf("ParseHandle(%q) = {%s, %s}, want {%s, %s}",
				tc.ref, h.Namespace, h.Name, tc.wantNS, tc.wantN)
		}
	}
}

func TestHandleString(t *testing.T) {
	h := Handle{Namespace: "sandboxes", Name: "sb-xyz"}
	if got := h.String(); got != "sandboxes/sb-xyz" {
		t.Errorf("got %q, want %q", got, "sandboxes/sb-xyz")
	}
}
