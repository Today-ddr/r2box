# R2Box

基于 Cloudflare R2 的轻量级临时文件分享网盘，支持前端直传、大文件分片上传、自动过期清理。

## 特性

- **前端直传** - 文件直接上传到 R2，不占用服务器带宽
- **大文件支持** - 支持最大 5GB 文件，自动分片上传
- **自动过期** - 支持 1天/3天/7天/30天 自动删除
- **R2 直链** - 上传完成后直接返回 R2 预签名下载链接
- **Token 鉴权** - 基于口令的访问控制
- **速率限制** - 防暴力破解，IP 限流保护
- **存储监控** - 实时查看存储空间使用情况
- **轻量部署** - 内存占用仅 ~55MB，适合低配服务器
- **引导配置** - 首次登录后通过 Web 界面配置 R2

## 界面展示

### 首页概览

![首页](img/homepage.png)

*登录后的主界面，简洁直观的操作入口*

### 文件上传

![上传界面](img/upload_interface.png)

*支持拖拽上传，可选择过期时间，上传完成后显示文件名和 R2 直链*

### 文件管理

![文件列表](img/file_list.png)

*查看已上传文件，支持下载和删除操作，显示剩余有效时间*

### 存储统计

![存储统计](img/storage_usage.png)

*实时监控存储空间使用情况，查看文件数量和过期统计*

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | Go 1.21+ (net/http 标准库) |
| 前端 | Vue.js 3 + Vite + Naive UI |
| 数据库 | SQLite |
| 存储 | Cloudflare R2 |
| 部署 | Docker + Docker Compose |

## 快速开始

### 1. 准备 Cloudflare R2

1. 登录 [Cloudflare Dashboard](https://dash.cloudflare.com)
2. 进入 **R2 Object Storage**
3. 创建存储桶（例如：`r2box-files`）
4. 创建 API Token（权限：Object Read & Write）
5. 记录以下信息：
   - R2 端点 URL（格式：`https://<account_id>.r2.cloudflarestorage.com`）
   - Access Key ID
   - Secret Access Key
   - Bucket Name

### 2. 配置 R2 CORS（推荐）

```bash
# 安装 Wrangler CLI
npm install -g wrangler

# 配置 CORS
wrangler r2 bucket cors set r2box-files --file r2-cors.json
```

### 3. 部署应用

```bash
# 克隆项目
git clone <repository-url>
cd r2box

# 修改 docker-compose.yml 中的 ACCESS_TOKEN
# 启动服务
docker-compose up -d
```

### 4. 首次配置

1. 访问 `http://localhost:9988`
2. 输入 `ACCESS_TOKEN` 登录
3. 在 R2 配置向导中填写 R2 信息
4. 测试连接并保存配置
5. 开始使用！

## 配置说明

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `ACCESS_TOKEN` | 访问口令（必需） | - |
| `PORT` | 服务器端口 | 9988 |
| `MAX_FILE_SIZE` | 最大文件大小（字节） | 5368709120 (5GB) |
| `TOTAL_STORAGE` | 总存储空间（字节） | 10737418240 (10GB) |
| `DATABASE_PATH` | 数据库路径 | ./data/r2box.db |

### docker-compose.yml 示例

```yaml
version: '3.8'

services:
  r2box:
    build: .
    container_name: r2box
    restart: unless-stopped
    ports:
      - "9988:9988"
    volumes:
      - ./data:/app/data
    environment:
      - ACCESS_TOKEN=your_secure_password_here
      - PORT=9988
      - MAX_FILE_SIZE=5368709120
      - TOTAL_STORAGE=10737418240
```

## 项目结构

```
r2box/
├── backend/              # Go 后端
│   ├── config/           # 配置管理
│   ├── database/         # 数据库
│   ├── handlers/         # API 处理器
│   ├── middleware/       # 中间件
│   ├── models/           # 数据模型
│   ├── services/         # R2 服务
│   └── main.go           # 入口文件
├── frontend/             # Vue.js 前端
│   ├── src/
│   │   ├── views/        # 页面组件
│   │   ├── stores/       # 状态管理
│   │   ├── services/     # API 服务
│   │   └── router/       # 路由配置
│   └── package.json
├── img/                  # 截图资源
├── Dockerfile            # Docker 构建
├── docker-compose.yml    # Docker Compose 配置
├── r2-cors.json          # R2 CORS 配置
└── r2-lifecycle.json     # R2 生命周期规则
```

## 本地开发

### 后端

```bash
cd backend
go mod download
go run main.go
```

### 前端

```bash
cd frontend
npm install
npm run dev
```

## 安全机制

- **Token 认证**：所有管理操作需要认证
- **速率限制**：每分钟最多 60 次请求
- **IP 锁定**：连续 5 次登录失败锁定 15 分钟
- **文件大小限制**：单文件最大 5GB

## 常见问题

**Q: 上传失败？**
- 检查 R2 配置是否正确
- 确认 CORS 已配置
- 查看浏览器控制台错误

**Q: 文件未自动删除？**
- 应用内置清理任务每小时执行一次
- R2 Lifecycle 规则每天执行，可能有延迟

**Q: 无法访问？**
- 检查防火墙是否开放端口
- 确认 Docker 容器正在运行

## 许可证

MIT License

## 致谢

- [Cloudflare R2](https://www.cloudflare.com/products/r2/)
- [Vue.js](https://vuejs.org/)
- [Naive UI](https://www.naiveui.com/)
- [Go](https://golang.org/)
