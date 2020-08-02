DO $$
BEGIN
    IF ((SELECT COUNT(*) FROM user_stats)) = 0 THEN
        INSERT INTO user_stats SELECT user_id, SUM(size_bytes) FROM media GROUP BY user_id;
    END IF;
END $$;
