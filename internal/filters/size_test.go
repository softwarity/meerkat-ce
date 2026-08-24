package filters

import "testing"

func TestParseSize(t *testing.T) {
	ok := map[string]int64{
		"2MB": 2 << 20, "2 mb": 2 << 20, "512KB": 512 << 10, "16k": 16 << 10,
		"1048576": 1 << 20, "900B": 900, "1GB": 1 << 30, "1.5MB": 1572864,
	}
	for in, want := range ok {
		got, err := ParseSize(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got != want {
			t.Fatalf("%q gave %d, want %d", in, got, want)
		}
	}
	// Zero is not "no limit", it is "refuse everything" - and the error has to
	// say where the answer actually is, because it is not a number.
	for _, bad := range []string{"", "0", "-1", "two megs", "MB", "1TB"} {
		if _, err := ParseSize(bad); err == nil {
			t.Fatalf("%q was accepted", bad)
		}
	}
}

func TestFormatBytesReadsBackTheWayItWasTyped(t *testing.T) {
	cases := map[int64]string{
		2 << 20: "2 MB", 512 << 10: "512 KB", 900: "900 bytes",
		// An exact whole number wins over a decimal one: 1536 KB is the same
		// size as 1.5 MB and cannot be misread as a rounding.
		1572864: "1536 KB", 1500: "1.5 KB",
	}
	for in, want := range cases {
		if got := FormatBytes(in); got != want {
			t.Fatalf("%d gave %q, want %q", in, got, want)
		}
	}
}
