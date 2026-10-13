package envfile

import (
	"os"
	"testing"
)

func TestProducerFileContract(t *testing.T) {
	for _, tt := range []struct {
		name, raw string
		names     []string
		bad       bool
	}{
		{"empty readiness", "", nil, false}, {"literal values", "A=\"literal=value\"\r\nB=\nEXTRA=ignored", []string{"A", "B"}, false},
		{"duplicate", "A=one\nA=two", []string{"A"}, true}, {"malformed", "export A=x", []string{"A"}, true}, {"missing", "B=one", []string{"A"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, clean, err := Prepare()
			if err != nil {
				t.Fatal(err)
			}
			defer clean()
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Fatal("path already present")
			}
			if err := os.WriteFile(p, []byte(tt.raw), 0600); err != nil {
				t.Fatal(err)
			}
			v, err := Read(p, tt.names)
			if (err != nil) != tt.bad {
				t.Fatalf("%v %v", v, err)
			}
			if !tt.bad && len(v) != len(tt.names) {
				t.Fatal(v)
			}
		})
	}
}
