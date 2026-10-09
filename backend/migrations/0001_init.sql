-- +goose Up
CREATE TABLE inv_item (
  item_id             TEXT PRIMARY KEY,
  item_stock          INT NOT NULL CHECK (item_stock >= 0),
  item_stock_reserved INT NOT NULL DEFAULT 0 CHECK (item_stock_reserved >= 0),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (item_stock_reserved <= item_stock)
);

COMMENT ON COLUMN inv_item.item_stock IS
  'Physical units on hand, including units held by pending reservations. Never the availability check on its own: available = item_stock - item_stock_reserved.';
COMMENT ON COLUMN inv_item.item_stock_reserved IS
  'Units held by pending reservations. Always equals the sum of reservation_quantity over this item''s PENDING reservations.';

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
DROP TABLE inv_item;
