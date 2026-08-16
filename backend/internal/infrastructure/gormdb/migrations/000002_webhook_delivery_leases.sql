ALTER TABLE webhook_deliveries
    ADD COLUMN locked_until timestamptz;

CREATE INDEX webhook_delivery_lease_index
    ON webhook_deliveries (locked_until, id)
    WHERE state = 'delivering';
