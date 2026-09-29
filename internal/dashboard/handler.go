package dashboard

import (
	"context"
	"strconv"
	"time"

	"bmj-backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

const queryTimeout = 5 * time.Second

type monthCount struct {
	Label string `json:"label"`
	Year  int    `json:"year"`
	Month int    `json:"month"`
	Count int    `json:"count"`
}

type categoryCount struct {
	Name      string `json:"name"`
	Total     int    `json:"total"`
	Published int    `json:"published"`
}

type recentProduct struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	IsPublished bool      `json:"is_published"`
	IsFeatured  bool      `json:"is_featured"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type whatsAppProduct struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Card     int    `json:"card_clicks"`
	Page     int    `json:"page_clicks"`
	Featured int    `json:"featured_clicks"`
}

type insightDay struct {
	Label     string `json:"label"`
	Visitors  int    `json:"visitors"`
	PageViews int    `json:"page_views"`
	Clicks    int    `json:"clicks"`
}

type summary struct {
	ProductsTotal           int               `json:"products_total"`
	ProductsPublished       int               `json:"products_published"`
	ProductsAddedThisMonth  int               `json:"products_added_this_month"`
	CategoriesTotal         int               `json:"categories_total"`
	CategoriesActive        int               `json:"categories_active"`
	ReviewsPublished        int               `json:"reviews_published"`
	RatingAverage           *float64          `json:"rating_average"`
	Visitors30d             int               `json:"visitors_30d"`
	PageViews30d            int               `json:"page_views_30d"`
	WhatsAppCardClicks      int               `json:"whatsapp_card_clicks"`
	WhatsAppCardClicksMonth int               `json:"whatsapp_card_clicks_month"`
	WhatsAppProducts        []whatsAppProduct `json:"whatsapp_products"`
	InsightDays             []insightDay      `json:"insight_days"`
	Months                  []monthCount      `json:"months"`
	Categories              []categoryCount   `json:"categories"`
	RecentProducts          []recentProduct   `json:"recent_products"`
}

// AdminHandler returns catalog totals for the admin dashboard.
func AdminHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		item, err := loadSummary(ctx, pool)
		if err != nil {
			response.JSONQueryError(c, err)
			return
		}
		response.JSONData(c, item)
	}
}

func loadSummary(ctx context.Context, pool *pgxpool.Pool) (summary, error) {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		location = time.FixedZone("WIB", 7*60*60)
	}
	now := time.Now().In(location)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	seriesStart := monthStart.AddDate(0, -11, 0)

	var item summary
	err = pool.QueryRow(ctx, `
SELECT
    (SELECT count(*) FROM products),
    (SELECT count(*) FROM products WHERE is_published),
    (SELECT count(*) FROM products WHERE created_at >= $1),
    (SELECT count(*) FROM categories),
    (SELECT count(*) FROM categories WHERE is_active),
    (SELECT count(*) FROM reviews WHERE is_published),
    (SELECT round(avg(rating)::numeric, 1) FROM reviews WHERE is_published AND rating IS NOT NULL)
`, monthStart).Scan(
		&item.ProductsTotal,
		&item.ProductsPublished,
		&item.ProductsAddedThisMonth,
		&item.CategoriesTotal,
		&item.CategoriesActive,
		&item.ReviewsPublished,
		&item.RatingAverage,
	)
	if err != nil {
		return summary{}, err
	}

	item.Months, err = loadMonths(ctx, pool, seriesStart, monthStart)
	if err != nil {
		return summary{}, err
	}
	item.Categories, err = loadCategories(ctx, pool)
	if err != nil {
		return summary{}, err
	}
	item.RecentProducts, err = loadRecentProducts(ctx, pool)
	if err != nil {
		return summary{}, err
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -13)
	if err := loadInsight(ctx, pool, &item, monthStart, now.AddDate(0, 0, -30), dayStart); err != nil {
		return summary{}, err
	}
	return item, nil
}

func loadInsight(ctx context.Context, pool *pgxpool.Pool, item *summary, monthStart, since, dayStart time.Time) error {
	item.WhatsAppProducts = []whatsAppProduct{}
	item.InsightDays = []insightDay{}
	err := pool.QueryRow(ctx, `
SELECT
    (SELECT count(DISTINCT visitor_key) FROM analytics_events WHERE kind = 'page_view' AND created_at >= $1),
    (SELECT count(*) FROM analytics_events WHERE kind = 'page_view' AND created_at >= $1),
    (SELECT count(*) FROM analytics_events WHERE kind = 'whatsapp_click' AND source = 'product_card'),
    (SELECT count(*) FROM analytics_events WHERE kind = 'whatsapp_click' AND source = 'product_card' AND created_at >= $2)
`, since, monthStart).Scan(
		&item.Visitors30d,
		&item.PageViews30d,
		&item.WhatsAppCardClicks,
		&item.WhatsAppCardClicksMonth,
	)
	if err != nil {
		return err
	}

	rows, err := pool.Query(ctx, `
SELECT p.name, p.slug,
    count(*) FILTER (WHERE e.source = 'product_card'),
    count(*) FILTER (WHERE e.source = 'product_page'),
    count(*) FILTER (WHERE e.source = 'featured')
FROM analytics_events e
JOIN products p ON p.id = e.product_id
WHERE e.kind = 'whatsapp_click'
GROUP BY p.id, p.name, p.slug
ORDER BY count(*) FILTER (WHERE e.source = 'product_card') DESC, count(*) DESC, p.name ASC
LIMIT 8
`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var product whatsAppProduct
		if err := rows.Scan(&product.Name, &product.Slug, &product.Card, &product.Page, &product.Featured); err != nil {
			return err
		}
		item.WhatsAppProducts = append(item.WhatsAppProducts, product)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return loadInsightDays(ctx, pool, item, dayStart)
}

func loadInsightDays(ctx context.Context, pool *pgxpool.Pool, item *summary, start time.Time) error {
	rows, err := pool.Query(ctx, `
SELECT (created_at AT TIME ZONE 'Asia/Jakarta')::date AS day,
    count(DISTINCT visitor_key) FILTER (WHERE kind = 'page_view'),
    count(*) FILTER (WHERE kind = 'page_view'),
    count(*) FILTER (WHERE kind = 'whatsapp_click' AND source = 'product_card')
FROM analytics_events
WHERE created_at >= $1
GROUP BY 1
`, start)
	if err != nil {
		return err
	}
	defer rows.Close()

	type dayCount struct {
		visitors  int
		pageViews int
		clicks    int
	}
	counts := map[string]dayCount{}
	for rows.Next() {
		var day time.Time
		var count dayCount
		if err := rows.Scan(&day, &count.visitors, &count.pageViews, &count.clicks); err != nil {
			return err
		}
		counts[day.Format("2006-01-02")] = count
	}
	if err := rows.Err(); err != nil {
		return err
	}

	location := start.Location()
	end := time.Date(time.Now().In(location).Year(), time.Now().In(location).Month(), time.Now().In(location).Day(), 0, 0, 0, 0, location)
	item.InsightDays = make([]insightDay, 0, 14)
	for cursor := start; !cursor.After(end); cursor = cursor.AddDate(0, 0, 1) {
		count := counts[cursor.Format("2006-01-02")]
		item.InsightDays = append(item.InsightDays, insightDay{
			Label:     fmtDay(cursor),
			Visitors:  count.visitors,
			PageViews: count.pageViews,
			Clicks:    count.clicks,
		})
	}
	return nil
}

func fmtDay(day time.Time) string {
	return strconv.Itoa(day.Day()) + "/" + strconv.Itoa(int(day.Month()))
}

func loadMonths(ctx context.Context, pool *pgxpool.Pool, start, end time.Time) ([]monthCount, error) {
	rows, err := pool.Query(ctx, `
SELECT date_trunc('month', created_at AT TIME ZONE 'Asia/Jakarta') AS month, count(*)
FROM products
WHERE created_at >= $1
GROUP BY 1
`, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var month time.Time
		var count int
		if err := rows.Scan(&month, &count); err != nil {
			return nil, err
		}
		counts[month.Format("2006-01")] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	items := make([]monthCount, 0, 12)
	for cursor := start; !cursor.After(end); cursor = cursor.AddDate(0, 1, 0) {
		items = append(items, monthCount{
			Label: cursor.Format("2006-01"),
			Year:  cursor.Year(),
			Month: int(cursor.Month()),
			Count: counts[cursor.Format("2006-01")],
		})
	}
	return items, nil
}

func loadCategories(ctx context.Context, pool *pgxpool.Pool) ([]categoryCount, error) {
	rows, err := pool.Query(ctx, `
SELECT c.name, count(p.id), count(p.id) FILTER (WHERE p.is_published)
FROM categories c
LEFT JOIN products p ON p.category_id = c.id
GROUP BY c.id, c.name, c.display_order
ORDER BY c.display_order ASC, c.name ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]categoryCount, 0)
	for rows.Next() {
		var item categoryCount
		if err := rows.Scan(&item.Name, &item.Total, &item.Published); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadRecentProducts(ctx context.Context, pool *pgxpool.Pool) ([]recentProduct, error) {
	rows, err := pool.Query(ctx, `
SELECT p.id::text, p.name, c.name, p.is_published, p.is_featured, p.updated_at
FROM products p
JOIN categories c ON c.id = p.category_id
ORDER BY p.updated_at DESC
LIMIT 8
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]recentProduct, 0)
	for rows.Next() {
		var item recentProduct
		if err := rows.Scan(&item.ID, &item.Name, &item.Category, &item.IsPublished, &item.IsFeatured, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
