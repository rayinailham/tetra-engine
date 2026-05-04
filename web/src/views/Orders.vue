<script setup>
import { ref, onMounted } from 'vue'

const orders = ref([])
const loading = ref(true)
const selectedOrder = ref(null)
const selectedOrderItems = ref([])
const detailsLoading = ref(false)
const detailsError = ref(null)

const fetchOrders = async () => {
  try {
    const res = await fetch('http://localhost:8080/api/dashboard/orders')
    orders.value = await res.json()
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchOrders()
  const interval = setInterval(fetchOrders, 5000)
  return () => clearInterval(interval)
})

const viewDetails = async (id) => {
  detailsLoading.value = true
  selectedOrder.value = null
  selectedOrderItems.value = []
  detailsError.value = null
  
  try {
    const res = await fetch(`http://localhost:8080/api/dashboard/orders/${id}`)
    if (!res.ok) throw new Error('Failed to load order details')
    const data = await res.json()
    selectedOrder.value = data.order
    selectedOrderItems.value = data.items
  } catch (e) {
    detailsError.value = e.message
  } finally {
    detailsLoading.value = false
  }
}

const closeDetails = () => {
  selectedOrder.value = null
}

const getBadgeClass = (status) => {
  switch (status) {
    case 'SYNCED': return 'badge'
    case 'PENDING': return 'badge info'
    case 'RECOMMENDED': return 'badge success'
    case 'PUSHED': return 'badge success'
    case 'NO RECOMMENDATION': return 'badge error'
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
    <div class="eyebrow">Order Management</div>
    <h1 style="margin-bottom: 2rem;">Orders</h1>
    
    <div v-if="loading" class="subtitle">Loading orders...</div>
    
    <div v-else class="glass-panel" style="padding: 1px;">
      <div class="glass-panel-inner" style="padding: 0; overflow-x: auto;">
        <table class="data-table">
          <thead>
            <tr>
              <th>Code</th>
              <th>Status</th>
              <th>Carton Assigned</th>
              <th>Items</th>
              <th>Last Updated</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="order in orders" :key="order.id" @click="viewDetails(order.id)" style="cursor: pointer;" title="Click to view items">
              <td style="font-weight: 500; letter-spacing: 0.05em; font-family: monospace; display: flex; align-items: center; gap: 0.5rem;">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="opacity: 0.5;"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>
                {{ order.code }}
              </td>
              <td><span :class="getBadgeClass(order.status)" :title="order.reason">{{ order.status }}</span></td>
              <td>
                <span v-if="order.carton_id" style="font-family: monospace;">{{ order.carton_id }}</span>
                <span v-else style="color: var(--text-secondary);">-</span>
              </td>
              <td>{{ order.item_count }}</td>
              <td style="color: var(--text-secondary); font-size: 0.85rem;">{{ formatTime(order.updated_at) }}</td>
            </tr>
            <tr v-if="orders.length === 0">
              <td colspan="5" style="text-align: center; color: var(--text-secondary);">No orders found.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    
    <!-- Order Details Modal -->
    <teleport to="body">
      <div v-if="detailsLoading || selectedOrder" class="modal-overlay">
        <div class="modal-content glass-panel animate-fade-up">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem; border-bottom: 1px solid var(--border-color); padding-bottom: 1rem;">
          <h2 style="margin: 0; font-family: monospace;">
            <span v-if="selectedOrder">{{ selectedOrder.code }}</span>
            <span v-else>Loading...</span>
          </h2>
          <button @click="closeDetails" style="background: none; border: none; color: var(--text-secondary); cursor: pointer; padding: 0.5rem;">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
          </button>
        </div>
        
        <div v-if="detailsLoading" class="subtitle" style="text-align: center; padding: 2rem;">Fetching item details...</div>
        <div v-else-if="detailsError" class="badge error">{{ detailsError }}</div>
        
        <div v-else-if="selectedOrder">
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 2rem; background: var(--surface-1); padding: 1.5rem; border-radius: var(--radius-md);">
            <div>
              <div style="font-size: 0.75rem; text-transform: uppercase; color: var(--text-secondary); letter-spacing: 0.1em; margin-bottom: 0.25rem;">Status</div>
              <span :class="getBadgeClass(selectedOrder.status)">{{ selectedOrder.status }}</span>
            </div>
            <div>
              <div style="font-size: 0.75rem; text-transform: uppercase; color: var(--text-secondary); letter-spacing: 0.1em; margin-bottom: 0.25rem;">Carton Assigned</div>
              <div style="font-family: monospace; font-size: 1.1rem;">{{ selectedOrder.carton_id || 'None' }}</div>
            </div>
            <div v-if="selectedOrder.reason" style="grid-column: span 2; margin-top: 0.5rem; padding-top: 1rem; border-top: 1px dashed var(--border-color);">
              <div style="font-size: 0.75rem; text-transform: uppercase; color: var(--text-secondary); letter-spacing: 0.1em; margin-bottom: 0.25rem;">Recommendation Note</div>
              <div style="font-size: 0.95rem; color: var(--text-primary);">{{ selectedOrder.reason }}</div>
            </div>
          </div>
          
          <h3 style="margin-bottom: 1rem; font-size: 1.1rem; color: var(--text-secondary);">Items ({{ selectedOrderItems.length }})</h3>
          
          <div style="max-height: 400px; overflow-y: auto; border: 1px solid var(--border-color); border-radius: var(--radius-sm);">
            <table class="data-table">
              <thead>
                <tr>
                  <th>SKU</th>
                  <th>Name</th>
                  <th>Qty</th>
                  <th>Dimensions (L x W x H)</th>
                  <th>Weight</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in selectedOrderItems" :key="item.id">
                  <td style="font-family: monospace;">{{ item.sku }}</td>
                  <td>{{ item.sku_name || '-' }}</td>
                  <td>{{ item.qty }}</td>
                  <td>{{ item.length }} x {{ item.width }} x {{ item.height }} cm</td>
                  <td>{{ item.weight }} g</td>
                </tr>
                <tr v-if="selectedOrderItems.length === 0">
                  <td colspan="5" style="text-align: center; color: var(--text-secondary);">No items in this order.</td>
                </tr>
              </tbody>
            </table>
          </div>
          </div>
        </div>
      </div>
    </teleport>
  </div>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(8px);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 100;
  padding: 2rem;
}
.modal-content {
  width: 100%;
  max-width: 900px;
  max-height: 90vh;
  overflow-y: auto;
  background: var(--bg-color);
}
</style>
