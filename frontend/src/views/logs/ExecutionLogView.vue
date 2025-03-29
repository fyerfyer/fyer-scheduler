<template>
  <div class="execution-log-view">
    <!-- Page header -->
    <div class="page-header">
      <div class="page-title">
        <h1>Execution Log</h1>
        <div class="breadcrumb">
          <router-link to="/logs">All Logs</router-link>
          <template v-if="execution && execution.job_id">
            / <router-link :to="`/logs/jobs/${execution.job_id}`">{{ jobName }}</router-link>
          </template>
          / <span>{{ executionId }}</span>
        </div>
      </div>

      <div class="page-actions">
        <el-switch
          v-model="streamMode"
          active-text="Live Stream"
          inactive-text="Static View"
          @change="handleStreamModeChange"
        />

        <el-button
          type="primary"
          size="small"
          @click="refreshLogs"
          :loading="isLoading"
        >
          Refresh
        </el-button>

        <el-button
          @click="goToJobDetails"
          size="small"
          v-if="execution && execution.job_id"
        >
          View Job Details
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

    <!-- Loading state -->
    <LoadingSpinner v-if="isLoading && !execution" text="Loading execution details..." />

    <!-- Execution not found -->
    <div v-else-if="!execution && !isLoading" class="not-found">
      <h2>Execution Not Found</h2>
      <p>The requested execution log could not be found.</p>
      <el-button @click="goToLogsList">Back to Logs</el-button>
    </div>

    <!-- Execution details -->
    <div v-else-if="execution" class="execution-details">
      <!-- Execution summary card -->
      <el-card class="summary-card">
        <div class="execution-summary">
          <div class="summary-column">
            <div class="summary-item">
              <div class="summary-label">Status</div>
              <div class="summary-value">
                <StatusBadge :status="execution.status" size="medium" />
              </div>
            </div>

            <div class="summary-item">
              <div class="summary-label">Job Name</div>
              <div class="summary-value">
                <router-link :to="`/jobs/${execution.job_id}`">{{ jobName }}</router-link>
              </div>
            </div>

            <div class="summary-item">
              <div class="summary-label">Execution ID</div>
              <div class="summary-value code-text">{{ executionId }}</div>
            </div>
          </div>

          <div class="summary-column">
            <div class="summary-item">
              <div class="summary-label">Start Time</div>
              <div class="summary-value">{{ formatDateTime(execution.start_time) }}</div>
            </div>

            <div class="summary-item">
              <div class="summary-label">End Time</div>
              <div class="summary-value">
                {{ execution.end_time ? formatDateTime(execution.end_time) : 'Running...' }}
              </div>
            </div>

            <div class="summary-item">
              <div class="summary-label">Duration</div>
              <div class="summary-value">{{ formatDuration(execution.start_time, execution.end_time) }}</div>
            </div>
          </div>

          <div class="summary-column">
            <div class="summary-item">
              <div class="summary-label">Worker</div>
              <div class="summary-value">{{ execution.worker_id || 'Not assigned' }}</div>
            </div>

            <div class="summary-item">
              <div class="summary-label">Exit Code</div>
              <div
                class="summary-value"
                :class="{'exit-code-success': execution.exit_code === 0, 'exit-code-failed': execution.exit_code > 0}"
              >
                {{ execution.exit_code !== undefined ? execution.exit_code : 'N/A' }}
              </div>
            </div>

            <div class="summary-item">
              <div class="summary-label">Attempt</div>
              <div class="summary-value">{{ execution.attempt || 1 }}</div>
            </div>
          </div>
        </div>
      </el-card>

      <!-- Command card -->
      <el-card class="command-card">
        <template #header>
          <div class="card-header">
            <h3>Command</h3>
          </div>
        </template>

        <div class="command-line code-block">
          {{ execution.command }} {{ execution.args ? execution.args.join(' ') : '' }}
        </div>

        <div v-if="execution.work_dir" class="work-dir">
          <span class="work-dir-label">Working Directory:</span>
          <span class="code-text">{{ execution.work_dir }}</span>
        </div>
      </el-card>

      <!-- Log viewer -->
      <div class="log-viewer-container">
        <template v-if="streamMode && execution.status === 'running'">
          <LogStream
            :executionId="executionId"
            @connected="handleStreamConnected"
            @disconnected="handleStreamDisconnected"
            @error="handleStreamError"
          />
        </template>
        <template v-else>
          <LogViewer
            :executionId="executionId"
            :autoRefresh="execution.status === 'running' && autoRefresh"
            :refreshInterval="5000"
          />
        </template>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';
import { useStore } from 'vuex';
import { useRouter, useRoute } from 'vue-router';
import LogStream from '@/components/logs/LogStream.vue';
import LogViewer from '@/components/logs/LogViewer.vue';
import StatusBadge from '@/components/common/StatusBadge.vue';
import LoadingSpinner from '@/components/common/LoadingSpinner.vue';

export default {
  name: 'ExecutionLogView',
  components: {
    LogStream,
    LogViewer,
    StatusBadge,
    LoadingSpinner
  },
  setup() {
    const store = useStore();
    const router = useRouter();
    const route = useRoute();

    // State variables
    const executionId = ref(route.params.id);
    const streamMode = ref(false);
    const autoRefresh = ref(true);
    const jobName = ref('');
    const refreshTimer = ref(null);

    // Computed properties
    const execution = computed(() => store.getters['logs/currentExecution']);
    const logEntries = computed(() => store.getters['logs/executionLogEntries']);
    const isLoading = computed(() => store.getters['logs/isLoadingExecution']);
    const error = computed(() => store.getters['logs/error']);
    const isStreaming = computed(() => store.getters['logs/isStreaming']);

    // Load execution log data
    const loadExecutionLog = async () => {
      try {
        await store.dispatch('logs/fetchExecutionLog', executionId.value);
        await loadJobInfo();
      } catch (err) {
        console.error('Failed to load execution log:', err);
      }
    };

    // Load job information for this execution
    const loadJobInfo = async () => {
      if (execution.value && execution.value.job_id) {
        try {
          const job = await store.dispatch('jobs/fetchJob', execution.value.job_id);
          if (job) {
            jobName.value = job.name;
          }
        } catch (err) {
          console.error('Failed to load job information:', err);
          jobName.value = 'Unknown Job';
        }
      }
    };

    // Start streaming logs for a running execution
    const startLogStreaming = async () => {
      if (streamMode.value && execution.value && execution.value.status === 'running') {
        try {
          await store.dispatch('logs/startLogStream', executionId.value);
        } catch (err) {
          console.error('Failed to start log streaming:', err);
          streamMode.value = false;
        }
      }
    };

    // Stop streaming logs
    const stopLogStreaming = () => {
      store.dispatch('logs/stopLogStream');
    };

    // Auto-refresh for non-streaming mode
    const setupAutoRefresh = () => {
      clearAutoRefresh();

      if (autoRefresh.value && !streamMode.value && execution.value && execution.value.status === 'running') {
        refreshTimer.value = setInterval(() => {
          refreshLogs();
        }, 5000); // Refresh every 5 seconds
      }
    };

    // Clear auto-refresh timer
    const clearAutoRefresh = () => {
      if (refreshTimer.value) {
        clearInterval(refreshTimer.value);
        refreshTimer.value = null;
      }
    };

    // Handle stream mode change
    const handleStreamModeChange = (newMode) => {
      if (newMode) {
        startLogStreaming();
        clearAutoRefresh();
      } else {
        stopLogStreaming();
        setupAutoRefresh();
      }
    };

    // Format date time
    const formatDateTime = (dateString) => {
      if (!dateString) return 'N/A';

      try {
        const date = new Date(dateString);
        return date.toLocaleString();
      } catch (e) {
        return dateString;
      }
    };

    // Format duration between two times
    const formatDuration = (startTime, endTime) => {
      if (!startTime) return 'N/A';

      const start = new Date(startTime);
      const end = endTime ? new Date(endTime) : new Date();
      const durationMs = end - start;

      if (durationMs < 0) return 'Invalid duration';

      // Format the duration
      if (durationMs < 1000) {
        return `${durationMs}ms`;
      } else if (durationMs < 60000) {
        return `${Math.floor(durationMs / 1000)}s`;
      } else if (durationMs < 3600000) {
        const minutes = Math.floor(durationMs / 60000);
        const seconds = Math.floor((durationMs % 60000) / 1000);
        return `${minutes}m ${seconds}s`;
      } else {
        const hours = Math.floor(durationMs / 3600000);
        const minutes = Math.floor((durationMs % 3600000) / 60000);
        return `${hours}h ${minutes}m`;
      }
    };

    // Navigation methods
    const goToJobDetails = () => {
      if (execution.value && execution.value.job_id) {
        router.push(`/jobs/${execution.value.job_id}`);
      }
    };

    const goToLogsList = () => {
      router.push('/logs');
    };

    // Refresh logs manually
    const refreshLogs = () => {
      loadExecutionLog();
    };

    // Clear error message
    const clearError = () => {
      store.commit('logs/SET_ERROR', null);
    };

    // Stream event handlers
    const handleStreamConnected = () => {
      console.log('Log stream connected');
    };

    const handleStreamDisconnected = () => {
      console.log('Log stream disconnected');
    };

    const handleStreamError = (error) => {
      console.error('Log stream error:', error);
      streamMode.value = false;
      setupAutoRefresh();
    };

    // Watch for route changes to reload data
    watch(() => route.params.id, (newId) => {
      if (newId && newId !== executionId.value) {
        executionId.value = newId;
        loadExecutionLog();

        if (streamMode.value) {
          stopLogStreaming();
          startLogStreaming();
        }
      }
    });

    // Watch for execution status changes to update streaming mode
    watch(() => execution.value?.status, (newStatus) => {
      if (newStatus && newStatus !== 'running') {
        // If execution is no longer running, disable streaming
        if (streamMode.value) {
          streamMode.value = false;
          stopLogStreaming();
        }
      }

      // Update auto-refresh based on status
      setupAutoRefresh();
    });

    // Lifecycle hooks
    onMounted(() => {
      loadExecutionLog();

      // Auto-enable stream mode for running executions
      if (execution.value && execution.value.status === 'running') {
        streamMode.value = true;
        startLogStreaming();
      } else {
        setupAutoRefresh();
      }
    });

    onBeforeUnmount(() => {
      stopLogStreaming();
      clearAutoRefresh();

      // Clean up execution data
      store.dispatch('logs/clearCurrentExecution');
    });

    return {
      executionId,
      execution,
      jobName,
      logEntries,
      isLoading,
      error,
      isStreaming,
      streamMode,
      autoRefresh,
      loadExecutionLog,
      refreshLogs,
      startLogStreaming,
      stopLogStreaming,
      formatDateTime,
      formatDuration,
      goToJobDetails,
      goToLogsList,
      clearError,
      handleStreamModeChange,
      handleStreamConnected,
      handleStreamDisconnected,
      handleStreamError
    };
  }
};
</script>

<style scoped>
.execution-log-view {
  display: flex;
  flex-direction: column;
  gap: 20px;
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

.not-found {
  text-align: center;
  padding: 40px;
  background-color: white;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.execution-details {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.summary-card, .command-card {
  margin-bottom: 0;
}

.execution-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
}

.summary-column {
  flex: 1;
  min-width: 200px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.summary-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.summary-label {
  font-size: 12px;
  color: #909399;
}

.summary-value {
  font-size: 14px;
  font-weight: 500;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
}

.command-line {
  font-family: monospace;
  white-space: pre-wrap;
  word-break: break-all;
}

.work-dir {
  margin-top: 12px;
  font-size: 13px;
  color: #606266;
}

.work-dir-label {
  margin-right: 8px;
}

.code-block {
  font-family: monospace;
  background-color: #f8f8f8;
  padding: 12px;
  border-radius: 4px;
  overflow-x: auto;
}

.code-text {
  font-family: monospace;
  background-color: #f8f8f8;
  padding: 2px 5px;
  border-radius: 3px;
}

.log-viewer-container {
  background-color: white;
  border-radius: 4px;
  padding: 20px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.exit-code-success {
  color: #52c41a;
}

.exit-code-failed {
  color: #f56c6c;
}
</style>