-- Migration: 017_fix_bucket_trigger
-- Fixes the bucket metrics trigger to handle cases where an object is moved between buckets

CREATE OR REPLACE FUNCTION update_bucket_metrics()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'INSERT') THEN
        UPDATE buckets
        SET total_objects = total_objects + 1,
            total_bytes = total_bytes + NEW.size_bytes
        WHERE id = NEW.bucket_id;
        RETURN NEW;
    ELSIF (TG_OP = 'UPDATE') THEN
        IF OLD.bucket_id != NEW.bucket_id THEN
            -- Subtract from old bucket
            UPDATE buckets
            SET total_objects = total_objects - 1,
                total_bytes = total_bytes - OLD.size_bytes
            WHERE id = OLD.bucket_id;
            
            -- Add to new bucket
            UPDATE buckets
            SET total_objects = total_objects + 1,
                total_bytes = total_bytes + NEW.size_bytes
            WHERE id = NEW.bucket_id;
        ELSE
            -- Same bucket, just update bytes if size changed
            IF OLD.size_bytes != NEW.size_bytes THEN
                UPDATE buckets
                SET total_bytes = total_bytes - OLD.size_bytes + NEW.size_bytes
                WHERE id = NEW.bucket_id;
            END IF;
        END IF;
        RETURN NEW;
    ELSIF (TG_OP = 'DELETE') THEN
        UPDATE buckets
        SET total_objects = total_objects - 1,
            total_bytes = total_bytes - OLD.size_bytes
        WHERE id = OLD.bucket_id;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
