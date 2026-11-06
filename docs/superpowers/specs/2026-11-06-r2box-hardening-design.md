# R2Box 可靠性加固设计

日期：2026-11-06
状态：待实施

## 背景与目标

R2Box 是一个自用的 Cloudflare R2 文件中转站：浏览器拿预签名 URL 直传，文件按天数过期后自动清理，提供短链和下载直链。当前版本可以跑通主流程，但在配额控制、过期语义、并发安全和部署可复现性上存在明确缺陷。

本次要解决的问题按优先级：

1. **配额是装饰品**：`TOTAL_STORAGE` 只用于展示，从未参与准入；预签名 PUT 不限制体积，绕开前端可以直接上传任意大小。
2. **30 天文件的下载直链是坏的**：用「文件剩余寿命」当签名有效期，超过 SigV4 的 7 天上限（`X-Amz-Expires` ≤ 604800 秒），链接在 R2 侧返回 400，而后端感知不到。
3. **SQLite 并发不安全**：没开 WAL、没设 busy_timeout，限流中间件每个请求多次读写，配合上传和清理任务会出现 `database is locked`。
4. **孤儿资源无人回收**：分片上传的 `uploadID` 没有落库，前端一刷新就无法 complete/abort；`pending` / `uploading` 记录永不清理，R2 侧的未完成分片持续计费。
5. **时间处理有两处错误**：`time.Now().Truncate(24h)` 按 UTC 零点截断，东八区差 8 小时；过期判断依赖 SQLite 文本时间的字典序比较。
6. **工程卫生**：缺 `go.sum` 导致构建现场解析依赖、镜像以 root 运行、没有优雅关闭、限流信任伪造的 `X-Forwarded-For`、存在从未被读取的死配置。

### 为什么现在做

服务刚在一台新服务器上完成部署，数据量接近零，做 schema 迁移的代价最低。

## 目标

- 上传体积在「准入」和「落库」两处被真实约束，配额能拦住超额上传。
- 下载行为在任何文件寿命下都正确，链接长期可用。
- 并发写不再报锁；孤儿记录和孤儿分片能被自动回收。
- 部署可复现、镜像非 root、进程能优雅退出。

## 非目标

- **不修认证体系**。空密码可被设置、无盐 SHA-256、token 即密码哈希、初始化竞态，这些是已知的 P0，本次明确不做。
- 不引入 HTTPS / 反向代理配置。部署形态仍是「单容器 + 端口直出」。
- 不做多用户、权限模型、审计日志。
- 不做每小时全量 `HeadObject` 对账。R2 没有对象事件通知，全量 HEAD 的成本随文件数线性增长，而上传确认时的校验已经覆盖了唯一的写入路径。

## 约束

- 运行时是 `alpine:3.19`，单容器，`mem_limit: 450m`，`cpus: 2`。
- 存储是 Cloudflare R2，S3 兼容，但**不支持** presigned POST 的 `content-length-range`。
- 数据库是单文件 SQLite（`mattn/go-sqlite3`，CGO）。
- 已有部署可能需要保留旧数据，迁移必须幂等且能在旧库上直接跑。

## 方案选择

### 一、上传体积与配额：事后对账（已选定）

R2 无法在签名 URL 上约束体积，所以只有三条路：

| 方案 | 结论 |
|---|---|
| 事后对账 | **选定**。签名前做准入，确认上传时用 `HeadObject` 校验真实大小，超限则删对象。 |
| 服务端中转 | 否决。5GB 文件、450MB 内存、双倍带宽，失去直传的全部意义。 |
| 换存储 | 否决。超出本次范围。 |

**取舍**：恶意超大文件在上传完成到被删除之间存在一个窗口，R2 的 PUT 请求费和这段窗口的存储费仍会产生。自用场景可接受，且 R2 出网流量不收费。

### 二、下载链接：统一走后端 302（已选定）

| 方案 | 结论 |
|---|---|
| 全部走 302 | **选定**。对外只暴露 `/api/files/{id}/download`，访问时临时签 5 分钟再跳转。 |
| 保留 R2 直链，有效期封顶 6 天 | 否决。复制给别人的链接会在 6 天后静默失效，且与「点下载」的行为不一致。 |

**取舍**：每次下载多一次 302 往返（R2 不收费的请求），换取链接语义稳定、过期校验集中在一处、可统计下载次数。

## 实施设计

### 1. 配额准入与上传后校验

**新增依赖注入**：`UploadHandler` 增加 `totalStorage int64` 字段，`NewUploadHandler` 增加对应参数。

**新增查询**（`models/file.go`）：

```go
// 已占用额度 = 在途 + 已完成
func GetCommittedStorage(db *sql.DB) (int64, error)
// SELECT COALESCE(SUM(size),0) FROM files WHERE upload_status IN ('pending','uploading','completed')
```

**准入规则**，`GeneratePresignURL` 与 `InitiateMultipartUpload` 共用：

- `size <= 0` → `400` 「文件大小无效」
- `size > maxFileSize` → `413` 「文件大小超过限制」
- `used + size > totalStorage` → `507` 「存储空间不足，请清理文件后重试」

**确认后校验**，`ConfirmUpload` 与 `CompleteMultipartUpload` 共用一段逻辑：

1. `HeadObject(key)` 取 `ContentLength`、`ContentType`。
2. 对象不存在（`NoSuchKey` / 404）→ 标记 `failed`，返回 `400`「对象未上传成功」。
3. **其他错误**（超时、5xx、权限）→ 返回 `502`「存储服务暂时不可用」，**不改变状态**，允许客户端重试。
4. `actualSize > maxFileSize`，或 `used - 声明值 + actualSize > totalStorage` → `DeleteObject`，标记 `failed`，返回 `413` / `507`。
5. 否则用真实 `ContentLength` 覆盖库里的 `size`，状态置 `completed`。

`failed` 不是确认流程的终点：客户端重传后可以再次 `confirm`，确认逻辑不因状态是否为 `failed` 而拒绝。

`R2Service` 新增 `HeadObject(key) (size int64, contentType string, err error)`。

**关于并发超卖**：准入是「查 `GetCommittedStorage` → 判断 → `Create`」，不在事务里。并发 presign 可能同时通过，造成有界超卖。接受这个行为——单用户场景下 presign 是低频操作，且确认阶段的 HEAD 校验是最终兜底。不引入 `BEGIN IMMEDIATE` 或预占表。

### 2. 下载统一 302

- `FilesHandler.List`：`download_url` 固定返回 `/api/files/{id}/download`（仅 `completed` 且未过期时；其余返回空串）。不再预签 R2 直链。
- `ConfirmUpload` / `CompleteMultipartUpload`：同样返回相对路径。
- `GetDownloadURL`：
  - 记录不存在 → `404`
  - `upload_status != "completed"` → `410`「文件不可用」
  - 已过期 → `410`「文件已过期」
  - 其余：签 5 分钟，`302`
- `/s/{code}` 保持现状（302 到 `/api/files/{id}/download`）。
- 前端 `Files.vue` 的 `getDownloadUrl` 统一用 `window.location.origin + /api/files/{id}/download`，不再优先取后端字段。

### 3. SQLite 并发

- DSN 追加：`?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL`。
- `db.SetMaxOpenConns(4)`，避免连接数无上限。

### 4. 时间改为整数 Unix 秒

新增两列并回填，旧 TEXT 列保留读取用于展示：

```sql
ALTER TABLE files ADD COLUMN created_at_unix INTEGER;
ALTER TABLE files ADD COLUMN expires_at_unix INTEGER;
CREATE INDEX IF NOT EXISTS idx_files_expires_unix ON files(expires_at_unix);
```

迁移步骤（幂等）：
1. `PRAGMA table_info(files)` 判断列是否已存在，不存在才 `ALTER`。
2. 对 `expires_at_unix IS NULL` 的行，逐行把 TEXT 时间 Scan 成 `time.Time` 后写回 Unix 秒。驱动写入的格式带内嵌时区偏移，Scan 会保留；`CURRENT_TIMESTAMP` 默认值写的是无偏移的 UTC，驱动按 UTC 解析。两条路径都正确。
3. 单行解析失败不中断迁移：`expires_at` 无法解析时回退为 `created_at_unix + expires_in*86400`；`created_at` 也无法解析时用当前时间并记错误日志。迁移结束后打印处理行数与失败行数。

写入路径（`File.Create`）同时写 TEXT 和整数两列；所有比较、排序、筛选改用整数列。

统计口径修正：`expiringToday` 的边界改为本地时区的次日零点，`expiringThisWeek` 改为本地时间 +7 天。

### 5. 孤儿回收

`upload_id` 落库：

```sql
ALTER TABLE files ADD COLUMN upload_id TEXT;
```

- `InitiateMultipartUpload`：`UPDATE files SET upload_id = ?, upload_status = 'uploading' WHERE id = ?`，检查错误。
- `CompleteMultipartUpload` / `CancelUpload`：优先使用库里的 `upload_id`；请求体带了且与库不一致则 `400`。

清理任务（每小时，启动时立即跑一次）扩展为三类：

- **过期已完成**：现有逻辑不变（删对象 → 标记 `deleted`）。
- **超时未完成**：`upload_status IN ('pending','uploading')` 且 `created_at_unix < now - 24h` → 若 `upload_id` 非空则 `AbortMultipartUpload` → 尝试 `DeleteObject`（忽略 NotFound）→ 标记 `failed`。**基准是创建时间，不是过期时间**：一个声明 30 天有效期的文件如果传了一半就断了，不能占着配额到 31 天后才回收。
- **陈旧失败记录**：`upload_status = 'failed'` 且 `created_at_unix < now - 7d` → 物理删除记录。

`failed` 与 `cancelled` 不计入 `GetCommittedStorage`，状态一改变，配额立即释放。

### 6. 优雅关闭

- `http.Server{ReadHeaderTimeout: 10s}` 替代裸 `ListenAndServe`。
- `signal.NotifyContext(SIGINT, SIGTERM)`，收到信号后 `Shutdown`，超时 15 秒。
- 清理任务的 ticker 也监听同一个 context。

### 7. 构建与镜像

- 用 `golang:1.21-alpine` 容器跑 `go mod tidy` 生成 `backend/go.sum` 并提交。
- `Dockerfile`：`COPY backend/go.mod backend/go.sum ./` + `RUN go mod download`，删除 `go mod tidy`。
- 运行时镜像创建 UID/GID 1000 的 `app` 用户，`chown -R app:app /app`，`USER app`。
- `docker-compose.yml` 的 healthcheck 改为 `/healthz`。
- README 部署段落补充：`sudo chown -R 1000:1000 ./data`（bind mount 权限）。

### 8. 限流与 IP 识别

- 新增环境变量 `TRUSTED_PROXIES`（逗号分隔 CIDR，默认空）。
- `getClientIP` 仅在 `RemoteAddr` 命中受信 CIDR 时解析 `X-Forwarded-For`，并从右往左取第一个非受信地址；否则一律用 `RemoteAddr`。
- 合并 `allowRequest` 的读+写为单条 `INSERT ... ON CONFLICT ... RETURNING`（SQLite ≥ 3.35，驱动 1.14.19 内置版本满足）。
- 清理任务顺带删除 `window_start` 早于 1 小时且 `blocked_until` 为空的限流行。

`handlers/auth.go` 里重复的 `getClientIP` 改为调用 middlewares 的导出版本。

### 9. 死配置与日志

- 删除 `config.AccessToken` 字段与 `.env.example` 中的 `ACCESS_TOKEN` 段落。
- 移除 `main.go` 中每个路由的 `log.Printf("[API] ...")`；保留错误与生命周期日志。
- `setup.go` 的 `TestConnection` 不再把 `err.Error()` 回传给前端，改为固定提示 + 服务端日志。

### 10. 健康检查

新增 `GET /healthz`：`database.DB.Ping()` 成功返回 `200 {"status":"ok"}`，失败返回 `503`。不需要认证。

### 11. 分片尺寸自适应

`partSize = max(20MiB, ceil(size / 9000))`，保证 `totalParts ≤ 9000`，避免调大 `MAX_FILE_SIZE` 后突破 10000 分片上限。

### 12. 统计口径与配额对齐

配额按 `pending + uploading + completed` 计算，而 `/api/stats` 只统计 `completed`，会出现「页面显示已用 3GB，却在 10GB 配额下被 507 拦住」的矛盾。调整：

- `usedSpace` 保持 `completed` 之和，表示真实占用。
- 新增 `committedSpace`：`pending + uploading + completed` 之和，即配额口径。
- `usagePercent` 改用 `committedSpace / totalStorage`。
- 新增 `inFlightSpace`：`committedSpace - usedSpace`。
- 前端存储环在 `inFlightSpace > 0` 时，tooltip 增加一行「上传中 X」。

## 已由实施者决定的取舍

这些点在设计讨论中没有定论，按下面处理；如果你不同意，改这里即可。

1. **孤儿回收基准用 `created_at`，24 小时，`failed` 立即释放配额。** 用 `expires_at` 会让声明 30 天的中断上传占住配额一个月。理由见 §5。
2. **`HeadObject` 非 404 错误返回 502 且不改状态。** 只有明确的「对象不存在」才判 `failed`，避免一次 R2 抖动就永久丢文件。`failed` 之后仍允许重新 confirm。
3. **接受并发超卖，不加事务。** 单用户、低频 presign，确认阶段的 HEAD 是最终闸门。
4. **统计改成 committed 口径并存两套数字。** 见 §12，目的是让 507 可解释。
5. **时间回填按内嵌偏移解析，失败行回退推算，不中断迁移。** 见 §4 步骤 3。

## 明确留给实施者的部分

- 迁移代码的具体组织方式（放 `database` 包还是单独函数）。
- `NoSuchKey` 的判定方式（`smithy.APIError` 的 code，或 `HeadObject` 的 404 状态码）。
- 前端错误提示的文案与展示方式。
- 测试的组织形式。当前项目零测试，本次至少覆盖：配额准入、过期判断、时间迁移、`getClientIP`。

## 验收标准

1. 声明 10MB、实际 PUT 100MB 的文件，确认时被删除且返回错误，库里状态为 `failed`。
2. 已用额度接近上限时，新的预签名请求返回 `507`。
3. 30 天有效期的文件，「复制直链」得到的是本站 URL，打开能正常下载。
4. 并发 50 个请求 + 清理任务同时运行，日志无 `database is locked`。
5. 分片上传到一半重启容器，24 小时后孤儿分片被 abort，记录变 `failed`。
6. `docker stop` 时进程在 15 秒内退出，无 panic。
7. `go build` 在无网络环境下可复现（`go.sum` 存在）。
8. 容器内 `id` 显示 UID 1000。
9. 伪造 `X-Forwarded-For` 无法绕过限流。
10. `/healthz` 在 DB 正常时返回 200。
