<template>
  <header class="app-header">
    <div class="logo">
      <h1>Fyer Scheduler</h1>
    </div>
    <div class="header-actions">
      <div class="system-status">
        <StatusBadge :status="systemStatus.status" />
        <span class="status-text">{{ systemStatusText }}</span>
      </div>
    </div>
  </header>
</template>

<script>
import { ref, computed, onMounted } from 'vue';
import StatusBadge from './StatusBadge.vue';
import { systemApi } from '@/services/api';

export default {
  name: 'AppHeader',
  components: {
    StatusBadge
  },
  setup() {
    const systemStatus = ref({
      status: 'loading',
      version: '',
      activeWorkers: 0,
      runningJobs: 0
    });

    const systemStatusText = computed(() => {
      if (systemStatus.value.status === 'loading') {
        return 'Loading...';
      }
      if (systemStatus.value.status === 'error') {
        return 'System Error';
      }
      return `v${systemStatus.value.version} | ${systemStatus.value.activeWorkers} Workers | ${systemStatus.value.runningJobs} Jobs`;
    });

    const fetchSystemStatus = async () => {
      try {
        const data = await systemApi.getSystemStatus();
        systemStatus.value = {
          status: 'healthy',
          version: data.version || '1.0.0',
          activeWorkers: data.active_workers || 0,
          runningJobs: data.running_jobs || 0
        };
      } catch (error) {
        console.error('Failed to fetch system status:', error);
        systemStatus.value.status = 'error';
      }
    };

    onMounted(() => {
      fetchSystemStatus();
      // Refresh status every 30 seconds
      setInterval(fetchSystemStatus, 30000);
    });

    return {
      systemStatus,
      systemStatusText
    };
  }
};
</script>

<style scoped>
.app-header {
  background-color: #304156;
  color: white;
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.12);
}

.logo h1 {
  font-size: 1.5rem;
  font-weight: 600;
  margin: 0;
}

.system-status {
  display: flex;
  align-items: center;
  font-size: 0.9rem;
}

.status-text {
  margin-left: 8px;
}
</style>