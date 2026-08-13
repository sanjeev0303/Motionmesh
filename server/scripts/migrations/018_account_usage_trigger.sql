-- Migration: 018_account_usage_trigger
-- Adds denormalized usage counters to the accounts table and a trigger to maintain them

ALTER TABLE accounts 
    ADD COLUMN IF NOT EXISTS total_storage_bytes BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_videos INTEGER NOT NULL DEFAULT 0;

CREATE OR REPLACE FUNCTION update_account_usage()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'INSERT') THEN
        UPDATE accounts
        SET total_videos = total_videos + 1,
            total_storage_bytes = total_storage_bytes + NEW.size_bytes
        WHERE id = NEW.account_id;
        RETURN NEW;
    ELSIF (TG_OP = 'UPDATE') THEN
        IF OLD.account_id != NEW.account_id THEN
            -- Subtract from old account
            UPDATE accounts
            SET total_videos = total_videos - 1,
                total_storage_bytes = total_storage_bytes - OLD.size_bytes
            WHERE id = OLD.account_id;
            
            -- Add to new account
            UPDATE accounts
            SET total_videos = total_videos + 1,
                total_storage_bytes = total_storage_bytes + NEW.size_bytes
            WHERE id = NEW.account_id;
        ELSE
            -- Same account, update bytes if size changed
            IF OLD.size_bytes != NEW.size_bytes THEN
                UPDATE accounts
                SET total_storage_bytes = total_storage_bytes - OLD.size_bytes + NEW.size_bytes
                WHERE id = NEW.account_id;
            END IF;
        END IF;
        RETURN NEW;
    ELSIF (TG_OP = 'DELETE') THEN
        UPDATE accounts
        SET total_videos = total_videos - 1,
            total_storage_bytes = total_storage_bytes - OLD.size_bytes
        WHERE id = OLD.account_id;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_account_usage ON videos;
CREATE TRIGGER trg_update_account_usage
AFTER INSERT OR UPDATE OR DELETE ON videos
FOR EACH ROW
EXECUTE FUNCTION update_account_usage();
