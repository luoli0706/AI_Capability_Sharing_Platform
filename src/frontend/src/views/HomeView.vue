<template>
  <div style="padding: 40px; max-width: 720px; margin: 0 auto;">
    <el-card shadow="hover">
      <template #header>
        <span>AI 能力共享平台 (ASCP) — 初始化就绪</span>
      </template>
      <p>前端框架：Vue 3 + TypeScript + Vite + Element Plus + Pinia + ECharts</p>
      <el-button type="primary" @click="checkHealth">检测后端健康状态</el-button>
      <p v-if="healthStatus" style="margin-top: 16px; color: #67c23a;">
        后端响应：{{ healthStatus }}
      </p>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import request from '@/api/request'

const healthStatus = ref('')

async function checkHealth() {
  const res = await request.get('/healthz')
  healthStatus.value = JSON.stringify(res.data)
}
</script>
