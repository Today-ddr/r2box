# Stage 1: 构建前端
FROM node:20-alpine AS frontend-builder

ARG APP_VERSION=0.0.0
ARG COMMIT_SHA=dev

WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./

ENV APP_VERSION=${APP_VERSION}
ENV COMMIT_SHA=${COMMIT_SHA}

RUN npm run build

# Stage 2: 构建后端
FROM golang:1.21-alpine AS backend-builder

ARG APP_VERSION=0.0.0
ARG COMMIT_SHA=dev

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

# 依赖锁定，构建结果可复现
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags "-s -w -X main.Version=${APP_VERSION} -X main.CommitSHA=${COMMIT_SHA}" \
    -a -installsuffix cgo -o main .

# Stage 3: 运行时镜像
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=backend-builder /app/main .
COPY --from=frontend-builder /app/frontend/dist ./static

# 以非 root 运行。宿主机的 ./data 需要 chown 1000:1000
RUN addgroup -g 1000 -S app \
    && adduser -u 1000 -S -G app app \
    && mkdir -p /app/data \
    && chown -R app:app /app

USER app

EXPOSE 9988
ENV PORT=9988

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:9988/healthz || exit 1

CMD ["./main"]
