package base62

import "testing"

func TestEncodeKnownValues(t *testing.T) {
	cases := map[uint64]string{
		0:  "0",
		1:  "1",
		61: "z",
		62: "10",
		63: "11",
	}
	for n, want := range cases {
		if got := Encode(n); got != want {
			t.Errorf("Encode(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	for _, n := range []uint64{0, 1, 61, 62, 12345, 1_000_000, 1 << 40, ^uint64(0)} {
		s := Encode(n)
		got, err := Decode(s)
		if err != nil {
			t.Fatalf("Decode(%q): %v", s, err)
		}
		if got != n {
			t.Errorf("round trip %d -> %q -> %d", n, s, got)
		}
	}
}

func TestDecodeRejectsInvalid(t *testing.T) {
	for _, s := range []string{"", "abc-def", "hello!", "with space"} {
		if _, err := Decode(s); err == nil {
			t.Errorf("Decode(%q) expected error", s)
		}
	}
}

func TestEncodeIsMonotonicLength(t *testing.T) {
	// Distinct inputs must produce distinct codes (structural uniqueness).
	seen := make(map[string]uint64)
	for n := uint64(0); n < 5000; n++ {
		s := Encode(n)
		if prev, ok := seen[s]; ok {
			t.Fatalf("collision: Encode(%d) == Encode(%d) == %q", n, prev, s)
		}
		seen[s] = n
	}
}
