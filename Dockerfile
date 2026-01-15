# 多阶段构建 Dockerfile for Billing Service
# ⚠️ 注意：此 Dockerfile 需要从项目根目录构建（便于处理本地依赖）
# 构建命令：docker build -f billing-service/Dockerfile -t image:tag .
# Stage 1: 构建阶段
FROM golang:1.25-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git make protobuf protobuf-dev

# 设置工作目录
WORKDIR /workspace

# 复制本地依赖（根据 go.mod 中的 replace 指令）
COPY go-pkg ./go-pkg
COPY payment-service ./payment-service

# 设置服务工作目录
WORKDIR /workspace/billing-service

# 复制服务的 go mod 文件
COPY billing-service/go.mod billing-service/go.sum ./

# 配置 replace 指令指向本地依赖
RUN go mod edit -replace github.com/gaoyong06/go-pkg=/workspace/go-pkg || true && \
    go mod edit -replace payment-service=/workspace/payment-service || true

# 下载依赖（包括本地依赖）
RUN go mod download

# 复制服务源代码
COPY billing-service/ .

# 更新 go.mod（确保依赖关系正确）
RUN go mod tidy

# 重新设置 replace 指令（go mod tidy 可能会移除 replace）
RUN go mod edit -replace github.com/gaoyong06/go-pkg=/workspace/go-pkg || true

# 生成 proto 和 wire 代码（如果需要）
RUN make api wire || true

# 构建二进制文件（包含 wire_gen.go 如果存在）
RUN if [ -f cmd/server/wire_gen.go ]; then \
      CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server cmd/server/main.go cmd/server/wire_gen.go; \
    elif [ -f cmd/scheduler/wire_gen.go ]; then \
      CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server cmd/scheduler/main.go cmd/scheduler/wire_gen.go; \
    else \
      CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server cmd/server/main.go; \
    fi

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
COPY --from=builder /workspace/billing-service/server .
COPY --from=builder /workspace/billing-service/configs ./configs

# 创建日志目录
RUN mkdir -p logs && chown -R app:app /app

# 切换到非 root 用户
USER app

# 暴露端口
EXPOSE 8107 9107

# 启动服务
CMD ["./server", "-conf", "configs/config.yaml"]
