-- +goose Up
INSERT INTO inv_item (item_id, item_stock)
VALUES ('item_4021', 100)
ON CONFLICT (item_id) DO NOTHING;

-- +goose Down
DELETE FROM inv_item WHERE item_id = 'item_4021';
