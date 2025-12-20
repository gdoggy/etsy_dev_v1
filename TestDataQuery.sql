INSERT INTO proxies (
    created_at, updated_at,
    ip, port, username, password, protocol,
    status, region, capacity, is_active
) VALUES (
             NOW(), NOW(),
             '127.0.0.1', '7897', '', '', 'http',
             1, 'US', 2, true
         );


INSERT INTO developers (
    created_at, updated_at,
    name,
    login_email,
    login_pwd,
    status,
    api_key,
    shared_secret
) VALUES (
             NOW(), NOW(),
             'Etsy Test App',
             'test_dev@example.com',
             '123456',
             1,
             'i0kg9uiwf7fajmq4jamiy3zr',
             'hjga5df5gv'
         );

-- ==========================================
-- 1. 插入 Shops 表 (店铺基础信息)
-- ==========================================
INSERT INTO shops (
    created_at, updated_at, deleted_at,

    -- 核心身份 (注意：这里用的是 snake_case)
    etsy_shop_id,  -- 对应 Model 的 EtsyShopID
    user_id,       -- 对应 Model 的 UserID
    shop_name,
    login_name,

    -- 运营指标
    listing_active_count,
    transaction_sold_count,
    review_count,
    review_average,
    currency_code,

    -- 状态与链接
    is_vacation,
    vacation_message,
    url,
    icon_url,

    -- 关联外键 (必须存在于 proxies 和 developers 表中)
    proxy_id,
    developer_id,

    -- Token 信息 (测试用 Dummy 数据)
    token_status,
    access_token,
    refresh_token,
    token_expires_at
) VALUES (
             NOW(), NOW(), NULL,

             12345678,               -- etsy_shop_id: 模拟的 Etsy 店铺 ID
             88888888,               -- user_id: 模拟的 Etsy 用户 ID
             'My Test Shop',         -- shop_name
             'test_user_login',      -- login_name

             100,                    -- listing_active_count: 在售商品
             50,                     -- transaction_sold_count: 销量
             10,                     -- review_count
             4.8,                    -- review_average
             'USD',                  -- currency_code

             false,                  -- is_vacation
             '',                     -- vacation_message
             'https://www.etsy.com/shop/MyTestShop', -- url
             '',                     -- icon_url

             1,                      -- proxy_id: 引用 ID 为 1 的代理
             1,                      -- developer_id: 引用 ID 为 1 的开发者账号

             'active',               -- token_status
             'test_access_token_xyz', -- access_token (测试时代码里不要真去调 Etsy，除非这是真的)
             'test_refresh_token_xyz',-- refresh_token
             NOW() + INTERVAL '1 hour' -- token_expires_at
         );