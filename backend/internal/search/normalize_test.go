package search

import "testing"

func TestNormalize(t *testing.T) {
	tests := map[string]string{
		"  Bicicletá   Aro 29 ": "bicicleta aro 29",
		"BICICLETA":             "bicicleta",
		"São Paulo":             "sao paulo",
	}
	for input, want := range tests {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSearchText(t *testing.T) {
	if got := SearchText("Bicicletá", "Quadro de alumínio"); got != "bicicleta quadro de aluminio" {
		t.Fatalf("SearchText() = %q", got)
	}
}
