# ---
# 开发环境 Mac (ARM64)，生产环境 Linux (AMD64/x86)，采用多阶段构建 (Multi-stage Build)
# ---

# ---
# 第一阶段：builder
# ---
FROM golang:1.24-alpine AS builder
# 环境变量
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64
# DOCKER工作目录
WORKDIR /app
# 依赖
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-w -s" -o etsy_dev .


# ---
# 第二阶段：runner
# ---
FROM alpine:latest
LABEL authors="vip"

# 安装证书
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/etsy_dev .
EXPOSE 8080
CMD ["./etsy_dev"]
