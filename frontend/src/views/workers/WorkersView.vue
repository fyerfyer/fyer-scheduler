<template>
  <div class="workers-page">
    <!-- Page header with stats -->
    <div class="page-header">
      <h1>Workers</h1>
      <div class="header-actions">
        <el-button
          type="primary"
          icon="el-icon-refresh"
          :loading="isLoading"
          @click="refreshWorkers"
        >
          Refresh
        </el-button>
      </div>
    </div>

    <!-- Stats cards -->
    <div class="stats-cards">
      <div class="stat-card" :class="{ 'clickable': true }" @click="filterByStatus('')">
        <div class="stat-card-value">{{ workersByStatus.total }}</div>
        <div class="stat-card-label">
          <span>Total Workers</span>
        </div>
      </div>

      <div
        class="stat-card healthy-card clickable"
        @click="filterByStatus('healthy')"
      >
        <div class="stat-card-value">{{ workersByStatus.healthy }}</div>
        <div class="stat-card-label">
          <StatusBadge status="healthy" size="small" />
          <span>Healthy</span>
        </div>
      </div>

      <div
        class="stat-card unhealthy-card clickable"
        @click="filterByStatus('unhealthy')"
      >
        <div class="stat-card-value">{{ workersByStatus.unhealthy }}</div>
        <div class="stat-card-label">
          <StatusBadge status="unhealthy" size="small" />
          <span>Unhealthy</span>
        </div>
      </div>

      <div
        class="stat-card offline-card clickable"
        @click="filterByStatus('offline')"
      >
        <div class="stat-card-value">{{ workersByStatus.offline }}</div>
        <div class="stat-card-label">
          <StatusBadge status="disabled" size="small" />
          <span>Offline</span>
        </div>
      </div>
    </div>

    <!-- Error alert -->
    <el-alert
      v-if="error"
      :title="error"
      type="error"
      show-icon
      closable
      @close="clearError"
      class="alert-message"
    />

    <!-- Success message -->
    <el-alert
      v-if="successMessage"
      :title="successMessage"
      type="success"
      show-icon
      closable
      @close="clearSuccessMessage"
      class="alert-message"
    />

    <!-- Worker list -->
    <div class="worker-list-wrapper">
      <WorkerList
        :initialStatusFilter="statusFilter"
        @refresh="refreshWorkers"
      />
    </div>

    <!-- Connection panel -->
    <el-card class="connection-panel" v-if="workersByStatus.total === 0">
      <template #header>
        <div class="connection-header">
          <h3>Connect Workers</h3>
        </div>
      </template>

      <div class="connection-instructions">
        <h4>How to connect workers to the scheduler:</h4>

        <div class="instruction-step">
          <div class="step-number">1</div>
          <div class="step-content">
            <strong>Install the worker agent</strong>
            <p>Download and install the Fyer Scheduler worker agent on each machine.</p>
            <div class="code-block">
              <pre>curl -sSL https://fyerscheduler.io/install.sh | sh</pre>
            </div>
          </div>
        </div>

        <div class="instruction-step">
          <div class="step-number">2</div>
          <div class="step-content">
            <strong>Configure the worker</strong>
            <p>Edit the configuration file to point to your master server.</p>
            <div class="code-block">
              <pre>sudo nano /etc/fyer-scheduler/worker.yaml</pre>
            </div>
            <p>Set the master endpoint:</p>
            <div class="code-block">
              <pre>master:
  endpoint: "{{ masterEndpoint }}"</pre>
            </div>
          </div>
        </div>

        <div class="instruction-step">
          <div class="step-number">3</div>
          <div class="step-content">
            <strong>Start the worker service</strong>
            <p>Start and enable the worker service to run on system boot.</p>
            <div class="code-block">
              <pre>sudo systemctl start fyer-scheduler-worker
sudo systemctl enable fyer-scheduler-worker</pre>
            </div>
          </div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script>
import { computed, ref, onMounted, onBeforeUnmount } from 'vue';
import { useStore } from 'vuex';
import { useRouter, useRoute } from 'vue-router';
import StatusBadge from '@/components/common/StatusBadge.vue';
import WorkerList from '@/components/workers/WorkerList.vue';

export default {
  name: 'WorkersView',
  components: {
    StatusBadge,
    WorkerList
  },
  setup() {
    const store = useStore();
    const router = useRouter();
    const route = useRoute();
    const autoRefreshInterval = ref(null);

    // Get status filter from query parameter if present
    const statusFilter = ref(route.query.status || '');

    // Computed properties
    const workersByStatus = computed(() => store.getters['workers/workersByStatus']);
    const error = computed(() => store.getters['workers/error']);
    const successMessage = computed(() => store.getters['workers/successMessage']);
    const isLoading = computed(() => store.getters['workers/isLoading']);

    // Master endpoint for worker connection instructions
    const masterEndpoint = computed(() => {
      return window.location.hostname + ':8080';
    });

    // Methods
    const refreshWorkers = async () => {
      try {
        await store.dispatch('workers/fetchWorkers');
      } catch (err) {
        console.error('Failed to refresh workers:', err);
      }
    };

    const filterByStatus = (status) => {
      statusFilter.value = status;
      store.dispatch('workers/setStatusFilter', status);

      // Update URL query parameter
      if (status) {
        router.push({ query: { status } });
      } else {
        router.push({ query: {} });
      }
    };

    const clearError = () => {
      store.commit('workers/SET_ERROR', null);
    };

    const clearSuccessMessage = () => {
      store.commit('workers/SET_SUCCESS_MESSAGE', null);
    };

    // Set up auto-refresh
    const startAutoRefresh = () => {
      // Refresh workers every 30 seconds
      autoRefreshInterval.value = setInterval(() => {
        refreshWorkers();
      }, 30000);
    };

    const stopAutoRefresh = () => {
      if (autoRefreshInterval.value) {
        clearInterval(autoRefreshInterval.value);
        autoRefreshInterval.value = null;
      }
    };

    // Initialize
    onMounted(() => {
      refreshWorkers();
      startAutoRefresh();

      // Apply initial filter from URL
      if (statusFilter.value) {
        store.dispatch('workers/setStatusFilter', statusFilter.value);
      }
    });

    // Clean up
    onBeforeUnmount(() => {
      stopAutoRefresh();
    });

    return {
      workersByStatus,
      error,
      successMessage,
      isLoading,
      statusFilter,
      masterEndpoint,
      refreshWorkers,
      filterByStatus,
      clearError,
      clearSuccessMessage
    };
  }
};
</script>

<style scoped>
.workers-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
  margin-bottom: 8px;
}

.stat-card {
  background-color: white;
  border-radius: 4px;
  padding: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  display: flex;
  flex-direction: column;
  transition: transform 0.2s;
}

.clickable {
  cursor: pointer;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
}

.healthy-card:hover {
  border-left: 3px solid #52c41a;
}

.unhealthy-card:hover {
  border-left: 3px solid #f5222d;
}

.offline-card:hover {
  border-left: 3px solid #8c8c8c;
}

.stat-card-value {
  font-size: 28px;
  font-weight: 600;
  margin-bottom: 8px;
}

.stat-card-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #606266;
}

.worker-list-wrapper {
  background-color: white;
  border-radius: 4px;
  padding: 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.alert-message {
  margin-bottom: 0;
}

.connection-panel {
  margin-top: 16px;
}

.connection-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
}

.connection-instructions {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.connection-instructions h4 {
  margin-bottom: 16px;
  color: #303133;
}

.instruction-step {
  display: flex;
  gap: 16px;
}

.step-number {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 32px;
  background-color: #1890ff;
  color: white;
  border-radius: 50%;
  font-weight: bold;
}

.step-content {
  flex: 1;
}

.step-content strong {
  display: block;
  margin-bottom: 8px;
  color: #303133;
}

.step-content p {
  margin-bottom: 8px;
  color: #606266;
}

.code-block {
  background-color: #f8f8f8;
  border-radius: 4px;
  padding: 12px;
  margin: 8px 0;
  overflow-x: auto;
}

.code-block pre {
  margin: 0;
  font-family: 'Courier New', Courier, monospace;
  color: #333;
}

@media (max-width: 768px) {
  .stats-cards {
    grid-template-columns: repeat(2, 1fr);
  }

  .instruction-step {
    flex-direction: column;
    gap: 8px;
  }

  .step-number {
    align-self: flex-start;
  }
}
</style>