# ---
# 第一阶段：builder
# ---
FROM --platform=linux/amd64 golang:1.24-alpine AS builder

# 1. 设置国内代理
ENV GOPROXY=https://goproxy.cn,direct

# 2. 环境变量
# CGO_ENABLED=0 确保生成静态链接二进制
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

# 3. 缓存依赖
COPY go.mod go.sum ./
RUN go mod download

# 4. 复制编译
COPY . .
# -ldflags="-w -s" 去掉调试信息，减小体积
RUN go build -ldflags="-w -s" -o etsy_dev ./cmd/main.go

# ---
# 第二阶段：runner
# ---
# 关键修改：在这里建议显式指定平台，防止在 Mac 上拉取了 ARM 版 Alpine 导致不兼容
# 或者在 docker build 命令中指定（见下文）
FROM --platform=linux/amd64 alpine:latest

LABEL authors="vip"

# 5. 安装基础依赖
# ca-certificates: HTTPS 请求必须
# tzdata: 设置时区必须
RUN apk --no-cache add ca-certificates tzdata

# 设置时区为上海
ENV TZ=Asia/Hong_Kong

WORKDIR /root/

# 6. 复制二进制文件
COPY --from=builder /app/etsy_dev .

# 7. [重要] 如果有配置文件需要复制
# COPY --from=builder /app/.env .
# COPY --from=builder /app/config.yaml .

EXPOSE 8080

CMD ["./etsy_dev"]