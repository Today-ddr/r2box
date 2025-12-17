<p align="center">
  <img src="img/logo.png" alt="R2Box Logo" width="120" />
</p>

<h1 align="center">R2Box</h1>

<p align="center">基于 Cloudflare R2 的轻量级临时文件分享网盘，支持前端直传、大文件分片上传、自动过期清理。</p>

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

### 文件上传

![上传界面](img/upload_interface.png)

### 文件管理

![文件列表](img/file_list.png)

### 存储统计

![存储统计](img/storage_usage.png)

## 快速开始

### 1. 准备 Cloudflare R2

1. 登录 [Cloudflare Dashboard](https://dash.cloudflare.com)
2. 进入 **R2 Object Storage** → 创建存储桶
3. 点击 **Manage R2 API Tokens** → 创建 API Token（权限：Object Read & Write）
4. 记录以下信息：
   - R2 端点 URL：`https://<account_id>.r2.cloudflarestorage.com`
   - Access Key ID
   - Secret Access Key
   - Bucket Name

### 2. 部署应用

**方式一：使用预构建镜像（推荐）**

```bash
# 创建数据目录
mkdir -p r2box/data && cd r2box

# 下载 docker-compose.yml
curl -O https://raw.githubusercontent.com/Today-ddr/r2box/master/docker-compose.yml

# 修改 ACCESS_TOKEN 后启动
docker compose up -d
```

或者使用 Docker 命令直接运行：

```bash
docker run -d \
  --name r2box \
  --restart unless-stopped \
  -p 9988:9988 \
  -v ./data:/app/data \
  -e ACCESS_TOKEN=your_secure_password \
  ghcr.io/today-ddr/r2box:latest
```

**方式二：从源码构建**

```bash
git clone https://github.com/Today-ddr/r2box.git && cd r2box && docker compose up -d --build
```

**方式三：手动下载**

1. 下载项目：[点击下载 ZIP](https://github.com/Today-ddr/r2box/archive/refs/heads/main.zip)
2. 解压后进入目录，运行：

```bash
docker compose up -d --build
```

### 3. 首次配置

1. 访问 `http://your-server-ip:9988`
2. 输入 `ACCESS_TOKEN` 登录（默认在 docker-compose.yml 中配置）
3. 在 R2 配置向导中填写步骤 1 记录的 R2 信息
4. 测试连接 → 保存配置 → 开始使用！

## 配置说明

### docker-compose.yml

```yaml
environment:
  - ACCESS_TOKEN=your_secure_password_here  # 必须修改！访问口令
  - PORT=9988                                # 服务端口
  - MAX_FILE_SIZE=5368709120                 # 最大文件 5GB
  - TOTAL_STORAGE=10737418240                # 总存储 10GB
```

## 常用命令

```bash
# 启动
docker compose up -d

# 查看日志
docker compose logs -f

# 停止
docker compose down

# 更新到最新版本
docker compose pull && docker compose up -d

# 从源码重新构建（需先修改 docker-compose.yml 启用 build）
docker compose up -d --build
```

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | Go 1.21+ |
| 前端 | Vue.js 3 + Naive UI |
| 数据库 | SQLite |
| 存储 | Cloudflare R2 |
| 部署 | Docker |

## 项目结构

```
r2box/
├── backend/              # Go 后端
├── frontend/             # Vue.js 前端
├── img/                  # 截图
├── Dockerfile
├── docker-compose.yml
└── r2-cors.json          # R2 CORS 配置
```

## 常见问题

**Q: 上传失败？**
- 检查 R2 CORS 是否配置
- 查看浏览器控制台错误

**Q: 无法访问？**
- 检查防火墙是否开放 9988 端口
- `docker compose logs` 查看错误日志

## 许可证

MIT License
