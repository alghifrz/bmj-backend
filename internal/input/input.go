package input

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	slugPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	uuidPattern  = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	pricePattern = regexp.MustCompile(`^\d{1,12}(\.\d{1,2})?$`)
)

// ValidSlug reports whether slug can be stored.
func ValidSlug(slug string) bool {
	return slug != "" && len(slug) <= 200 && slugPattern.MatchString(slug)
}

// ValidUUID reports whether value is a UUID string.
func ValidUUID(value string) bool {
	return uuidPattern.MatchString(value)
}

// NormalizeAvailability accepts the public availability labels.
func NormalizeAvailability(raw string) (string, error) {
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

// CleanText trims a value and treats blank text as NULL.
func CleanText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// Decimal is an optional money value. Omitted, null, and a number are distinct.
type Decimal struct {
	Set  bool
	Null bool
	Text string
}

// UnmarshalJSON accepts a JSON number, a numeric string, or null.
func (d *Decimal) UnmarshalJSON(data []byte) error {
	d.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		d.Null = true
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		d.Text = strings.TrimSpace(text)
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil {
		return fmt.Errorf("invalid price")
	}
	d.Text = number.String()
	return nil
}

// Valid reports whether a provided price can be stored.
func (d Decimal) Valid() bool {
	if !d.Set || d.Null || d.Text == "" {
		return true
	}
	return pricePattern.MatchString(d.Text)
}

// Value returns the database value for the price.
func (d Decimal) Value() any {
	if !d.Set || d.Null || d.Text == "" {
		return nil
	}
	return d.Text
}

// OptionalString distinguishes an omitted field from JSON null.
type OptionalString struct {
	Set   bool
	Value *string
}

// UnmarshalJSON records whether the field was present.
func (o *OptionalString) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	o.Value = &text
	return nil
}

// OptionalInt distinguishes an omitted integer from JSON null.
type OptionalInt struct {
	Set   bool
	Value *int
}

// UnmarshalJSON records whether the field was present.
func (o *OptionalInt) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	var value int
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

// OptionalFloat distinguishes an omitted number from JSON null.
type OptionalFloat struct {
	Set   bool
	Value *float64
}

// UnmarshalJSON records whether the field was present.
func (o *OptionalFloat) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	var value float64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}
