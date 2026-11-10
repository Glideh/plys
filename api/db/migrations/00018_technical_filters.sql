-- +goose Up
-- Technical catalog filters, computed from the probed streams of a file (scanner.Stream).

-- Resolution class of the first video stream, with the thresholds of resolutionLabel (internal/api/catalog.go):
-- the width counts too, so that cropped widescreen films (1920x800) are still 1080p.
-- +goose StatementBegin
CREATE FUNCTION media_resolution(streams jsonb) RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
  SELECT CASE
    WHEN v.w = 0 AND v.h = 0 THEN NULL
    WHEN v.w >= 3200 OR v.h >= 1800 THEN '2160p'
    WHEN v.w >= 1700 OR v.h >= 1000 THEN '1080p'
    WHEN v.w >= 1200 OR v.h >= 700 THEN '720p'
    ELSE 'SD'
  END
  FROM (
    SELECT coalesce((s->>'width')::integer, 0) AS w, coalesce((s->>'height')::integer, 0) AS h
    FROM jsonb_array_elements(CASE WHEN jsonb_typeof(streams) = 'array' THEN streams ELSE '[]'::jsonb END) WITH ORDINALITY AS t(s, n)
    WHERE s->>'type' = 'video'
    ORDER BY n
    LIMIT 1
  ) v
$$;
-- +goose StatementEnd

-- Sound formats offered by the audio streams of a file. A stream may yield several formats:
-- TrueHD with Atmos is both truehd and atmos, DTS-HD MA is both dtsHdMa and dts.
-- +goose StatementBegin
CREATE FUNCTION media_audio_formats(streams jsonb) RETURNS text[]
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
  SELECT coalesce(array_agg(DISTINCT format), '{}'::text[])
  FROM jsonb_array_elements(CASE WHEN jsonb_typeof(streams) = 'array' THEN streams ELSE '[]'::jsonb END) AS s
  CROSS JOIN LATERAL unnest(ARRAY[
    CASE WHEN coalesce(s->>'profile', '') ILIKE '%atmos%' THEN 'atmos' END,
    CASE s->>'codec'
      WHEN 'truehd' THEN 'truehd'
      WHEN 'eac3' THEN 'eac3'
      WHEN 'ac3' THEN 'ac3'
      WHEN 'dts' THEN 'dts'
      WHEN 'flac' THEN 'flac'
      WHEN 'aac' THEN 'aac'
      WHEN 'opus' THEN 'opus'
      WHEN 'mp3' THEN 'mp3'
    END,
    CASE WHEN s->>'codec' LIKE 'pcm\_%' THEN 'pcm' END,
    CASE WHEN s->>'codec' = 'dts' AND coalesce(s->>'profile', '') ILIKE 'DTS-HD MA%' THEN 'dtsHdMa' END
  ]) AS format
  WHERE s->>'type' = 'audio' AND format IS NOT NULL
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION media_audio_formats(jsonb);
DROP FUNCTION media_resolution(jsonb);
