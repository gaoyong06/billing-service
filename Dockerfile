# 多阶段构建 Dockerfile for Billing Service
# Stage 1: 构建阶段
FROM golang:1.25-alpine AS builder

# 设置工作目录
WORKDIR /build

# 安装必要的构建工具
RUN apk add --no-cache git make protobuf protobuf-dev

# 复制 go mod 文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 生成 proto 和 wire 代码
RUN make api wire || true

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server cmd/server/main.go cmd/server/wire_gen.go

# Stage 2: 运行阶段
FROM alpine:latest

# 安装 ca-certificates 用于 HTTPS 请求
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非 root 用户
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/server .
COPY --from=builder /build/configs ./configs

# 创建日志目录
RUN mkdir -p logs && chown -R app:app /app

# 切换到非 root 用户
USER app

# 暴露端口
EXPOSE 8107 9107

# 启动服务
CMD ["./server", "-conf", "configs/config.yaml"]





