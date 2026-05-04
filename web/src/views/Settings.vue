<script setup>
import { ref, onMounted } from 'vue';

const API_BASE = 'http://localhost:8080/api/dashboard';

const loading = ref(false);
const saving = ref(false);
const resetting = ref(false);
const message = ref('');
const error = ref('');

const settings = ref({
  order_sync_interval: '',
  carton_sync_interval: '',
  recommendation_interval: '',
  push_interval: ''
});

const fetchSettings = async () => {
  loading.value = true;
  try {
    const res = await fetch(`${API_BASE}/settings`);
    if (res.ok) {
      settings.value = await res.json();
    }
  } catch (e) {
    console.error('Failed to load settings', e);
  } finally {
    loading.value = false;
  }
};

const saveSettings = async () => {
  saving.value = true;
  message.value = '';
  error.value = '';
  try {
    const res = await fetch(`${API_BASE}/settings`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings.value)
    });
    
    if (!res.ok) {
      const data = await res.json();
      error.value = data.error || 'Failed to save settings';
    } else {
      message.value = 'Settings updated successfully';
    }
  } catch (e) {
    error.value = 'Connection error';
  } finally {
    saving.value = false;
  }
};

const resetDatabase = async () => {
  if (!confirm('Are you completely sure you want to reset the database? All records will be permanently deleted.')) {
    return;
  }
  
  resetting.value = true;
  message.value = '';
  error.value = '';
  
  try {
    const res = await fetch(`${API_BASE}/engine/reset`, { method: 'POST' });
    if (res.ok) {
      const data = await res.json();
      message.value = data.message;
    } else {
      error.value = 'Failed to reset database';
    }
  } catch (e) {
    error.value = 'Connection error';
  } finally {
    resetting.value = false;
  }
};

onMounted(() => {
  fetchSettings();
});
</script>

<template>
  <div class="page-header">
    <div>
      <h1 class="page-title">Settings</h1>
      <p style="color: var(--text-secondary); margin-top: 0.25rem;">Configure engine intervals and system state</p>
    </div>
  </div>

  <div class="glass-panel" style="margin-bottom: 2rem;">
    <div class="glass-panel-inner" style="padding: 2rem;">
      <h2 style="font-size: 1.25rem; font-weight: 600; margin-bottom: 1.5rem; color: #fff;">Scheduler Intervals</h2>
      
      <div v-if="loading" style="color: var(--text-secondary);">Loading settings...</div>
      
      <form v-else @submit.prevent="saveSettings">
        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem; margin-bottom: 2rem;">
          <div class="form-group">
            <label style="display: block; margin-bottom: 0.5rem; font-size: 0.875rem; color: var(--text-secondary);">Order Sync Interval</label>
            <input type="text" v-model="settings.order_sync_interval" class="form-input" placeholder="e.g. 15m" required>
            <span style="font-size: 0.75rem; color: #666; margin-top: 0.25rem; display: block;">Time between pulling new orders from Flux.</span>
          </div>
          
          <div class="form-group">
            <label style="display: block; margin-bottom: 0.5rem; font-size: 0.875rem; color: var(--text-secondary);">Carton Sync Interval</label>
            <input type="text" v-model="settings.carton_sync_interval" class="form-input" placeholder="e.g. 30m" required>
            <span style="font-size: 0.75rem; color: #666; margin-top: 0.25rem; display: block;">Time between updating carton master data.</span>
          </div>

          <div class="form-group">
            <label style="display: block; margin-bottom: 0.5rem; font-size: 0.875rem; color: var(--text-secondary);">Recommendation Interval</label>
            <input type="text" v-model="settings.recommendation_interval" class="form-input" placeholder="e.g. 5m" required>
            <span style="font-size: 0.75rem; color: #666; margin-top: 0.25rem; display: block;">How often the engine computes recommendations.</span>
          </div>

          <div class="form-group">
            <label style="display: block; margin-bottom: 0.5rem; font-size: 0.875rem; color: var(--text-secondary);">Push Interval</label>
            <input type="text" v-model="settings.push_interval" class="form-input" placeholder="e.g. 5m" required>
            <span style="font-size: 0.75rem; color: #666; margin-top: 0.25rem; display: block;">How often the engine pushes results back to Flux.</span>
          </div>
        </div>
        
        <div style="display: flex; gap: 1rem; align-items: center;">
          <button type="submit" class="button button-primary" :disabled="saving">
            {{ saving ? 'Saving...' : 'Save Settings' }}
          </button>
          <span v-if="message && !error" style="color: #10b981; font-size: 0.875rem;">{{ message }}</span>
          <span v-if="error" style="color: #ef4444; font-size: 0.875rem;">{{ error }}</span>
        </div>
      </form>
    </div>
  </div>

  <div class="glass-panel" style="border-color: rgba(239, 68, 68, 0.2);">
    <div class="glass-panel-inner" style="padding: 2rem;">
      <h2 style="font-size: 1.25rem; font-weight: 600; margin-bottom: 0.5rem; color: #ef4444;">Danger Zone</h2>
      <p style="color: var(--text-secondary); font-size: 0.875rem; margin-bottom: 1.5rem;">
        Resetting the database will permanently delete all orders, items, products, cartons, and scheduler logs. Use this only for starting a fresh presentation or debugging. Engine will be stopped automatically.
      </p>
      
      <button @click="resetDatabase" class="button" style="background: rgba(239, 68, 68, 0.1); color: #ef4444; border: 1px solid rgba(239, 68, 68, 0.2);" :disabled="resetting">
        {{ resetting ? 'Resetting...' : 'Reset Database' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.form-input {
  width: 100%;
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 0.75rem 1rem;
  color: #fff;
  font-family: inherit;
  font-size: 0.875rem;
  transition: all 0.2s ease;
}

.form-input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.1);
}
</style>
