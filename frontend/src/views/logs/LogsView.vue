<template>
  <div class="logs-page">
    <!-- Page header -->
    <div class="page-header">
      <div class="page-title">
        <h1>{{ selectedJob ? `Logs: ${selectedJobName}` : 'Logs' }}</h1>
        <div v-if="selectedJob" class="breadcrumb">
          <router-link to="/logs">All Logs</router-link> /
          <span>{{ selectedJobName }}</span>
        </div>
      </div>

      <div class="page-actions">
        <el-radio-group v-model="viewType" size="small" class="view-type-selector">
          <el-radio-button label="job">Job Logs</el-radio-button>
          <el-radio-button label="system">System Logs</el-radio-button>
        </el-radio-group>

        <el-select
          v-if="viewType === 'job' && !selectedJob"
          v-model="selectedJobId"
          filterable
          placeholder="Select job"
          size="small"
          clearable
          @change="handleJobChange"
        >
          <el-option
            v-for="job in jobs"
            :key="job.id"
            :label="job.name"
            :value="job.id"
          />
        </el-select>

        <el-button
          type="primary"
          size="small"
          @click="refreshLogs"
          :loading="isLoading"
        >
          Refresh
        </el-button>
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

    <!-- Log viewer component -->
    <div class="log-viewer-container">
      <LogViewer
        :job-id="selectedJobId"
        :auto-refresh="autoRefresh"
        v-if="viewType === 'job'"
      />

      <div v-else-if="viewType === 'system'" class="system-logs">
        <el-alert
          type="info"
          show-icon
          :closable="false"
          title="System logs display all scheduler related events like job scheduling, worker registration, etc."
          class="info-box"
        />

        <LoadingSpinner v-if="isLoading" text="Loading system logs..." />

        <div v-else-if="systemLogs.length === 0" class="empty-logs">
          <i class="el-icon-document"></i>
          <p>No system logs available</p>
        </div>

        <div v-else class="system-logs-container">
          <div
            v-for="(log, index) in systemLogs"
            :key="index"
            class="system-log-entry"
            :class="`log-level-${log.level.toLowerCase()}`"
          >
            <div class="log-header">
              <span class="log-timestamp">{{ formatTimestamp(log.timestamp) }}</span>
              <span class="log-level">{{ log.level }}</span>
              <span class="log-component" v-if="log.component">{{ log.component }}</span>
            </div>
            <div class="log-message">{{ log.message }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Auto-refresh toggle -->
    <div class="auto-refresh-toggle">
      <el-switch
        v-model="autoRefresh"
        active-text="Auto-refresh"
        inactive-text="Manual refresh"
      />
      <span class="refresh-hint" v-if="autoRefresh">
        Logs will refresh automatically every {{ refreshInterval / 1000 }} seconds
      </span>
    </div>
  </div>
</template>

<script>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';
import { useStore } from 'vuex';
import { useRoute, useRouter } from 'vue-router';
import LogViewer from '@/components/logs/LogViewer.vue';
import LoadingSpinner from '@/components/common/LoadingSpinner.vue';

export default {
  name: 'LogsView',
  components: {
    LogViewer,
    LoadingSpinner
  },
  setup() {
    const store = useStore();
    const route = useRoute();
    const router = useRouter();

    // State
    const viewType = ref('job');
    const selectedJobId = ref(null);
    const autoRefresh = ref(false);
    const refreshInterval = ref(5000); // 5 seconds
    const refreshTimer = ref(null);

    // Computed properties
    const selectedJob = computed(() => {
      if (!selectedJobId.value) return null;
      return jobs.value.find(job => job.id === selectedJobId.value);
    });

    const selectedJobName = computed(() => {
      return selectedJob.value ? selectedJob.value.name : '';
    });

    const jobs = computed(() => store.getters['jobs/allJobs']);
    const systemLogs = computed(() => store.getters['logs/systemLogs']);
    const isLoading = computed(() => store.getters['logs/isLoading']);
    const error = computed(() => store.getters['logs/error']);

    // Methods
    const loadJobs = async () => {
      if (jobs.value.length === 0) {
        try {
          await store.dispatch('jobs/fetchJobs');
        } catch (err) {
          console.error('Failed to load jobs:', err);
        }
      }
    };

    const loadLogs = async () => {
      if (viewType.value === 'job') {
        if (selectedJobId.value) {
          await loadJobLogs();
        }
      } else {
        await loadSystemLogs();
      }
    };

    const loadJobLogs = async () => {
      try {
        await store.dispatch('logs/fetchJobLogs', {
          jobId: selectedJobId.value
        });
      } catch (err) {
        console.error('Failed to load job logs:', err);
      }
    };

    const loadSystemLogs = async () => {
      try {
        await store.dispatch('logs/fetchSystemLogs');
      } catch (err) {
        console.error('Failed to load system logs:', err);
      }
    };

    const refreshLogs = () => {
      loadLogs();
    };

    const handleJobChange = (jobId) => {
      if (jobId) {
        router.push({ path: `/logs/jobs/${jobId}` });
      } else {
        router.push({ path: '/logs' });
      }
    };

    const clearError = () => {
      store.commit('logs/SET_ERROR', null);
    };

    const setupAutoRefresh = () => {
      if (autoRefresh.value) {
        refreshTimer.value = setInterval(() => {
          refreshLogs();
        }, refreshInterval.value);
      }
    };

    const clearAutoRefresh = () => {
      if (refreshTimer.value) {
        clearInterval(refreshTimer.value);
        refreshTimer.value = null;
      }
    };

    const formatTimestamp = (timestamp) => {
      if (!timestamp) return '';

      try {
        const date = new Date(timestamp);
        return date.toLocaleString();
      } catch (e) {
        return timestamp;
      }
    };

    // Handle route changes
    const handleRouteChange = () => {
      const jobId = route.params.jobId;

      if (jobId) {
        viewType.value = 'job';
        selectedJobId.value = jobId;
      } else {
        if (route.path === '/logs/system') {
          viewType.value = 'system';
          selectedJobId.value = null;
        } else {
          viewType.value = 'job';
          selectedJobId.value = null;
        }
      }

      loadLogs();
    };

    // Watch for changes
    watch(() => route.params.jobId, () => {
      handleRouteChange();
    });

    watch(() => viewType.value, (newValue) => {
      if (newValue === 'system') {
        router.push('/logs/system');
      } else {
        if (selectedJobId.value) {
          router.push(`/logs/jobs/${selectedJobId.value}`);
        } else {
          router.push('/logs');
        }
      }
    });

    watch(() => autoRefresh.value, (newValue) => {
      if (newValue) {
        setupAutoRefresh();
      } else {
        clearAutoRefresh();
      }
    });

    // Lifecycle hooks
    onMounted(async () => {
      await loadJobs();
      handleRouteChange();

      if (autoRefresh.value) {
        setupAutoRefresh();
      }
    });

    onBeforeUnmount(() => {
      clearAutoRefresh();
    });

    return {
      viewType,
      selectedJobId,
      selectedJob,
      selectedJobName,
      autoRefresh,
      refreshInterval,
      jobs,
      systemLogs,
      isLoading,
      error,
      refreshLogs,
      handleJobChange,
      clearError,
      formatTimestamp
    };
  }
};
</script>

<style scoped>
.logs-page {
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

.view-type-selector {
  margin-right: 16px;
}

.log-viewer-container {
  background-color: white;
  border-radius: 4px;
  padding: 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.info-box {
  margin-bottom: 16px;
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

.system-logs-container {
  background-color: #1e1e1e;
  border-radius: 4px;
  padding: 16px;
  max-height: 600px;
  overflow-y: auto;
  color: #d4d4d4;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.5;
}

.system-log-entry {
  padding: 4px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.system-log-entry:last-child {
  border-bottom: none;
}

.log-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 2px;
}

.log-timestamp {
  color: #6a9955;
  font-size: 12px;
}

.log-level {
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: bold;
}

.log-component {
  font-size: 11px;
  color: #c586c0;
  font-style: italic;
}

.log-level-info .log-level {
  background-color: rgba(64, 158, 255, 0.1);
  color: #409EFF;
}

.log-level-warn .log-level {
  background-color: rgba(230, 162, 60, 0.1);
  color: #E6A23C;
}

.log-level-error .log-level {
  background-color: rgba(245, 108, 108, 0.1);
  color: #F56C6C;
}

.log-level-debug .log-level {
  background-color: rgba(144, 147, 153, 0.1);
  color: #909399;
}

.log-message {
  white-space: pre-wrap;
  word-break: break-word;
}

.empty-logs {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 48px 0;
  color: #909399;
}

.empty-logs i {
  font-size: 48px;
  margin-bottom: 16px;
}
</style>