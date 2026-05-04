<script setup>
import { ref, onMounted } from 'vue'

const stats = ref(null)
const error = ref(null)
const loading = ref(true)
const engineRunning = ref(false)

const fetchStats = async () => {
  try {
    const res = await fetch('http://localhost:8080/api/dashboard/stats')
    if (!res.ok) throw new Error('Failed to fetch stats')
    stats.value = await res.json()
    
    const engineRes = await fetch('http://localhost:8080/api/dashboard/engine/status')
    if (engineRes.ok) {
      const engineData = await engineRes.json()
      engineRunning.value = engineData.running
    }
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

const triggerJob = async (jobName) => {
  try {
    await fetch(`http://localhost:8080/api/internal/trigger/${jobName}`, { method: 'POST' })
    alert(`Job ${jobName} triggered.`)
    fetchStats()
  } catch (e) {
    alert(`Failed to trigger ${jobName}`)
  }
}

const toggleEngine = async (start) => {
  try {
    const endpoint = start ? 'start' : 'stop'
    await fetch(`http://localhost:8080/api/dashboard/engine/${endpoint}`, { method: 'POST' })
    engineRunning.value = start
    alert(`Engine ${start ? 'started' : 'stopped'} successfully.`)
  } catch (e) {
    alert(`Failed to change engine state`)
  }
}

const formatTime = (ts) => {
  if (!ts) return '-'
  return ts.replace('T', ' ') + ' WIB'
}

onMounted(() => {
  fetchStats()
})
</script>

<template>
  <div class="animate-fade-up">
    <div class="eyebrow">Overview</div>
    <h1>System Status</h1>
    
    <div v-if="loading" class="subtitle">Connecting to Engine...</div>
    <div v-else-if="error" class="badge error">Connection Error: {{ error }}</div>
    
    <div v-else class="bento-grid" style="margin-top: 2rem;">
      <!-- Total Orders Card -->
      <div class="col-span-8 glass-panel" style="padding: 1px;">
        <div class="glass-panel-inner" style="display: flex; flex-direction: column; justify-content: center; position: relative;">
          <h2 style="font-size: 4rem; margin: 0; color: var(--text-primary);">{{ stats.orders.total }}</h2>
          <span class="subtitle" style="font-size: 1.25rem;">Total Synchronized Orders</span>
          
          <div style="display: flex; gap: 1rem; margin-top: 2rem; flex-wrap: wrap;">
            <div class="badge">Synced: {{ stats.orders.synced }}</div>
            <div class="badge info">Pending: {{ stats.orders.pending }}</div>
            <div class="badge warning">Recommended: {{ stats.orders.recommended }}</div>
            <div class="badge success">Pushed: {{ stats.orders.pushed }}</div>
          </div>
          
          <svg style="position: absolute; right: 2rem; bottom: 2rem; opacity: 0.05;" width="120" height="120" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/></svg>
        </div>
      </div>
      
      <!-- Cartons Card -->
      <div class="col-span-4 glass-panel" style="padding: 1px;">
        <div class="glass-panel-inner" style="display: flex; flex-direction: column; justify-content: center;">
          <div class="eyebrow">Master Data</div>
          <h2 style="font-size: 3rem; margin: 0;">{{ stats.cartons.total }}</h2>
          <span class="subtitle">Cartons available</span>
          <div style="margin-top: 1rem;">
            <span class="badge success">{{ stats.cartons.active }} Active</span>
          </div>
        </div>
      </div>
      
      <!-- Quick Actions -->
      <div class="col-span-12 glass-panel" style="padding: 1px;">
        <div class="glass-panel-inner">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem;">
            <h2>Quick Actions</h2>
            <div v-if="stats.last_sync" class="subtitle" style="font-size: 0.85rem;">
              Last Sync: {{ formatTime(stats.last_sync.started_at) }}
            </div>
          </div>
          
          <div style="display: flex; gap: 1rem; flex-wrap: wrap; margin-bottom: 2rem; padding-bottom: 1rem; border-bottom: 1px solid var(--border-color);">
            <button v-if="!engineRunning" @click="toggleEngine(true)" class="btn-primary" style="background: rgba(16,185,129,0.1); border-color: rgba(16,185,129,0.3); color: #10b981;">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
              Start Engine
            </button>
            <button v-else @click="toggleEngine(false)" class="btn-primary" style="background: rgba(239,68,68,0.1); border-color: rgba(239,68,68,0.3); color: #ef4444;">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg>
              Stop Engine
            </button>
          </div>
          
          <div style="display: flex; gap: 1rem; flex-wrap: wrap;">
            <button @click="triggerJob('order_retrieval')" class="btn-primary">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/><path d="m16 5 3 3-3 3"/><path d="M19 8H9"/></svg>
              Pull Orders
            </button>
            <button @click="triggerJob('carton_sync')" class="btn-primary">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5"/><path d="M12 22V12"/></svg>
              Sync Cartons
            </button>
            <button @click="triggerJob('carton_recommendation')" class="btn-primary" style="border-color: rgba(245,158,11,0.5); color: #fcd34d;">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="m12 16 4-4-4-4"/><path d="M8 12h8"/></svg>
              Run Engine
            </button>
            <button @click="triggerJob('carton_push')" class="btn-primary" style="border-color: rgba(16,185,129,0.5); color: #6ee7b7;">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14"/><path d="m12 5 7 7-7 7"/></svg>
              Push Results
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
