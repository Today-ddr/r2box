// Version information composable
// Provides access to build-time injected version constants

export function useVersion() {
  const version = __APP_VERSION__ || '0.0.0'
  const commitHash = __COMMIT_HASH__ || 'dev'
  const buildTime = __BUILD_TIME__ || new Date().toISOString()

  // Format: v1.0.0 or v1.0.0-abc1234 (with commit hash)
  const displayVersion = commitHash !== 'dev'
    ? `v${version}`
    : `v${version}-dev`

  // Full version string for tooltips
  const fullVersion = `v${version} (${commitHash})`

  // Format build time for display
  const formatBuildTime = () => {
    try {
      const date = new Date(buildTime)
      return date.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit'
      })
    } catch {
      return buildTime
    }
  }

  return {
    version,
    commitHash,
    buildTime,
    displayVersion,
    fullVersion,
    formattedBuildTime: formatBuildTime()
  }
}
