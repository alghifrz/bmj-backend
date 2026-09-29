CREATE TABLE analytics_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL,
    source TEXT NOT NULL,
    product_id UUID REFERENCES products (id) ON DELETE SET NULL,
    visitor_key TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '/',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT analytics_events_kind_check CHECK (kind IN ('page_view', 'whatsapp_click')),
    CONSTRAINT analytics_events_source_check CHECK (
        source IN ('page', 'product_card', 'product_page', 'featured')
    ),
    CONSTRAINT analytics_events_visitor_key_check CHECK (length(visitor_key) BETWEEN 16 AND 64)
);

CREATE INDEX analytics_events_created_at_idx ON analytics_events (created_at);
CREATE INDEX analytics_events_kind_created_idx ON analytics_events (kind, created_at);
CREATE INDEX analytics_events_product_idx ON analytics_events (product_id) WHERE product_id IS NOT NULL;
