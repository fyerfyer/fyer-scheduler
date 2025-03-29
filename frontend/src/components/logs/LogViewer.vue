<template>
  <div class="log-viewer">
    <!-- Log filters -->
    <LogFilter
      :initial-time-range="timeRange"
      :initial-level="logLevel"
      :initial-search-text="searchText"
      :is-filtering="isLoading"
      @update:time-range="updateTimeRange"
      @update:level="updateLogLevel"
      @update:search-text="updateSearchText"
      @filter="applyFilters"
      @reset="resetFilters"
    />

    <!-- Log statistics -->
    <div class="log-stats" v-if="hasLogs">
      <div class="stats-item">
        <span class="stats-label">Total logs:</span>
        <span class="stats-value">{{ totalLogs }}</span>
      </div>

      <div class="stats-item" v-for="(count, level) in logLevelCounts" :key="level">
        <span class="stats-label">{{ level }}:</span>
        <span class="stats-value" :class="`level-${level.toLowerCase()}`">{{ count }}</span>
      </div>

      <div class="stats-item">
        <span class="stats-label">Showing:</span>
        <span class="stats-value">{{ filteredLogs.length }} entries</span>
      </div>

      <div class="stats-actions">
        <el-button
          size="small"
          type="primary"
          plain
          icon="el-icon-refresh"
          @click="refreshLogs"
          :loading="isLoading"
        >
          Refresh
        </el-button>

        <el-tooltip content="Export logs as JSON" placement="top">
          <el-button
            size="small"
            type="default"
            plain
            icon="el-icon-download"
            @click="exportLogs"
            :disabled="!hasLogs"
          />
        </el-tooltip>

        <el-tooltip content="Auto-scroll to bottom" placement="top">
          <el-button
            size="small"
            type="default"
            plain
            icon="el-icon-bottom"
            :class="{ 'active-control': autoScroll }"
            @click="toggleAutoScroll"
            :disabled="!hasLogs"
          />
        </el-tooltip>

        <el-tooltip content="Toggle timestamp display" placement="top">
          <el-button
            size="small"
            type="default"
            plain
            icon="el-icon-time"
            :class="{ 'active-control': showTimestamps }"
            @click="toggleTimestamps"
            :disabled="!hasLogs"
          />
        </el-tooltip>
      </div>
    </div>

    <!-- Loading state -->
    <LoadingSpinner v-if="isLoading" text="Loading logs..." />

    <!-- Empty state -->
    <div v-else-if="!hasLogs" class="log-empty-state">
      <i class="el-icon-document"></i>
      <p>No logs available</p>
      <p class="empty-hint">Logs will appear here when jobs are executed.</p>
    </div>

    <!-- Logs display -->
    <div v-else ref="logContainer" class="log-container">
      <div
        v-for="(log, index) in filteredLogs"
        :key="index"
        class="log-entry"
        :class="`log-level-${log.level.toLowerCase()}`"
      >
        <div class="log-header">
          <span v-if="showTimestamps" class="log-timestamp">
            {{ formatTimestamp(log.timestamp) }}
          </span>
          <span class="log-level">{{ log.level }}</span>
        </div>
        <div class="log-message" v-html="highlightSearchTerm(log.message)"></div>
        <div v-if="log.data && showLogDetails" class="log-data">
          <pre>{{ JSON.stringify(log.data, null, 2) }}</pre>
        </div>
      </div>

      <!-- Show streaming indicator if streaming -->
      <div v-if="isStreaming" class="streaming-indicator">
        <div class="streaming-dots">
          <span></span>
          <span></span>
          <span></span>
        </div>
        <span>Streaming logs...</span>
      </div>
    </div>

    <!-- Pagination if not streaming -->
    <div v-if="!isStreaming && totalPages > 1" class="log-pagination">
      <el-pagination
        layout="prev, pager, next"
        :total="totalLogs"
        :page-size="pageSize"
        :current-page="currentPage"
        @current-change="handlePageChange"
      />
    </div>
  </div>
</template>

<script>
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue';
import { useStore } from 'vuex';
import LogFilter from './LogFilter.vue';
import LoadingSpinner from '@/components/common/LoadingSpinner.vue';

export default {
  name: 'LogViewer',
  components: {
    LogFilter,
    LoadingSpinner
  },
  props: {
    executionId: {
      type: String,
      default: ''
    },
    jobId: {
      type: String,
      default: ''
    },
    autoRefresh: {
      type: Boolean,
      default: false
    },
    refreshInterval: {
      type: Number,
      default: 5000 // Default refresh every 5 seconds
    }
  },
  setup(props) {
    const store = useStore();
    const logContainer = ref(null);

    // Local state
    const timeRange = ref([]);
    const logLevel = ref('');
    const searchText = ref('');
    const currentPage = ref(1);
    const pageSize = ref(100);
    const autoScroll = ref(true);
    const showTimestamps = ref(true);
    const showLogDetails = ref(false);
    const refreshTimer = ref(null);

    // Computed properties
    const logs = computed(() => {
      if (props.executionId) {
        return store.getters['logs/executionLogEntries'];
      } else if (props.jobId) {
        return store.getters['logs/allJobLogs'];
      }
      return [];
    });

    const filteredLogs = computed(() => {
      let result = [...logs.value];

      // Apply level filter
      if (logLevel.value) {
        result = result.filter(log =>
          log.level.toUpperCase() === logLevel.value.toUpperCase()
        );
      }

      // Apply search filter
      if (searchText.value) {
        const searchTermLower = searchText.value.toLowerCase();
        result = result.filter(log =>
          log.message.toLowerCase().includes(searchTermLower)
        );
      }

      return result;
    });

    const hasLogs = computed(() => logs.value.length > 0);

    const totalLogs = computed(() => logs.value.length);

    const totalPages = computed(() =>
      Math.ceil(filteredLogs.value.length / pageSize.value)
    );

    const isLoading = computed(() => {
      if (props.executionId) {
        return store.getters['logs/isLoadingExecution'];
      } else if (props.jobId) {
        return store.getters['logs/isLoading'];
      }
      return false;
    });

    const isStreaming = computed(() =>
      store.getters['logs/isStreaming'] &&
      store.getters['logs/streamingStatus'] === 'connected'
    );

    const logLevelCounts = computed(() => {
      const counts = {
        INFO: 0,
        WARN: 0,
        ERROR: 0,
        DEBUG: 0
      };

      logs.value.forEach(log => {
        const level = log.level.toUpperCase();
        if (counts[level] !== undefined) {
          counts[level]++;
        }
      });

      return counts;
    });

    // Methods
    const loadLogs = async () => {
      try {
        if (props.executionId) {
          await store.dispatch('logs/fetchExecutionLog', props.executionId);
        } else if (props.jobId) {
          await store.dispatch('logs/fetchJobLogs', {
            jobId: props.jobId,
            page: currentPage.value,
            pageSize: pageSize.value,
            timeRange: timeRange.value.length ? {
              startTime: timeRange.value[0],
              endTime: timeRange.value[1]
            } : null
          });
        }
      } catch (error) {
        console.error('Failed to load logs:', error);
      }
    };

    const startStreaming = async () => {
      if (props.executionId) {
        try {
          await store.dispatch('logs/startLogStream', props.executionId);
        } catch (error) {
          console.error('Failed to start log streaming:', error);
        }
      }
    };

    const stopStreaming = () => {
      store.dispatch('logs/stopLogStream');
    };

    const refreshLogs = () => {
      loadLogs();
    };

    const exportLogs = () => {
      if (!hasLogs.value) return;

      const dataStr = JSON.stringify(logs.value, null, 2);
      const dataUri = 'data:application/json;charset=utf-8,' + encodeURIComponent(dataStr);

      const exportFileDefaultName = props.executionId
        ? `execution_${props.executionId}_logs.json`
        : `job_${props.jobId}_logs.json`;

      const linkElement = document.createElement('a');
      linkElement.setAttribute('href', dataUri);
      linkElement.setAttribute('download', exportFileDefaultName);
      linkElement.click();
    };

    const toggleAutoScroll = () => {
      autoScroll.value = !autoScroll.value;
    };

    const toggleTimestamps = () => {
      showTimestamps.value = !showTimestamps.value;
    };

    const handlePageChange = (page) => {
      currentPage.value = page;
      if (!isStreaming.value) {
        loadLogs();
      }
    };

    const updateTimeRange = (range) => {
      timeRange.value = range;
    };

    const updateLogLevel = (level) => {
      logLevel.value = level;
    };

    const updateSearchText = (text) => {
      searchText.value = text;
    };

    const applyFilters = () => {
      currentPage.value = 1;
      loadLogs();
    };

    const resetFilters = () => {
      timeRange.value = [];
      logLevel.value = '';
      searchText.value = '';
      currentPage.value = 1;
      loadLogs();
    };

    const scrollToBottom = () => {
      if (autoScroll.value && logContainer.value) {
        nextTick(() => {
          logContainer.value.scrollTop = logContainer.value.scrollHeight;
        });
      }
    };

    const setupAutoRefresh = () => {
      if (props.autoRefresh && !isStreaming.value) {
        refreshTimer.value = setInterval(() => {
          refreshLogs();
        }, props.refreshInterval);
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
        return date.toLocaleTimeString() + '.' + String(date.getMilliseconds()).padStart(3, '0');
      } catch (e) {
        return timestamp;
      }
    };

    const highlightSearchTerm = (text) => {
      if (!searchText.value || !text) return text;

      const searchTermLower = searchText.value.toLowerCase();
      const textLower = text.toLowerCase();

      let result = '';
      let lastIndex = 0;
      let index = textLower.indexOf(searchTermLower);

      while (index !== -1) {
        result += text.substring(lastIndex, index);
        result += `<span class="highlight">${text.substring(index, index + searchText.value.length)}</span>`;
        lastIndex = index + searchText.value.length;
        index = textLower.indexOf(searchTermLower, lastIndex);
      }

      result += text.substring(lastIndex);
      return result;
    };

    // Watchers
    watch(logs, () => {
      scrollToBottom();
    }, { deep: true });

    watch(() => props.executionId, (newVal) => {
      if (newVal) {
        loadLogs();
        if (props.autoRefresh) {
          startStreaming();
        }
      }
    });

    watch(() => props.jobId, (newVal) => {
      if (newVal) {
        loadLogs();
        setupAutoRefresh();
      }
    });

    watch(() => props.autoRefresh, (newVal) => {
      if (newVal) {
        if (props.executionId) {
          startStreaming();
        } else {
          setupAutoRefresh();
        }
      } else {
        if (props.executionId) {
          stopStreaming();
        }
        clearAutoRefresh();
      }
    });

    // Lifecycle hooks
    onMounted(() => {
      if (props.executionId) {
        loadLogs();
        if (props.autoRefresh) {
          startStreaming();
        }
      } else if (props.jobId) {
        loadLogs();
        if (props.autoRefresh) {
          setupAutoRefresh();
        }
      }
    });

    onBeforeUnmount(() => {
      stopStreaming();
      clearAutoRefresh();
    });

    return {
      logContainer,
      timeRange,
      logLevel,
      searchText,
      currentPage,
      pageSize,
      autoScroll,
      showTimestamps,
      showLogDetails,
      logs,
      filteredLogs,
      hasLogs,
      totalLogs,
      totalPages,
      isLoading,
      isStreaming,
      logLevelCounts,
      loadLogs,
      refreshLogs,
      exportLogs,
      toggleAutoScroll,
      toggleTimestamps,
      handlePageChange,
      updateTimeRange,
      updateLogLevel,
      updateSearchText,
      applyFilters,
      resetFilters,
      formatTimestamp,
      highlightSearchTerm
    };
  }
};
</script>

<style scoped>
.log-viewer {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.log-stats {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  background-color: white;
  border-radius: 4px;
  padding: 12px 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.stats-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.stats-label {
  font-size: 13px;
  color: #606266;
}

.stats-value {
  font-size: 13px;
  font-weight: 500;
}

.level-info {
  color: #409EFF;
}

.level-warn {
  color: #E6A23C;
}

.level-error {
  color: #F56C6C;
}

.level-debug {
  color: #909399;
}

.stats-actions {
  margin-left: auto;
  display: flex;
  gap: 8px;
}

.active-control {
  background-color: #ecf5ff;
  color: #409EFF;
  border-color: #b3d8ff;
}

.log-container {
  background-color: #1e1e1e;
  border-radius: 4px;
  padding: 16px;
  max-height: 600px;
  overflow-y: auto;
  color: #d4d4d4;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.5;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.log-entry {
  padding: 4px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.log-entry:last-child {
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

.log-data {
  margin-top: 4px;
  padding: 4px;
  background-color: rgba(255, 255, 255, 0.05);
  border-radius: 3px;
  font-size: 12px;
}

.log-data pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
}

.log-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background-color: white;
  border-radius: 4px;
  padding: 40px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  color: #909399;
}

.log-empty-state i {
  font-size: 48px;
  margin-bottom: 16px;
}

.log-empty-state p {
  margin-bottom: 8px;
}

.empty-hint {
  font-size: 13px;
  color: #c0c4cc;
}

.log-pagination {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}

.streaming-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding: 8px;
  background-color: rgba(64, 158, 255, 0.1);
  border-radius: 4px;
  color: #409EFF;
}

.streaming-dots {
  display: flex;
  align-items: center;
  gap: 4px;
}

.streaming-dots span {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: #409EFF;
  animation: pulse 1.4s infinite ease-in-out;
}

.streaming-dots span:nth-child(2) {
  animation-delay: 0.2s;
}

.streaming-dots span:nth-child(3) {
  animation-delay: 0.4s;
}

@keyframes pulse {
  0%, 80%, 100% {
    transform: scale(0.6);
    opacity: 0.4;
  }
  40% {
    transform: scale(1);
    opacity: 1;
  }
}

.highlight {
  background-color: rgba(255, 255, 0, 0.3);
  color: #ffffff;
  border-radius: 2px;
  padding: 0 2px;
}
</style>