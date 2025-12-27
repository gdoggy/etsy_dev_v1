-- ============================================================
-- 分区表: ai_call_logs (基于 model.AICallLog)
-- 分区策略: 按月范围分区 (created_at)
-- 保留策略: 6个月
-- ============================================================

CREATE TABLE IF NOT EXISTS ai_call_logs (
    -- BaseModel
                                            id BIGSERIAL,
                                            created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                            updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                            deleted_at TIMESTAMPTZ,

    -- 关联
                                            shop_id BIGINT,
                                            task_id BIGINT,

    -- 调用信息
                                            call_type VARCHAR(32),
    model_name VARCHAR(64),

    -- 用量统计
    input_tokens INT DEFAULT 0,
    output_tokens INT DEFAULT 0,
    image_count INT DEFAULT 0,

    -- 性能与成本
    duration_ms BIGINT DEFAULT 0,
    cost_usd DECIMAL(10,6) DEFAULT 0,

    -- 状态
    status VARCHAR(32) DEFAULT 'success',
    error_msg VARCHAR(1024),

    -- 分区表主键
    PRIMARY KEY (id, created_at)
    ) PARTITION BY RANGE (created_at);

-- 索引
CREATE INDEX IF NOT EXISTS idx_ai_call_logs_shop_id ON ai_call_logs (shop_id);
CREATE INDEX IF NOT EXISTS idx_ai_call_logs_task_id ON ai_call_logs (task_id);
CREATE INDEX IF NOT EXISTS idx_ai_call_logs_call_type ON ai_call_logs (call_type);
CREATE INDEX IF NOT EXISTS idx_ai_call_logs_status ON ai_call_logs (status);
CREATE INDEX IF NOT EXISTS idx_ai_call_logs_deleted_at ON ai_call_logs (deleted_at) WHERE deleted_at IS NOT NULL;