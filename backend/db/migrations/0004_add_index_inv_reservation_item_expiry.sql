-- +goose Up
-- Serves the release a reserve runs for one item when it finds too little
-- stock: that item's pending reservations, oldest deadline first. The index
-- on the deadline alone (0002) still serves the sweeper, which works across
-- all items.
CREATE INDEX inv_reservation_item_pending_expiry_idx
  ON inv_reservation (item_id, reservation_expires_date) WHERE reservation_status = 'PENDING';

-- +goose Down
DROP INDEX inv_reservation_item_pending_expiry_idx;
