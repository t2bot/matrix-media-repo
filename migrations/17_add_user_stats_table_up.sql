CREATE TABLE IF NOT EXISTS user_stats (
	user_id TEXT PRIMARY KEY NOT NULL,
	uploaded_bytes BIGINT NOT NULL
);
CREATE OR REPLACE FUNCTION track_update_user_media()
    RETURNS TRIGGER
    LANGUAGE PLPGSQL
    AS
$$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        INSERT INTO user_stats (user_id, uploaded_bytes) VALUES (NEW.user_id, 0) ON CONFLICT (user_id) DO NOTHING;
        INSERT INTO user_stats (user_id, uploaded_bytes) VALUES (OLD.user_id, 0) ON CONFLICT (user_id) DO NOTHING;

        IF NEW.user_id <> OLD.user_id THEN
            UPDATE user_stats SET uploaded_bytes = user_stats.uploaded_bytes - OLD.size_bytes WHERE user_stats.user_id = OLD.user_id;
            UPDATE user_stats SET uploaded_bytes = user_stats.uploaded_bytes + NEW.size_bytes WHERE user_stats.user_id = NEW.user_id;
        ELSIF NEW.size_bytes <> OLD.size_bytes THEN
            UPDATE user_stats SET uploaded_bytes = user_stats.uploaded_bytes - OLD.size_bytes + NEW.size_bytes WHERE user_stats.user_id = NEW.user_id;
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE user_stats SET uploaded_bytes = user_stats.uploaded_bytes - OLD.size_bytes WHERE user_stats.user_id = OLD.user_id;
        RETURN OLD;
    ELSIF TG_OP = 'INSERT' THEN
        INSERT INTO user_stats (user_id, uploaded_bytes) VALUES (NEW.user_id, NEW.size_bytes) ON CONFLICT (user_id) DO UPDATE SET uploaded_bytes = user_stats.uploaded_bytes + NEW.size_bytes;
        RETURN NEW;
    END IF;
END;
$$;
DROP TRIGGER IF EXISTS media_change_for_user ON media;
CREATE TRIGGER media_change_for_user AFTER INSERT OR UPDATE OR DELETE ON media FOR EACH ROW EXECUTE PROCEDURE track_update_user_media();
