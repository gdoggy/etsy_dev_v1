-- ============================================================
-- 分区表: tracking_events (基于 model.TrackingEvent)
-- 分区策略: 按月范围分区 (created_at)
-- 保留策略: 12个月
-- ============================================================

CREATE TABLE IF NOT EXISTS tracking_events (
    -- BaseModel
                                               id BIGSERIAL,
                                               created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                               updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                               deleted_at TIMESTAMPTZ,

    -- 关联
                                               shipment_id BIGINT NOT NULL,
                                               karrio_event_id VARCHAR(64),

    -- 事件信息
    occurred_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(32),
    status_code VARCHAR(32),
    description VARCHAR(500),
    location VARCHAR(255),

    -- 原始数据
    raw_payload JSONB,

    -- 分区表主键
    PRIMARY KEY (id, created_at)
    ) PARTITION BY RANGE (created_at);

-- 索引
CREATE INDEX IF NOT EXISTS idx_tracking_events_shipment_id ON tracking_events (shipment_id);
CREATE INDEX IF NOT EXISTS idx_tracking_events_occurred_at ON tracking_events (occurred_at);
CREATE INDEX IF NOT EXISTS idx_tracking_events_status ON tracking_events (status);
CREATE INDEX IF NOT EXISTS idx_tracking_events_deleted_at ON tracking_events (deleted_at) WHERE deleted_at IS NOT NULL;