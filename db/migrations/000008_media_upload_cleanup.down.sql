DROP INDEX IF EXISTS idx_media_assets_pending_expiry;
DROP INDEX IF EXISTS idx_media_assets_expired_cleanup;

ALTER TABLE media_assets DROP CONSTRAINT media_assets_status_check;
ALTER TABLE media_assets
    ADD CONSTRAINT media_assets_status_check
    CHECK (status IN ('pending', 'ready', 'failed', 'deleted'));

ALTER TABLE media_assets
    DROP COLUMN cleanup_last_error,
    DROP COLUMN cleanup_attempts,
    DROP COLUMN cleanup_claimed_at,
    DROP COLUMN upload_expires_at;
