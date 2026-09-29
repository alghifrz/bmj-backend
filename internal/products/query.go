package products

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	defaultPage  = 1
	defaultLimit = 12
	maxLimit     = 48
	maxPage      = 10000
	maxSearchLen = 100
)

// ListQuery is the public product catalog filter.
type ListQuery struct {
	Page         int
	Limit        int
	Offset       int
	Search       string
	Category     string
	Brand        string
	Availability string
	Featured     string
	Sort         string
}

// ParseListQuery validates catalog query parameters.
func ParseListQuery(values url.Values) (ListQuery, error) {
	page, err := parsePositive(values.Get("page"), defaultPage)
	if err != nil || page > maxPage {
		return ListQuery{}, fmt.Errorf("invalid page")
	}

	limit, err := parsePositive(values.Get("limit"), defaultLimit)
	if err != nil {
		return ListQuery{}, fmt.Errorf("invalid limit")
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	sort, err := normalizeSort(values.Get("sort"))
	if err != nil {
		return ListQuery{}, err
	}

	availability, err := normalizeAvailability(values.Get("availability"))
	if err != nil {
		return ListQuery{}, err
	}

	featured, err := normalizeFeatured(values.Get("featured"))
	if err != nil {
		return ListQuery{}, err
	}

	search := strings.TrimSpace(values.Get("search"))
	if len(search) > maxSearchLen {
		return ListQuery{}, fmt.Errorf("invalid search")
	}

	return ListQuery{
		Page:         page,
		Limit:        limit,
		Offset:       (page - 1) * limit,
		Search:       likePattern(search),
		Category:     strings.TrimSpace(values.Get("category")),
		Brand:        strings.TrimSpace(values.Get("brand")),
		Availability: availability,
		Featured:     featured,
		Sort:         sort,
	}, nil
}

func parsePositive(raw string, fallback int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("invalid number")
	}
	return value, nil
}

func normalizeSort(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "featured":
		return "featured", nil
	case "newest":
		return "newest", nil
	case "name_asc", "name-asc":
		return "name_asc", nil
	case "name_desc", "name-desc":
		return "name_desc", nil
	case "price_asc", "price-asc":
		return "price_asc", nil
	case "price_desc", "price-desc":
		return "price_desc", nil
	default:
		return "", fmt.Errorf("invalid sort")
	}
}

func normalizeAvailability(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return "", nil
	case "available":
		return "Available", nil
	case "contact us", "contact_us":
		return "Contact Us", nil
	case "out of stock", "out_of_stock":
		return "Out of Stock", nil
	default:
		return "", fmt.Errorf("invalid availability")
	}
}

func normalizeFeatured(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return "", nil
	case "true", "1":
		return "true", nil
	case "false", "0":
		return "false", nil
	default:
		return "", fmt.Errorf("invalid featured")
	}
}

func likePattern(search string) string {
	if search == "" {
		return ""
	}
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(search)
	return "%" + escaped + "%"
}

func orderBy(sort string) string {
	switch sort {
	case "newest":
		return "p.created_at DESC, p.id ASC"
	case "name_asc":
		return "p.name ASC, p.id ASC"
	case "name_desc":
		return "p.name DESC, p.id ASC"
	case "price_asc":
		return "CASE WHEN p.price_visible THEN p.price END ASC NULLS LAST, p.name ASC, p.id ASC"
	case "price_desc":
		return "CASE WHEN p.price_visible THEN p.price END DESC NULLS LAST, p.name ASC, p.id ASC"
	default:
		return "p.is_featured DESC, p.display_order ASC, p.created_at DESC, p.id ASC"
	}
}
