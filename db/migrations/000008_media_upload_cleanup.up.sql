ALTER TABLE media_assets
    ADD COLUMN upload_expires_at TIMESTAMPTZ,
    ADD COLUMN cleanup_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN cleanup_last_error TEXT,
    ADD COLUMN cleanup_claimed_at TIMESTAMPTZ;

ALTER TABLE media_assets DROP CONSTRAINT media_assets_status_check;
ALTER TABLE media_assets
    ADD CONSTRAINT media_assets_status_check
    CHECK (status IN ('pending', 'ready', 'failed', 'expired', 'deleted'));

CREATE INDEX idx_media_assets_pending_expiry
    ON media_assets (upload_expires_at)
    WHERE status = 'pending';
CREATE INDEX idx_media_assets_expired_cleanup
    ON media_assets (cleanup_claimed_at)
    WHERE status = 'expired';
