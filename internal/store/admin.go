package store

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"bmj-backend/internal/input"
	"bmj-backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type adminLocation struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Address       string    `json:"address"`
	GoogleMapsURL *string   `json:"google_maps_url"`
	Latitude      *float64  `json:"latitude"`
	Longitude     *float64  `json:"longitude"`
	IsPublished   bool      `json:"is_published"`
	DisplayOrder  int       `json:"display_order"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type adminStore struct {
	BusinessName    *string         `json:"business_name"`
	WhatsAppNumber  *string         `json:"whatsapp_number"`
	Email           *string         `json:"email"`
	OperatingHours  *string         `json:"operating_hours"`
	InstagramURL    *string         `json:"instagram_url"`
	FacebookURL     *string         `json:"facebook_url"`
	TikTokURL       *string         `json:"tiktok_url"`
	FooterText      *string         `json:"footer_text"`
	MessageTemplate *string         `json:"whatsapp_message_template"`
	AboutTitle      *string         `json:"about_title"`
	AboutSummary    *string         `json:"about_summary"`
	AboutBody       *string         `json:"about_body"`
	Locations       []adminLocation `json:"locations"`
	UpdatedAt       *time.Time      `json:"updated_at"`
}

type settingsPatch struct {
	BusinessName    input.OptionalString `json:"business_name"`
	WhatsAppNumber  input.OptionalString `json:"whatsapp_number"`
	Email           input.OptionalString `json:"email"`
	OperatingHours  input.OptionalString `json:"operating_hours"`
	InstagramURL    input.OptionalString `json:"instagram_url"`
	FacebookURL     input.OptionalString `json:"facebook_url"`
	TikTokURL       input.OptionalString `json:"tiktok_url"`
	FooterText      input.OptionalString `json:"footer_text"`
	MessageTemplate input.OptionalString `json:"whatsapp_message_template"`
	AboutTitle      input.OptionalString `json:"about_title"`
	AboutSummary    input.OptionalString `json:"about_summary"`
	AboutBody       input.OptionalString `json:"about_body"`
}

type locationWrite struct {
	Name          string   `json:"name"`
	Address       string   `json:"address"`
	GoogleMapsURL *string  `json:"google_maps_url"`
	Latitude      *float64 `json:"latitude"`
	Longitude     *float64 `json:"longitude"`
	IsPublished   bool     `json:"is_published"`
	DisplayOrder  int      `json:"display_order"`
}

type locationPatch struct {
	Name          *string              `json:"name"`
	Address       *string              `json:"address"`
	GoogleMapsURL input.OptionalString `json:"google_maps_url"`
	Latitude      input.OptionalFloat  `json:"latitude"`
	Longitude     input.OptionalFloat  `json:"longitude"`
	IsPublished   *bool                `json:"is_published"`
	DisplayOrder  *int                 `json:"display_order"`
}

const adminLocationSelect = `
SELECT id::text, name, address, google_maps_url, latitude, longitude, is_published, display_order, created_at, updated_at
FROM store_locations`

// AdminGetHandler returns store settings and every location.
func AdminGetHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		item, err := getAdminStore(ctx, pool)
		if err != nil {
			writeStoreDBError(c, err)
			return
		}
		response.JSONData(c, item)
	}
}

// AdminUpdateHandler updates the singleton store settings row.
func AdminUpdateHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body settingsPatch
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		update := settingsUpdate(body)
		if update.Empty() {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		if err := saveSettings(ctx, pool, body, update); err != nil {
			writeStoreDBError(c, err)
			return
		}

		item, err := getAdminStore(ctx, pool)
		if err != nil {
			writeStoreDBError(c, err)
			return
		}
		response.JSONData(c, item)
	}
}

// AdminCreateLocationHandler inserts a store location.
func AdminCreateLocationHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body locationWrite
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		name := strings.TrimSpace(body.Name)
		address := strings.TrimSpace(body.Address)
		if name == "" || address == "" || !coordinatesTogether(body.Latitude, body.Longitude) {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		var id string
		err := pool.QueryRow(ctx, `
INSERT INTO store_locations (name, address, google_maps_url, latitude, longitude, is_published, display_order)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id::text`,
			name,
			address,
			input.CleanText(body.GoogleMapsURL),
			body.Latitude,
			body.Longitude,
			body.IsPublished,
			body.DisplayOrder,
		).Scan(&id)
		if err != nil {
			writeStoreDBError(c, err)
			return
		}

		item, err := findAdminLocation(ctx, pool, id)
		if err != nil {
			writeStoreDBError(c, err)
			return
		}
		response.JSONStatus(c, http.StatusCreated, item)
	}
}

// AdminUpdateLocationHandler applies a partial location update.
func AdminUpdateLocationHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if !input.ValidUUID(id) {
			response.JSONError(c, http.StatusNotFound, "LOCATION_NOT_FOUND", "Location not found")
			return
		}

		var body locationPatch
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		update, err := locationUpdate(body)
		if err != nil || update.Empty() {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		query, args := update.SQL("store_locations", id)
		tag, err := pool.Exec(ctx, query, args...)
		if err != nil {
			writeStoreDBError(c, err)
			return
		}
		if tag.RowsAffected() == 0 {
			response.JSONError(c, http.StatusNotFound, "LOCATION_NOT_FOUND", "Location not found")
			return
		}

		item, err := findAdminLocation(ctx, pool, id)
		if err != nil {
			writeStoreDBError(c, err)
			return
		}
		response.JSONData(c, item)
	}
}

// AdminDeleteLocationHandler removes a store location.
func AdminDeleteLocationHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if !input.ValidUUID(id) {
			response.JSONError(c, http.StatusNotFound, "LOCATION_NOT_FOUND", "Location not found")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		tag, err := pool.Exec(ctx, `DELETE FROM store_locations WHERE id = $1::uuid`, id)
		if err != nil {
			writeStoreDBError(c, err)
			return
		}
		if tag.RowsAffected() == 0 {
			response.JSONError(c, http.StatusNotFound, "LOCATION_NOT_FOUND", "Location not found")
			return
		}
		response.JSONData(c, gin.H{"deleted": true})
	}
}

func getAdminStore(ctx context.Context, pool *pgxpool.Pool) (adminStore, error) {
	item, err := getAdminSettings(ctx, pool)
	if err != nil {
		return adminStore{}, err
	}
	locations, err := listAdminLocations(ctx, pool)
	if err != nil {
		return adminStore{}, err
	}
	item.Locations = locations
	return item, nil
}

func getAdminSettings(ctx context.Context, pool *pgxpool.Pool) (adminStore, error) {
	const settingsSQL = `
SELECT business_name, whatsapp_number, email, operating_hours, instagram_url, facebook_url, tiktok_url, footer_text, whatsapp_message_template, about_title, about_summary, about_body, updated_at
FROM store_settings
ORDER BY created_at ASC
LIMIT 1`

	var item adminStore
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
		return adminStore{Locations: []adminLocation{}}, nil
	}
	if err != nil {
		return adminStore{}, err
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
	item.Locations = []adminLocation{}
	return item, nil
}

func saveSettings(ctx context.Context, pool *pgxpool.Pool, body settingsPatch, update input.Update) error {
	id, err := settingsID(ctx, pool)
	if errors.Is(err, pgx.ErrNoRows) {
		insertErr := insertSettings(ctx, pool, body)
		if insertErr == nil {
			return nil
		}
		if !uniqueViolation(insertErr) {
			return insertErr
		}
		id, err = settingsID(ctx, pool)
	}
	if err != nil {
		return err
	}

	query, args := update.SQL("store_settings", id)
	_, err = pool.Exec(ctx, query, args...)
	return err
}

func settingsID(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `SELECT id::text FROM store_settings ORDER BY created_at ASC LIMIT 1`).Scan(&id)
	return id, err
}

func insertSettings(ctx context.Context, pool *pgxpool.Pool, body settingsPatch) error {
	_, err := pool.Exec(ctx, `
INSERT INTO store_settings (
    business_name, whatsapp_number, email, operating_hours, instagram_url, facebook_url, tiktok_url, footer_text, whatsapp_message_template, about_title, about_summary, about_body
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		optionalText(body.BusinessName),
		optionalText(body.WhatsAppNumber),
		optionalText(body.Email),
		optionalText(body.OperatingHours),
		optionalText(body.InstagramURL),
		optionalText(body.FacebookURL),
		optionalText(body.TikTokURL),
		optionalText(body.FooterText),
		optionalText(body.MessageTemplate),
		optionalText(body.AboutTitle),
		optionalText(body.AboutSummary),
		optionalText(body.AboutBody),
	)
	return err
}

func settingsUpdate(body settingsPatch) input.Update {
	var update input.Update
	setSetting(&update, "business_name", body.BusinessName)
	setSetting(&update, "whatsapp_number", body.WhatsAppNumber)
	setSetting(&update, "email", body.Email)
	setSetting(&update, "operating_hours", body.OperatingHours)
	setSetting(&update, "instagram_url", body.InstagramURL)
	setSetting(&update, "facebook_url", body.FacebookURL)
	setSetting(&update, "tiktok_url", body.TikTokURL)
	setSetting(&update, "footer_text", body.FooterText)
	setSetting(&update, "whatsapp_message_template", body.MessageTemplate)
	setSetting(&update, "about_title", body.AboutTitle)
	setSetting(&update, "about_summary", body.AboutSummary)
	setSetting(&update, "about_body", body.AboutBody)
	return update
}

func setSetting(update *input.Update, column string, value input.OptionalString) {
	if !value.Set {
		return
	}
	update.Set(column, input.CleanText(value.Value))
}

func optionalText(value input.OptionalString) any {
	if !value.Set {
		return nil
	}
	return input.CleanText(value.Value)
}

func locationUpdate(body locationPatch) (input.Update, error) {
	var update input.Update
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			return update, errInvalidLocation
		}
		update.Set("name", name)
	}
	if body.Address != nil {
		address := strings.TrimSpace(*body.Address)
		if address == "" {
			return update, errInvalidLocation
		}
		update.Set("address", address)
	}
	if body.GoogleMapsURL.Set {
		update.Set("google_maps_url", input.CleanText(body.GoogleMapsURL.Value))
	}
	if body.Latitude.Set || body.Longitude.Set {
		if !body.Latitude.Set || !body.Longitude.Set || !coordinatesTogether(body.Latitude.Value, body.Longitude.Value) {
			return update, errInvalidLocation
		}
		update.Set("latitude", body.Latitude.Value)
		update.Set("longitude", body.Longitude.Value)
	}
	if body.IsPublished != nil {
		update.Set("is_published", *body.IsPublished)
	}
	if body.DisplayOrder != nil {
		update.Set("display_order", *body.DisplayOrder)
	}
	return update, nil
}

func coordinatesTogether(latitude, longitude *float64) bool {
	return (latitude == nil && longitude == nil) || (latitude != nil && longitude != nil)
}

func listAdminLocations(ctx context.Context, pool *pgxpool.Pool) ([]adminLocation, error) {
	rows, err := pool.Query(ctx, adminLocationSelect+` ORDER BY display_order ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]adminLocation, 0)
	for rows.Next() {
		item, err := scanAdminLocation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func findAdminLocation(ctx context.Context, pool *pgxpool.Pool, id string) (adminLocation, error) {
	return scanAdminLocation(pool.QueryRow(ctx, adminLocationSelect+` WHERE id = $1::uuid`, id))
}

type locationScanner interface {
	Scan(dest ...any) error
}

func scanAdminLocation(row locationScanner) (adminLocation, error) {
	var item adminLocation
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Address,
		&item.GoogleMapsURL,
		&item.Latitude,
		&item.Longitude,
		&item.IsPublished,
		&item.DisplayOrder,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return adminLocation{}, err
	}
	item.GoogleMapsURL = blankToNil(item.GoogleMapsURL)
	return item, nil
}

func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func writeStoreDBError(c *gin.Context, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(c, http.StatusNotFound, "LOCATION_NOT_FOUND", "Location not found")
		return
	}
	if response.JSONConstraint(c, err) {
		return
	}
	response.JSONQueryError(c, err)
}

var errInvalidLocation = errors.New("invalid location")
