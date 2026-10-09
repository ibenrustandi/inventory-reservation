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

-- +goose Down
DROP TABLE inv_item;
