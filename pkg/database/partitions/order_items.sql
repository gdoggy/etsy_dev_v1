-- ============================================================
-- 分区表: order_items (基于 model.OrderItem)
-- 分区策略: 按月范围分区 (created_at)
-- ============================================================

CREATE TABLE IF NOT EXISTS order_items (
    -- BaseModel
                                           id BIGSERIAL,
                                           created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                           updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                           deleted_at TIMESTAMPTZ,

    -- 关联
                                           order_id BIGINT NOT NULL,
                                           etsy_transaction_id BIGINT NOT NULL,

    -- 商品信息
                                           listing_id BIGINT,
                                           product_id BIGINT,
                                           title VARCHAR(500),
    sku VARCHAR(100),

    -- 数量与价格
    quantity INT DEFAULT 1,
    price_amount BIGINT DEFAULT 0,
    shipping_cost BIGINT DEFAULT 0,
    currency VARCHAR(10),

    -- 变体信息
    variations JSONB,

    -- 图片
    listing_image_id BIGINT,
    image_url VARCHAR(500),

    -- 数字商品
    is_digital BOOLEAN DEFAULT FALSE,

    -- 时间戳
    paid_at TIMESTAMPTZ,
    shipped_at TIMESTAMPTZ,

    -- 分区表主键
    PRIMARY KEY (id, created_at)
    ) PARTITION BY RANGE (created_at);

-- 索引
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items (order_id);
CREATE INDEX IF NOT EXISTS idx_order_items_etsy_tx_id ON order_items (etsy_transaction_id);
CREATE INDEX IF NOT EXISTS idx_order_items_listing_id ON order_items (listing_id);
CREATE INDEX IF NOT EXISTS idx_order_items_sku ON order_items (sku);
CREATE INDEX IF NOT EXISTS idx_order_items_deleted_at ON order_items (deleted_at) WHERE deleted_at IS NOT NULL;