-- ============================================================
-- 分区表: orders (基于 model.Order)
-- 分区策略: 按月范围分区 (created_at)
-- ============================================================

CREATE TABLE IF NOT EXISTS orders (
    -- BaseModel
                                      id BIGSERIAL,
                                      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                      updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                      deleted_at TIMESTAMPTZ,

    -- 核心字段
                                      etsy_receipt_id BIGINT NOT NULL,
                                      shop_id BIGINT NOT NULL,

    -- 买家信息
                                      buyer_user_id BIGINT,
                                      buyer_email VARCHAR(255),
    buyer_name VARCHAR(255),

    -- 状态
    status VARCHAR(32) DEFAULT 'pending',
    etsy_status VARCHAR(32),

    -- 消息
    message_from_buyer TEXT,
    message_from_seller TEXT,

    -- 礼物
    is_gift BOOLEAN DEFAULT FALSE,
    gift_message TEXT,

    -- 收货地址
    shipping_address JSONB,

    -- 金额（分）
    subtotal_amount BIGINT DEFAULT 0,
    shipping_amount BIGINT DEFAULT 0,
    tax_amount BIGINT DEFAULT 0,
    discount_amount BIGINT DEFAULT 0,
    grand_total_amount BIGINT DEFAULT 0,
    currency VARCHAR(10) DEFAULT 'USD',

    -- 支付
    payment_method VARCHAR(64),
    is_paid BOOLEAN DEFAULT FALSE,
    paid_at TIMESTAMPTZ,

    -- 发货
    is_shipped BOOLEAN DEFAULT FALSE,
    shipped_at TIMESTAMPTZ,

    -- Etsy 原始数据
    etsy_raw_data JSONB,

    -- 同步时间
    etsy_created_at TIMESTAMPTZ,
    etsy_updated_at TIMESTAMPTZ,
    etsy_synced_at TIMESTAMPTZ,

    -- 分区表主键
    PRIMARY KEY (id, created_at)
    ) PARTITION BY RANGE (created_at);

-- 索引
CREATE INDEX IF NOT EXISTS idx_orders_shop_id ON orders (shop_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);
CREATE INDEX IF NOT EXISTS idx_orders_etsy_receipt_id ON orders (etsy_receipt_id);
CREATE INDEX IF NOT EXISTS idx_orders_shop_status ON orders (shop_id, status);
CREATE INDEX IF NOT EXISTS idx_orders_deleted_at ON orders (deleted_at) WHERE deleted_at IS NOT NULL;