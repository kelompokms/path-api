-- +goose Up
CREATE TABLE file (
    id SERIAL PRIMARY KEY,
    nama TEXT NOT NULL,
    path TEXT NOT NULL,
    ukuran INTEGER NOT NULL,
    tanggal DATE NOT NULL
);

-- +goose Down
DROP TABLE file;
