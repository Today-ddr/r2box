# Changelog

All notable changes to R2Box will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
