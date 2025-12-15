<template>
  <n-config-provider :theme="null" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <n-notification-provider>
          <div id="app">
            <router-view />
          </div>
        </n-notification-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup>
import { NConfigProvider, NMessageProvider, NDialogProvider, NNotificationProvider } from 'naive-ui'

// Modern tech-style theme (Vercel/Stripe inspired)
const themeOverrides = {
  common: {
    primaryColor: '#0070f3',
    primaryColorHover: '#0060df',
    primaryColorPressed: '#0050c0',
    borderRadius: '10px',
    borderRadiusSmall: '8px'
  },
  Button: {
    borderRadiusMedium: '10px',
    borderRadiusSmall: '8px',
    borderRadiusLarge: '12px',
    borderRadiusTiny: '6px',
    colorPrimary: '#0070f3',
    colorHoverPrimary: '#0060df',
    colorPressedPrimary: '#0050c0'
  },
  Card: {
    borderRadius: '16px'
  },
  Input: {
    borderRadius: '10px'
  },
  Tag: {
    borderRadius: '8px'
  },
  Alert: {
    borderRadius: '12px'
  },
  Modal: {
    borderRadius: '20px'
  },
  Dialog: {
    borderRadius: '20px'
  },
  Popconfirm: {
    borderRadius: '12px'
  },
  Popover: {
    borderRadius: '12px'
  },
  Select: {
    borderRadius: '10px'
  },
  Progress: {
    borderRadius: '10px'
  },
  DataTable: {
    borderRadius: '12px'
  },
  Upload: {
    borderRadius: '12px'
  },
  Notification: {
    borderRadius: '14px'
  },
  Message: {
    borderRadius: '10px'
  },
  Descriptions: {
    borderRadius: '12px'
  },
  Steps: {
    indicatorBorderRadius: '50%'
  },
  Radio: {
    buttonBorderRadius: '8px'
  }
}
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  background: #f5f7fa;
}

#app {
  min-height: 100vh;
}
</style>
