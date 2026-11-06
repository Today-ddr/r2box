# Changelog

All notable changes to R2Box will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `/healthz` 健康检查端点，容器 healthcheck 改用它
- `TRUSTED_PROXIES` 环境变量，只有受信代理的 `X-Forwarded-For` 才被采信
- 上传准入与配额校验：超额返回 507，上传完成后用 `HeadObject` 复核真实大小
- 分片上传的 `uploadID` 落库，超时未完成的上传会被自动中止并释放配额
- 统计接口新增 `committedSpace` / `inFlightSpace`，与配额口径对齐

### Changed
- 下载统一经过 `/api/files/{id}/download`，后端校验过期后 302 到 R2 临时签名链接
- 预签名下载链接有效期收紧到 5 分钟
- SQLite 启用 WAL 与 busy_timeout，时间列改用 Unix 整数比较
- 容器以 UID 1000 非 root 运行，`go.sum` 入库使构建可复现
- 移除无效的 `ACCESS_TOKEN` 配置

### Fixed
- 30 天有效期文件的下载直链超出 SigV4 上限而失效
- 「今日过期」统计按 UTC 截断导致的时区偏差
- 伪造 `X-Forwarded-For` 可绕过限流
- 进程收到 SIGTERM 时无法优雅退出

## [1.1.0] - 2024-12-24

### Added
- Automated version control and release workflow
- Version badge display in UI header (GitHub icon right side)
- Build-time version injection via Vite
- Release script for easy version management (`scripts/release.sh`)

### Changed
- Updated CI/CD workflow to support version injection
- Enhanced Docker build with version arguments
- Added GitHub link to Stats and Files page headers

## [1.0.0] - 2024-01-01

### Added
- Initial release of R2Box
- File upload with presigned URLs
- Multipart upload for large files (>100MB)
- File expiration management
- Storage statistics dashboard
- R2 configuration management
- User authentication system

### Features
- Support for Cloudflare R2 storage
- Automatic file cleanup based on expiration
- Real-time upload progress tracking
- Short URL generation for shared files

[Unreleased]: https://github.com/Today-ddr/r2box/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/Today-ddr/r2box/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/Today-ddr/r2box/releases/tag/v1.0.0
