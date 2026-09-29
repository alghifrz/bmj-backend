CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE TABLE admins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admins_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT admins_email_not_blank CHECK (length(btrim(email)) > 0),
    CONSTRAINT admins_password_hash_not_blank CHECK (length(btrim(password_hash)) > 0)
);

CREATE UNIQUE INDEX admins_email_lower_idx ON admins (lower(email));

CREATE TRIGGER admins_set_updated_at
    BEFORE UPDATE ON admins
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,
    image_url TEXT,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT categories_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT categories_slug_not_blank CHECK (length(btrim(slug)) > 0),
    CONSTRAINT categories_slug_unique UNIQUE (slug)
);

CREATE INDEX categories_is_active_idx ON categories (is_active);

CREATE TRIGGER categories_set_updated_at
    BEFORE UPDATE ON categories
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    short_description TEXT,
    description TEXT,
    brand TEXT,
    model TEXT,
    material TEXT,
    size TEXT,
    function TEXT,
    included_components TEXT,
    specifications TEXT,
    additional_information TEXT,
    price NUMERIC(14, 2),
    price_visible BOOLEAN NOT NULL DEFAULT FALSE,
    availability TEXT,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT products_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT products_slug_not_blank CHECK (length(btrim(slug)) > 0),
    CONSTRAINT products_slug_unique UNIQUE (slug),
    CONSTRAINT products_price_non_negative CHECK (price IS NULL OR price >= 0),
    CONSTRAINT products_availability_check CHECK (
        availability IS NULL
        OR availability IN ('Available', 'Contact Us', 'Out of Stock')
    )
);

CREATE INDEX products_category_id_idx ON products (category_id);
CREATE INDEX products_is_published_idx ON products (is_published);
CREATE INDEX products_is_featured_idx ON products (is_featured);

CREATE TRIGGER products_set_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE product_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    storage_path TEXT NOT NULL,
    image_url TEXT,
    alt_text TEXT NOT NULL DEFAULT '',
    display_order INTEGER NOT NULL DEFAULT 0,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT product_images_storage_path_not_blank CHECK (length(btrim(storage_path)) > 0)
);

CREATE INDEX product_images_product_id_idx ON product_images (product_id);
CREATE UNIQUE INDEX product_images_one_primary_idx
    ON product_images (product_id)
    WHERE is_primary;

CREATE TRIGGER product_images_set_updated_at
    BEFORE UPDATE ON product_images
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_name TEXT NOT NULL,
    customer_role_or_organization TEXT,
    review_text TEXT NOT NULL,
    rating SMALLINT,
    customer_image TEXT,
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reviews_customer_name_not_blank CHECK (length(btrim(customer_name)) > 0),
    CONSTRAINT reviews_text_not_blank CHECK (length(btrim(review_text)) > 0),
    CONSTRAINT reviews_rating_range CHECK (rating IS NULL OR rating BETWEEN 1 AND 5)
);

CREATE INDEX reviews_is_published_idx ON reviews (is_published);
CREATE INDEX reviews_is_featured_idx ON reviews (is_featured);

CREATE TRIGGER reviews_set_updated_at
    BEFORE UPDATE ON reviews
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE store_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_name TEXT,
    whatsapp_number TEXT,
    email TEXT,
    operating_hours TEXT,
    instagram_url TEXT,
    facebook_url TEXT,
    tiktok_url TEXT,
    footer_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX store_settings_singleton_idx ON store_settings ((TRUE));

CREATE TRIGGER store_settings_set_updated_at
    BEFORE UPDATE ON store_settings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE store_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    address TEXT NOT NULL,
    google_maps_url TEXT,
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT store_locations_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT store_locations_address_not_blank CHECK (length(btrim(address)) > 0),
    CONSTRAINT store_locations_coordinates_check CHECK (
        (latitude IS NULL AND longitude IS NULL)
        OR (
            latitude BETWEEN -90 AND 90
            AND longitude BETWEEN -180 AND 180
        )
    )
);

CREATE INDEX store_locations_is_published_idx ON store_locations (is_published);

CREATE TRIGGER store_locations_set_updated_at
    BEFORE UPDATE ON store_locations
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

INSERT INTO store_locations (name, address, google_maps_url, is_published, display_order)
VALUES
    (
        'Pasar Pramuka, Lantai 1 AKS 36',
        'Jl. Pramuka, RT.1/RW.6, Palmeriam, Kec. Matraman, Kota Jakarta Timur, Daerah Khusus Ibukota Jakarta 13140',
        'https://maps.app.goo.gl/KFmcXPtdbctpb4nD6',
        TRUE,
        1
    ),
    (
        'ITC Cempaka Mas Blok D Lantai 1 No. 222 E',
        'Jl. Letjen Suprapto No.1, RW.8, Sumur Batu, Kec. Kemayoran, Kota Jakarta Pusat, Daerah Khusus Ibukota Jakarta 10640',
        'https://maps.app.goo.gl/2mHpXTCo3TaUMqcx6',
        TRUE,
        2
    );
