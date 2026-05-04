<script setup>
import { ref, onMounted } from 'vue'

const cartons = ref([])
const loading = ref(true)

onMounted(async () => {
  try {
    const res = await fetch('http://localhost:8080/api/dashboard/cartons')
    cartons.value = await res.json()
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="animate-fade-up">
    <div class="eyebrow">Inventory</div>
    <h1 style="margin-bottom: 2rem;">Carton Master Data</h1>
    
    <div v-if="loading" class="subtitle">Loading cartons...</div>
    
    <div v-else class="bento-grid">
      <div v-for="carton in cartons" :key="carton.id" class="col-span-4 glass-panel" style="padding: 1px;">
        <div class="glass-panel-inner" style="position: relative;">
          <h2 style="font-family: monospace; font-size: 2rem; margin-bottom: 0.25rem;">{{ carton.code }}</h2>
          <div style="margin-bottom: 1.5rem;">
            <span v-if="carton.is_active" class="badge success">Active</span>
            <span v-else class="badge error">Inactive</span>
          </div>
          
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; color: var(--text-secondary); font-size: 0.9rem;">
            <div>
              <div style="font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.1em; opacity: 0.7;">Dimensions</div>
              <div>{{ carton.length }}x{{ carton.width }}x{{ carton.height }} cm</div>
            </div>
            <div>
              <div style="font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.1em; opacity: 0.7;">Volume</div>
              <div>{{ carton.volume.toLocaleString() }} cm³</div>
            </div>
            <div>
              <div style="font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.1em; opacity: 0.7;">Max Weight</div>
              <div>{{ carton.max_weight }} g</div>
            </div>
          </div>
          
          <svg style="position: absolute; right: 1rem; top: 1.5rem; opacity: 0.05;" width="60" height="60" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5"/><path d="M12 22V12"/></svg>
        </div>
      </div>
      
      <div v-if="cartons.length === 0" class="col-span-12 subtitle" style="text-align: center; padding: 4rem 0;">
        No carton data synchronized yet.
      </div>
    </div>
  </div>
</template>
