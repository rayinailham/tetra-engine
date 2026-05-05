<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const logs = ref([])
const loading = ref(true)
const sortBy = ref('started_at')
const order = ref('desc')
let pollIntervalId = null

const fetchLogs = async () => {
  try {
    const res = await fetch(`http://localhost:8080/api/dashboard/logs?limit=50&sort_by=${sortBy.value}&order=${order.value}`)
    logs.value = await res.json()
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const setSort = (field) => {
  if (sortBy.value === field) {
    order.value = order.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = field
    order.value = 'desc'
  }
  fetchLogs()
}

onMounted(() => {
  fetchLogs()

  // Standard polling for robust updates
  pollIntervalId = setInterval(fetchLogs, 5000)
})

onUnmounted(() => {
  if (pollIntervalId !== null) {
    clearInterval(pollIntervalId)
    pollIntervalId = null
  }
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
      <div>
        <h1 style="margin: 0;">Scheduler Logs</h1>
        <p class="subtitle" style="font-size: 0.85rem; margin-top: 0.25rem;">Live update enabled • Sorted by {{ sortBy === 'started_at' ? 'Started At' : 'Finished At' }} ({{ order.toUpperCase() }})</p>
      </div>
      
      <div style="display: flex; gap: 0.5rem;">
        <button @click="setSort('started_at')" class="btn-primary" :class="{ 'active': sortBy === 'started_at' }" style="font-size: 0.75rem; padding: 0.5rem 1rem;">
          Sort by Start {{ sortBy === 'started_at' ? (order === 'asc' ? '↑' : '↓') : '' }}
        </button>
        <button @click="setSort('finished_at')" class="btn-primary" :class="{ 'active': sortBy === 'finished_at' }" style="font-size: 0.75rem; padding: 0.5rem 1rem;">
          Sort by Finish {{ sortBy === 'finished_at' ? (order === 'asc' ? '↑' : '↓') : '' }}
        </button>
      </div>
    </div>
    
    <div v-if="loading && logs.length === 0" class="subtitle">Loading logs...</div>
    
    <div v-else class="glass-panel" style="padding: 1px;">
      <div class="glass-panel-inner" style="padding: 0; overflow-x: auto;">
        <table class="data-table">
          <thead>
            <tr>
              <th>Job Name</th>
              <th>Status</th>
              <th>Records</th>
              <th @click="setSort('started_at')" style="cursor: pointer; user-select: none;">
                Started At 
                <span v-if="sortBy === 'started_at'">{{ order === 'asc' ? '↑' : '↓' }}</span>
              </th>
              <th @click="setSort('finished_at')" style="cursor: pointer; user-select: none;">
                Finished At
                <span v-if="sortBy === 'finished_at'">{{ order === 'asc' ? '↑' : '↓' }}</span>
              </th>
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

<style scoped>
.active {
  background: var(--surface-2) !important;
  border-color: var(--accent) !important;
  color: var(--accent) !important;
}
</style>
