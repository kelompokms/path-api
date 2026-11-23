-- +goose Up
ALTER TABLE postingan
	ADD COLUMN IF NOT EXISTS tenggat_waktu TIMESTAMP;

-- +goose Down
ALTER TABLE postingan
	DROP COLUMN IF EXISTS tenggat_waktu;
