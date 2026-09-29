ALTER TABLE store_settings
    DROP COLUMN IF EXISTS about_body,
    DROP COLUMN IF EXISTS about_summary,
    DROP COLUMN IF EXISTS about_title;
