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
  <div class="animate-fade-up">
    <div class="eyebrow">Configuration</div>
    <h1 style="margin-bottom: 0.5rem;">System Settings</h1>
    <p class="subtitle" style="margin-bottom: 3rem;">Fine-tune engine intervals and manage core data states.</p>

    <div class="bento-grid">
      <!-- Scheduler Intervals -->
      <div class="col-span-8 glass-panel" style="padding: 1px;">
        <div class="glass-panel-inner" style="display: flex; flex-direction: column; gap: 2rem;">
          <div>
            <h2 style="color: var(--text-primary); margin-bottom: 0.5rem;">Scheduler Intervals</h2>
            <p class="subtitle" style="font-size: 0.85rem;">Adjust the background sync frequency. Requires standard time durations (e.g., 5m, 30s).</p>
          </div>
          
          <div v-if="loading" class="subtitle" style="padding: 2rem 0;">Loading settings...</div>
          
          <form v-else @submit.prevent="saveSettings" style="display: flex; flex-direction: column; gap: 2rem;">
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 2rem;">
              <div class="form-group">
                <label>Order Sync Interval</label>
                <div class="input-wrapper">
                  <input type="text" v-model="settings.order_sync_interval" class="form-input" placeholder="15m" required>
                  <div class="input-icon"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></div>
                </div>
                <span class="help-text">Pull new orders from Flux</span>
              </div>
              
              <div class="form-group">
                <label>Carton Sync Interval</label>
                <div class="input-wrapper">
                  <input type="text" v-model="settings.carton_sync_interval" class="form-input" placeholder="30m" required>
                  <div class="input-icon"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5"/><path d="M12 22V12"/></svg></div>
                </div>
                <span class="help-text">Update carton master data</span>
              </div>

              <div class="form-group">
                <label>Recommendation Interval</label>
                <div class="input-wrapper">
                  <input type="text" v-model="settings.recommendation_interval" class="form-input" placeholder="5m" required>
                  <div class="input-icon"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="m12 16 4-4-4-4"/><path d="M8 12h8"/></svg></div>
                </div>
                <span class="help-text">Compute recommendations</span>
              </div>

              <div class="form-group">
                <label>Push Interval</label>
                <div class="input-wrapper">
                  <input type="text" v-model="settings.push_interval" class="form-input" placeholder="5m" required>
                  <div class="input-icon"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14"/><path d="m12 5 7 7-7 7"/></svg></div>
                </div>
                <span class="help-text">Push results to Flux</span>
              </div>
            </div>
            
            <div style="display: flex; justify-content: space-between; align-items: center; padding-top: 1.5rem; border-top: 1px dashed var(--border-color);">
              <div style="display: flex; gap: 1rem; align-items: center;">
                <div v-if="message && !error" class="badge success">{{ message }}</div>
                <div v-if="error" class="badge error">{{ error }}</div>
              </div>
              <button type="submit" class="btn-primary" :disabled="saving" style="background: rgba(16,185,129,0.1); border-color: rgba(16,185,129,0.3); color: #10b981;">
                <svg v-if="!saving" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>
                <svg v-else class="spin" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
                {{ saving ? 'Applying...' : 'Apply Changes' }}
              </button>
            </div>
          </form>
        </div>
      </div>

      <!-- Danger Zone -->
      <div class="col-span-4 glass-panel danger-panel" style="padding: 1px; display: flex; flex-direction: column;">
        <div class="glass-panel-inner danger-inner" style="flex: 1; display: flex; flex-direction: column; justify-content: space-between;">
          <div>
            <div class="eyebrow" style="background: rgba(239, 68, 68, 0.2); color: #ef4444; border: 1px solid rgba(239, 68, 68, 0.3);">Danger Zone</div>
            <h2 style="color: #ef4444; margin-bottom: 0.5rem; font-size: 1.75rem;">System Reset</h2>
            <p class="subtitle" style="font-size: 0.9rem; line-height: 1.5; margin-bottom: 2rem; color: rgba(255,255,255,0.7);">
              Permanently wipe the database: orders, items, products, cartons, and logs. This process is irreversible and forces the engine to halt.
            </p>
          </div>
          
          <button @click="resetDatabase" class="btn-primary" style="background: rgba(239,68,68,0.1); border-color: rgba(239,68,68,0.3); color: #ef4444; width: 100%; justify-content: center; padding: 1rem;" :disabled="resetting">
            <svg v-if="!resetting" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/><line x1="10" y1="11" x2="10" y2="17"/><line x1="14" y1="11" x2="14" y2="17"/></svg>
            <svg v-else class="spin" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
            {{ resetting ? 'Purging Database...' : 'Erase Everything' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.form-group label {
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.help-text {
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.4);
}

.form-input {
  width: 100%;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  padding: 0.875rem 1rem 0.875rem 2.5rem; /* left padding for icon */
  color: #fff;
  font-family: inherit;
  font-size: 0.95rem;
  transition: all 0.2s ease;
  box-shadow: inset 0 2px 4px rgba(0,0,0,0.1);
}

.form-input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px rgba(16, 185, 129, 0.1), inset 0 2px 4px rgba(0,0,0,0.1);
}

.input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.input-icon {
  position: absolute;
  left: 0.75rem;
  color: var(--text-secondary);
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.danger-panel {
  border-color: rgba(239, 68, 68, 0.2);
}
.danger-panel:hover {
  border-color: rgba(239, 68, 68, 0.4);
}
.danger-inner {
  background: linear-gradient(180deg, rgba(239, 68, 68, 0.05) 0%, rgba(239, 68, 68, 0.01) 100%);
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
.spin {
  animation: spin 1s linear infinite;
}
</style>
