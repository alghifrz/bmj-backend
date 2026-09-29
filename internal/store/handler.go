package store

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"bmj-backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const queryTimeout = 5 * time.Second

type locationResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Address       string   `json:"address"`
	GoogleMapsURL *string  `json:"google_maps_url"`
	Latitude      *float64 `json:"latitude"`
	Longitude     *float64 `json:"longitude"`
	DisplayOrder  int      `json:"display_order"`
}

type storeResponse struct {
	BusinessName    *string            `json:"business_name"`
	WhatsAppNumber  *string            `json:"whatsapp_number"`
	Email           *string            `json:"email"`
	OperatingHours  *string            `json:"operating_hours"`
	InstagramURL    *string            `json:"instagram_url"`
	FacebookURL     *string            `json:"facebook_url"`
	TikTokURL       *string            `json:"tiktok_url"`
	FooterText      *string            `json:"footer_text"`
	MessageTemplate *string            `json:"whatsapp_message_template"`
	AboutTitle      *string            `json:"about_title"`
	AboutSummary    *string            `json:"about_summary"`
	AboutBody       *string            `json:"about_body"`
	Locations       []locationResponse `json:"locations"`
	UpdatedAt       *time.Time         `json:"updated_at"`
}

// GetHandler returns public store settings and published locations.
func GetHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if pool == nil {
			response.JSONError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		item, err := getStore(ctx, pool)
		if err != nil {
			response.JSONQueryError(c, err)
			return
		}

		response.JSONData(c, item)
	}
}

func getStore(ctx context.Context, pool *pgxpool.Pool) (storeResponse, error) {
	item, err := getSettings(ctx, pool)
	if err != nil {
		return storeResponse{}, err
	}

	locations, err := listLocations(ctx, pool)
	if err != nil {
		return storeResponse{}, err
	}
	item.Locations = locations
	return item, nil
}

func getSettings(ctx context.Context, pool *pgxpool.Pool) (storeResponse, error) {
	const settingsSQL = `
SELECT
    business_name,
    whatsapp_number,
    email,
    operating_hours,
    instagram_url,
    facebook_url,
    tiktok_url,
    footer_text,
    whatsapp_message_template,
    about_title,
    about_summary,
    about_body,
    updated_at
FROM store_settings
ORDER BY created_at ASC
LIMIT 1`

	var item storeResponse
	err := pool.QueryRow(ctx, settingsSQL).Scan(
		&item.BusinessName,
		&item.WhatsAppNumber,
		&item.Email,
		&item.OperatingHours,
		&item.InstagramURL,
		&item.FacebookURL,
		&item.TikTokURL,
		&item.FooterText,
		&item.MessageTemplate,
		&item.AboutTitle,
		&item.AboutSummary,
		&item.AboutBody,
		&item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return storeResponse{Locations: []locationResponse{}}, nil
	}
	if err != nil {
		return storeResponse{}, err
	}

	item.BusinessName = blankToNil(item.BusinessName)
	item.WhatsAppNumber = blankToNil(item.WhatsAppNumber)
	item.Email = blankToNil(item.Email)
	item.OperatingHours = blankToNil(item.OperatingHours)
	item.InstagramURL = blankToNil(item.InstagramURL)
	item.FacebookURL = blankToNil(item.FacebookURL)
	item.TikTokURL = blankToNil(item.TikTokURL)
	item.FooterText = blankToNil(item.FooterText)
	item.MessageTemplate = blankToNil(item.MessageTemplate)
	item.AboutTitle = blankToNil(item.AboutTitle)
	item.AboutSummary = blankToNil(item.AboutSummary)
	item.AboutBody = blankToNil(item.AboutBody)
	item.Locations = []locationResponse{}
	return item, nil
}

func listLocations(ctx context.Context, pool *pgxpool.Pool) ([]locationResponse, error) {
	const locationSQL = `
SELECT id::text, name, address, google_maps_url, latitude, longitude, display_order
FROM store_locations
WHERE is_published
ORDER BY display_order ASC, name ASC`

	rows, err := pool.Query(ctx, locationSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	locations := make([]locationResponse, 0)
	for rows.Next() {
		var location locationResponse
		if err := rows.Scan(
			&location.ID,
			&location.Name,
			&location.Address,
			&location.GoogleMapsURL,
			&location.Latitude,
			&location.Longitude,
			&location.DisplayOrder,
		); err != nil {
			return nil, err
		}
		location.GoogleMapsURL = blankToNil(location.GoogleMapsURL)
		locations = append(locations, location)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return locations, nil
}

func blankToNil(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
