<template>
  <n-tooltip trigger="hover" placement="bottom">
    <template #trigger>
      <span class="version-badge" @click="handleClick">
        {{ displayVersion }}
      </span>
    </template>
    <div class="version-tooltip">
      <div class="version-row">
        <span class="label">版本</span>
        <span class="value">{{ version }}</span>
      </div>
      <div class="version-row">
        <span class="label">提交</span>
        <span class="value mono">{{ commitHash }}</span>
      </div>
      <div class="version-row">
        <span class="label">构建</span>
        <span class="value">{{ formattedBuildTime }}</span>
      </div>
    </div>
  </n-tooltip>
</template>

<script setup>
import { NTooltip } from 'naive-ui'
import { useVersion } from '../composables/useVersion'

const { version, commitHash, displayVersion, formattedBuildTime } = useVersion()

// Click to copy version info
const handleClick = () => {
  const info = `R2Box ${displayVersion} (${commitHash})`
  navigator.clipboard.writeText(info)
}
</script>

<style scoped>
.version-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 500;
  color: #6b7280;
  background: #f3f4f6;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

.version-badge:hover {
  color: #374151;
  background: #e5e7eb;
}

.version-tooltip {
  min-width: 140px;
}

.version-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 2px 0;
  font-size: 12px;
}

.version-row:not(:last-child) {
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  padding-bottom: 4px;
  margin-bottom: 4px;
}

.label {
  color: rgba(255, 255, 255, 0.7);
  margin-right: 12px;
}

.value {
  color: #fff;
  font-weight: 500;
}

.value.mono {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Fira Code', monospace;
}
</style>
