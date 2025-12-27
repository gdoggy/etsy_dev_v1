-- ============================================================
-- 分区表: shipments (基于 model.Shipment)
-- 分区策略: 按月范围分区 (created_at)
-- ============================================================

CREATE TABLE IF NOT EXISTS shipments (
    -- BaseModel
                                         id BIGSERIAL,
                                         created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                         updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                         deleted_at TIMESTAMPTZ,

    -- 关联
                                         order_id BIGINT NOT NULL,

    -- Karrio 关联
                                         karrio_shipment_id VARCHAR(64),
    karrio_tracker_id VARCHAR(64),

    -- 物流商信息（国际段）
    carrier_code VARCHAR(32) NOT NULL,
    carrier_name VARCHAR(64),
    tracking_number VARCHAR(64),
    service_code VARCHAR(32),

    -- 目的地物流（末端配送）
    dest_carrier_code VARCHAR(32),
    dest_carrier_name VARCHAR(64),
    dest_tracking_number VARCHAR(64),

    -- 面单
    label_url VARCHAR(500),
    label_type VARCHAR(10),

    -- 包裹信息
    weight DOUBLE PRECISION,
    weight_unit VARCHAR(10) DEFAULT 'KG',

    -- 状态
    status VARCHAR(32) DEFAULT 'created',

    -- Etsy 同步
    etsy_synced BOOLEAN DEFAULT FALSE,
    etsy_synced_at TIMESTAMPTZ,
    etsy_sync_error TEXT,

    -- 最后跟踪信息
    last_tracking_status VARCHAR(64),
    last_tracking_time TIMESTAMPTZ,
    last_tracking_location VARCHAR(255),

    -- 时间
    shipped_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,

    -- 分区表主键
    PRIMARY KEY (id, created_at)
    ) PARTITION BY RANGE (created_at);

-- 索引
CREATE INDEX IF NOT EXISTS idx_shipments_order_id ON shipments (order_id);
CREATE INDEX IF NOT EXISTS idx_shipments_tracking_number ON shipments (tracking_number);
CREATE INDEX IF NOT EXISTS idx_shipments_karrio_tracker_id ON shipments (karrio_tracker_id);
CREATE INDEX IF NOT EXISTS idx_shipments_status ON shipments (status);
CREATE INDEX IF NOT EXISTS idx_shipments_etsy_synced ON shipments (etsy_synced) WHERE etsy_synced = FALSE;
CREATE INDEX IF NOT EXISTS idx_shipments_deleted_at ON shipments (deleted_at) WHERE deleted_at IS NOT NULL;