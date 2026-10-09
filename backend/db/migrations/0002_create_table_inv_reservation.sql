-- +goose Up
CREATE TABLE inv_reservation (
  reservation_id             TEXT PRIMARY KEY,
  user_id                    TEXT NOT NULL,
  item_id                    TEXT NOT NULL REFERENCES inv_item(item_id),
  reservation_quantity       INT  NOT NULL CHECK (reservation_quantity > 0),
  reservation_status         TEXT NOT NULL CHECK (reservation_status IN ('PENDING','CONFIRMED','EXPIRED')),
  reservation_date           TIMESTAMPTZ NOT NULL DEFAULT now(),
  reservation_expires_date   TIMESTAMPTZ NOT NULL,
  reservation_confirmed_date TIMESTAMPTZ,
  reservation_expired_date   TIMESTAMPTZ
);

-- Finds reservations that are past their deadline and still holding stock.
CREATE INDEX inv_reservation_pending_expiry_idx
  ON inv_reservation (reservation_expires_date) WHERE reservation_status = 'PENDING';

-- +goose Down
DROP TABLE inv_reservation;
