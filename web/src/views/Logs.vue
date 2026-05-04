<script setup>
import { ref, onMounted } from 'vue'

const logs = ref([])
const loading = ref(true)

const fetchLogs = async () => {
  try {
    const res = await fetch('http://localhost:8080/api/dashboard/logs?limit=50')
    logs.value = await res.json()
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchLogs()
  // Auto refresh every 10 seconds
  setInterval(fetchLogs, 10000)
})

const getBadgeClass = (status) => {
  switch (status) {
    case 'RUNNING': return 'badge info'
    case 'SUCCESS': return 'badge success'
    case 'FAILED': return 'badge error'
    default: return 'badge'
  }
}

const formatTime = (ts) => {
  if (!ts) return '-'
  return ts.replace('T', ' ') + ' WIB'
}
</script>

<template>
  <div class="animate-fade-up">
    <div class="eyebrow">Monitoring</div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem;">
      <h1 style="margin: 0;">Scheduler Logs</h1>
      <div class="subtitle" style="font-size: 0.85rem;">Live update enabled</div>
    </div>
    
    <div v-if="loading && logs.length === 0" class="subtitle">Loading logs...</div>
    
    <div v-else class="glass-panel" style="padding: 1px;">
      <div class="glass-panel-inner" style="padding: 0; overflow-x: auto;">
        <table class="data-table">
          <thead>
            <tr>
              <th>Job Name</th>
              <th>Status</th>
              <th>Records Processed</th>
              <th>Started At</th>
              <th>Finished At</th>
              <th>Error</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="log in logs" :key="log.id">
              <td style="font-weight: 500;">{{ log.scheduler_name }}</td>
              <td><span :class="getBadgeClass(log.status)">{{ log.status }}</span></td>
              <td>{{ log.records_processed !== null ? log.records_processed : '-' }}</td>
              <td style="color: var(--text-secondary); font-size: 0.85rem;">{{ formatTime(log.started_at) }}</td>
              <td style="color: var(--text-secondary); font-size: 0.85rem;">{{ formatTime(log.finished_at) }}</td>
              <td style="color: #ef4444; font-size: 0.85rem; max-width: 200px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;" :title="log.error_message">
                {{ log.error_message || '-' }}
              </td>
            </tr>
            <tr v-if="logs.length === 0">
              <td colspan="6" style="text-align: center; color: var(--text-secondary);">No logs found.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
