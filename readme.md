# 1. 初始化 Web 框架 (Gin)
go get -u github.com/gin-gonic/gin

# 2. 初始化数据库相关 (GORM + Postgres)
go get -u gorm.io/gorm
go get -u gorm.io/driver/postgres

# 3. HTTP 客户端 (Resty v2)
# 用于请求 Etsy API，支持方便的 Proxy 设置
go get -u github.com/go-resty/resty/v2

# 4. 配置文件管理 (Viper)
# 用于读取 config.yaml，管理 API Key、Proxy 列表等
go get -u github.com/spf13/viper

# 5. API 文档自动生成 (Swag)
go get -u github.com/swaggo/swag/cmd/swag
go get -u github.com/swaggo/gin-swagger
go get -u github.com/swaggo/files

# 6. 日志库 (Zap)
go get -u go.uber.org/zap