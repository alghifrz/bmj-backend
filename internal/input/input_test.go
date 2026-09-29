package input

import "testing"

func TestDecimalAcceptsNumberAndNull(t *testing.T) {
	var price Decimal
	if err := price.UnmarshalJSON([]byte("250000.5")); err != nil {
		t.Fatalf("number: %v", err)
	}
	if !price.Valid() || price.Value() != "250000.5" {
		t.Fatalf("price = %+v", price)
	}

	var empty Decimal
	if err := empty.UnmarshalJSON([]byte("null")); err != nil {
		t.Fatalf("null: %v", err)
	}
	if !empty.Null || empty.Value() != nil {
		t.Fatalf("null price = %+v", empty)
	}
}

func TestSlugAndAvailability(t *testing.T) {
	if !ValidSlug("alat-diagnostik") {
		t.Fatal("expected a hyphenated slug")
	}
	if ValidSlug("Alat Diagnostik") {
		t.Fatal("expected spaces to be rejected")
	}
	got, err := NormalizeAvailability("out of stock")
	if err != nil || got != "Out of Stock" {
		t.Fatalf("availability = %q, %v", got, err)
	}
}
