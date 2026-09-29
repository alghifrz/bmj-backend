package products

import (
	"net/url"
	"testing"
)

func TestParseListQueryDefaults(t *testing.T) {
	query, err := ParseListQuery(url.Values{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if query.Page != 1 || query.Limit != 12 || query.Offset != 0 || query.Sort != "featured" {
		t.Fatalf("query = %+v", query)
	}
	if query.Search != "" || query.Featured != "" {
		t.Fatalf("filters = %+v", query)
	}
}

func TestParseListQueryFilters(t *testing.T) {
	values := url.Values{}
	values.Set("page", "2")
	values.Set("limit", "100")
	values.Set("search", "100%_ox")
	values.Set("category", "alat-diagnostik")
	values.Set("brand", "Omron")
	values.Set("availability", "out of stock")
	values.Set("featured", "true")
	values.Set("sort", "price-desc")

	query, err := ParseListQuery(values)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if query.Page != 2 || query.Limit != 48 || query.Offset != 48 {
		t.Fatalf("paging = %+v", query)
	}
	if query.Search != `%100\%\_ox%` {
		t.Fatalf("search = %q", query.Search)
	}
	if query.Availability != "Out of Stock" || query.Featured != "true" || query.Sort != "price_desc" {
		t.Fatalf("filters = %+v", query)
	}
	if query.Category != "alat-diagnostik" || query.Brand != "Omron" {
		t.Fatalf("category/brand = %+v", query)
	}
}

func TestParseListQueryRejectsInvalidSort(t *testing.T) {
	values := url.Values{}
	values.Set("sort", "cheapest")
	if _, err := ParseListQuery(values); err == nil {
		t.Fatal("expected invalid sort to fail")
	}
}
