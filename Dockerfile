# Stage 1: 构建前端
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

# 先复制依赖文件（利用 Docker 缓存）
COPY frontend/package*.json ./

# 安装依赖（依赖不变时使用缓存）
RUN npm install

# 再复制源码
COPY frontend/ ./

# 构建
RUN npm run build

# Stage 2: 构建后端
FROM golang:1.21-alpine AS backend-builder

WORKDIR /app

# 安装构建依赖（SQLite 需要 CGO）
RUN apk add --no-cache gcc musl-dev

# 先复制依赖文件
COPY backend/go.mod ./

# 复制源码
COPY backend/ ./

# 下载依赖并生成 go.sum
RUN go mod tidy

# 构建后端（启用 CGO 以支持 SQLite）
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main .

# Stage 3: 运行时镜像
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从构建阶段复制文件
COPY --from=backend-builder /app/main .
COPY --from=frontend-builder /app/frontend/dist ./static

# 创建数据目录
RUN mkdir -p /app/data

# 暴露端口
EXPOSE 9988

# 设置环境变量
ENV PORT=9988

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:9988/ || exit 1

# 启动应用
CMD ["./main"]
