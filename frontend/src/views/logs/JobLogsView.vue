<template>
  <div class="job-logs-page">
    <!-- Page header -->
    <div class="page-header">
      <div class="page-title">
        <h1>Job Logs: {{ jobName }}</h1>
        <div class="breadcrumb">
          <router-link to="/logs">All Logs</router-link> / Job Logs
        </div>
      </div>
      <div class="page-actions">
        <el-button @click="refreshLogs">Refresh</el-button>
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

    <!-- Loading state -->
    <LoadingSpinner v-if="loading && !logs.length" text="Loading logs..." />

    <!-- Log viewer component -->
    <div v-else class="log-viewer-container">
      <LogViewer 
        :logs="logs" 
        :job-id="jobId"
        @filter="handleFilterChange"
      />
    </div>
  </div>
</template>

<script>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';
import { useStore } from 'vuex';
import { useRoute } from 'vue-router';
import LogViewer from '@/components/logs/LogViewer.vue';
import LoadingSpinner from '@/components/common/LoadingSpinner.vue';

export default {
  name: 'JobLogsView',
  components: {
    LogViewer,
    LoadingSpinner
  },
  props: {
    jobId: {
      type: String,
      required: true
    }
  },
  setup(props) {
    const store = useStore();
    const route = useRoute();
    
    // State
    const logs = ref([]);
    const autoRefresh = ref(false);
    const refreshInterval = ref(null);
    const timeRange = ref(null);
    const level = ref(null);
    const jobName = ref('');

    // Computed
    const loading = computed(() => store.getters['logs/isLoading']);
    const error = computed(() => store.getters['logs/error']);
    
    // Methods
    const loadJobDetails = async () => {
      try {
        await store.dispatch('jobs/fetchJob', props.jobId);
        const job = store.getters['jobs/currentJob'];
        if (job) {
          jobName.value = job.name || 'Unknown Job';
        }
      } catch (err) {
        console.error('Failed to load job details:', err);
      }
    };
    
    const loadLogs = async () => {
      try {
        await store.dispatch('logs/fetchJobLogs', {
          jobId: props.jobId,
          timeRange: timeRange.value
        });
        logs.value = store.getters['logs/allJobLogs'];
      } catch (err) {
        console.error('Failed to load logs:', err);
      }
    };
    
    const refreshLogs = () => {
      loadLogs();
    };
    
    const startAutoRefresh = () => {
      if (refreshInterval.value) clearInterval(refreshInterval.value);
      refreshInterval.value = setInterval(() => {
        loadLogs();
      }, 10000); // Refresh every 10 seconds
    };
    
    const stopAutoRefresh = () => {
      if (refreshInterval.value) {
        clearInterval(refreshInterval.value);
        refreshInterval.value = null;
      }
    };
    
    const handleFilterChange = (filters) => {
      timeRange.value = filters.timeRange;
      level.value = filters.level;
      loadLogs();
    };
    
    const clearError = () => {
      store.commit('logs/SET_ERROR', null);
    };
    
    // Watch auto-refresh toggle
    watch(autoRefresh, (newValue) => {
      if (newValue) {
        startAutoRefresh();
      } else {
        stopAutoRefresh();
      }
    });
    
    // Watch for job ID changes
    watch(() => props.jobId, () => {
      loadJobDetails();
      loadLogs();
    });
    
    // Initialize
    onMounted(() => {
      loadJobDetails();
      loadLogs();
      
      // Set up auto-refresh if enabled
      if (autoRefresh.value) {
        startAutoRefresh();
      }
    });
    
    // Clean up
    onBeforeUnmount(() => {
      stopAutoRefresh();
    });
    
    return {
      logs,
      loading,
      error,
      jobName,
      autoRefresh,
      refreshLogs,
      clearError,
      handleFilterChange
    };
  }
};
</script>

<style scoped>
.job-logs-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-title {
  display: flex;
  flex-direction: column;
}

.breadcrumb {
  font-size: 14px;
  margin-top: 4px;
  color: #909399;
}

.breadcrumb a {
  color: #409EFF;
  text-decoration: none;
}

.breadcrumb a:hover {
  text-decoration: underline;
}

.alert-message {
  margin-bottom: 16px;
}

.log-viewer-container {
  background-color: white;
  border-radius: 4px;
  padding: 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.auto-refresh-toggle {
  display: flex;
  align-items: center;
  margin-top: 16px;
  gap: 12px;
}

.refresh-hint {
  font-size: 13px;
  color: #909399;
}
</style>