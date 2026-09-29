package products

import "testing"

func TestProductPatchRejectsEmptyBody(t *testing.T) {
	update, err := productUpdate(productPatch{})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !update.Empty() {
		t.Fatal("expected an empty patch")
	}
}

func TestProductPatchClearsPrice(t *testing.T) {
	var body productPatch
	if err := body.Price.UnmarshalJSON([]byte("null")); err != nil {
		t.Fatalf("price: %v", err)
	}

	update, err := productUpdate(body)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	query, args := update.SQL("products", "00000000-0000-4000-8000-000000000001")
	if update.Empty() || len(args) != 2 || args[0] != nil {
		t.Fatalf("query = %s args = %#v", query, args)
	}
}
