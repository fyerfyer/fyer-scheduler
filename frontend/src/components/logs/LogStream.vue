<template>
  <div class="log-stream">
    <!-- Stream controls -->
    <div class="stream-controls">
      <div class="status-indicator">
        <span class="status-dot" :class="connectionStatusClass"></span>
        <span class="status-text">{{ connectionStatusText }}</span>
      </div>

      <div class="filter-controls">
        <el-select
          v-model="logLevel"
          placeholder="Log Level"
          size="small"
          clearable
          @change="filterLogs"
        >
          <el-option
            v-for="level in logLevels"
            :key="level.value"
            :label="level.label"
            :value="level.value"
          >
            <div class="level-option">
              <span class="log-level-indicator" :class="`level-${level.value.toLowerCase()}`"></span>
              <span>{{ level.label }}</span>
            </div>
          </el-option>
        </el-select>

        <el-input
          v-model="searchText"
          placeholder="Search logs"
          size="small"
          clearable
          @input="debouncedFilter"
        >
          <template #prefix>
            <i class="el-icon-search"></i>
          </template>
        </el-input>
      </div>

      <div class="action-controls">
        <el-tooltip content="Auto-scroll to bottom" placement="top">
          <el-button
            size="small"
            type="default"
            :class="{ 'active-control': autoScroll }"
            @click="toggleAutoScroll"
            icon="el-icon-bottom"
          />
        </el-tooltip>

        <el-tooltip content="Clear log display" placement="top">
          <el-button
            size="small"
            type="default"
            @click="clearLogs"
            icon="el-icon-delete"
          />
        </el-tooltip>

        <el-tooltip content="Copy logs to clipboard" placement="top">
          <el-button
            size="small"
            type="default"
            @click="copyLogs"
            icon="el-icon-document-copy"
          />
        </el-tooltip>

        <el-tooltip :content="isPaused ? 'Resume streaming' : 'Pause streaming'" placement="top">
          <el-button
            size="small"
            type="default"
            :class="{ 'active-control': isPaused }"
            @click="togglePause"
            :icon="isPaused ? 'el-icon-video-play' : 'el-icon-video-pause'"
          />
        </el-tooltip>
      </div>
    </div>

    <!-- Log viewer -->
    <div
      ref="logContainer"
      class="log-container"
      @scroll="handleScroll"
    >
      <div v-if="filteredLogs.length === 0 && !isStreaming" class="empty-logs">
        <p>No logs available</p>
        <p v-if="!isConnected" class="connect-hint">
          Attempting to connect to log stream...
        </p>
      </div>

      <div
        v-for="(log, index) in filteredLogs"
        :key="index"
        class="log-entry"
        :class="`log-level-${log.level.toLowerCase()}`"
      >
        <div class="log-header">
          <span class="log-timestamp">{{ formatTimestamp(log.timestamp) }}</span>
          <span class="log-level-badge">{{ log.level }}</span>
        </div>
        <div class="log-message" v-html="highlightSearchTerm(log.message)"></div>
      </div>

      <!-- Show loading/streaming indicator -->
      <div v-if="isStreaming && !isPaused" class="streaming-indicator">
        <div class="streaming-dots">
          <span></span>
          <span></span>
          <span></span>
        </div>
        <span>Streaming logs...</span>
      </div>
    </div>

    <!-- Log stats -->
    <div class="log-stats" v-if="logs.length > 0">
      <div class="stats-item">
        <span class="stats-label">Total:</span>
        <span class="stats-value">{{ logs.length }}</span>
      </div>
      <div class="stats-item">
        <span class="stats-label">Filtered:</span>
        <span class="stats-value">{{ filteredLogs.length }}</span>
      </div>
      <div class="stats-item" v-for="(count, level) in logLevelCounts" :key="level">
        <span class="stats-label">{{ level }}:</span>
        <span class="stats-value" :class="`level-${level.toLowerCase()}`">{{ count }}</span>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue';

export default {
  name: 'LogStream',
  props: {
    // Execution ID to stream logs for
    executionId: {
      type: String,
      required: true
    },
    // Maximum number of logs to keep in memory
    maxLogs: {
      type: Number,
      default: 1000
    },
    // Whether to auto-scroll to bottom by default
    defaultAutoScroll: {
      type: Boolean,
      default: true
    },
    // Stream URL - if not provided, will be constructed from execution ID
    streamUrl: {
      type: String,
      default: ''
    }
  },
  emits: ['connected', 'disconnected', 'error'],
  setup(props, { emit }) {
    const logContainer = ref(null);
    const eventSource = ref(null);
    const logs = ref([]);
    const logLevel = ref('');
    const searchText = ref('');
    const autoScroll = ref(props.defaultAutoScroll);
    const isPaused = ref(false);
    const isConnected = ref(false);
    const connectionStatus = ref('connecting');
    let filterTimeout = null;

    // Define log levels
    const logLevels = [
      { value: 'INFO', label: 'Information' },
      { value: 'WARN', label: 'Warning' },
      { value: 'ERROR', label: 'Error' },
      { value: 'DEBUG', label: 'Debug' }
    ];

    // Connection status
    const connectionStatusClass = computed(() => {
      switch (connectionStatus.value) {
        case 'connected':
          return 'status-connected';
        case 'connecting':
          return 'status-connecting';
        case 'disconnected':
          return 'status-disconnected';
        case 'error':
          return 'status-error';
        default:
          return '';
      }
    });

    const connectionStatusText = computed(() => {
      switch (connectionStatus.value) {
        case 'connected':
          return 'Connected';
        case 'connecting':
          return 'Connecting...';
        case 'disconnected':
          return 'Disconnected';
        case 'error':
          return 'Connection Error';
        default:
          return 'Unknown';
      }
    });

    // Is actively streaming
    const isStreaming = computed(() => {
      return isConnected.value && !isPaused.value;
    });

    // Filtered logs based on level and search text
    const filteredLogs = computed(() => {
      let result = [...logs.value];

      // Apply level filter if set
      if (logLevel.value) {
        result = result.filter(log =>
          log.level.toUpperCase() === logLevel.value.toUpperCase()
        );
      }

      // Apply search filter if set
      if (searchText.value) {
        const searchTermLower = searchText.value.toLowerCase();
        result = result.filter(log =>
          log.message.toLowerCase().includes(searchTermLower)
        );
      }

      return result;
    });

    // Count logs by level
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

    // Initialize EventSource connection
    const initStream = () => {
      if (eventSource.value) {
        closeStream();
      }

      if (isPaused.value) {
        connectionStatus.value = 'paused';
        return;
      }

      try {
        const url = props.streamUrl ||
          `${process.env.VUE_APP_API_URL || ''}/api/v1/logs/executions/${props.executionId}/stream`;

        connectionStatus.value = 'connecting';
        eventSource.value = new EventSource(url);

        eventSource.value.onopen = () => {
          isConnected.value = true;
          connectionStatus.value = 'connected';
          emit('connected');
        };

        eventSource.value.onmessage = (event) => {
          handleLogMessage(event.data);
        };

        eventSource.value.onerror = (error) => {
          console.error('Log stream error:', error);
          connectionStatus.value = 'error';
          isConnected.value = false;
          emit('error', error);

          // Auto-reconnect after a delay
          setTimeout(() => {
            if (eventSource.value) {
              closeStream();
              initStream();
            }
          }, 3000);
        };
      } catch (error) {
        console.error('Failed to create log stream:', error);
        connectionStatus.value = 'error';
        emit('error', error);
      }
    };

    // Close EventSource connection
    const closeStream = () => {
      if (eventSource.value) {
        eventSource.value.close();
        eventSource.value = null;
      }

      isConnected.value = false;
      connectionStatus.value = 'disconnected';
      emit('disconnected');
    };

    // Handle incoming log message
    const handleLogMessage = (data) => {
      if (isPaused.value) {
        return;
      }

      try {
        // Try to parse as JSON
        let logEntry;
        try {
          const parsed = JSON.parse(data);
          logEntry = {
            id: logs.value.length,
            timestamp: parsed.time || new Date().toISOString(),
            level: parsed.level || 'INFO',
            message: parsed.message || data,
            data: parsed
          };
        } catch (e) {
          // If not JSON, create basic log entry
          logEntry = {
            id: logs.value.length,
            timestamp: new Date().toISOString(),
            level: 'INFO',
            message: data,
            data: null
          };
        }

        // Add to logs array
        logs.value.push(logEntry);

        // Limit the number of logs kept in memory
        if (logs.value.length > props.maxLogs) {
          logs.value = logs.value.slice(-props.maxLogs);
        }

        // Auto-scroll to bottom if enabled
        if (autoScroll.value) {
          scrollToBottom();
        }
      } catch (error) {
        console.error('Error processing log message:', error);
      }
    };

    // Filter logs (debounced)
    const debouncedFilter = () => {
      if (filterTimeout) {
        clearTimeout(filterTimeout);
      }

      filterTimeout = setTimeout(() => {
        filterLogs();
      }, 300);
    };

    // Apply filters
    const filterLogs = () => {
      // No action needed as filtering is done via computed properties
      // but we can scroll to bottom after filtering if needed
      if (autoScroll.value) {
        scrollToBottom();
      }
    };

    // Toggle auto-scroll
    const toggleAutoScroll = () => {
      autoScroll.value = !autoScroll.value;

      if (autoScroll.value) {
        scrollToBottom();
      }
    };

    // Toggle pause/resume
    const togglePause = () => {
      isPaused.value = !isPaused.value;

      if (isPaused.value) {
        // When pausing, we just stop processing new messages but keep the connection
        connectionStatus.value = 'paused';
      } else {
        // When resuming, we start processing messages again
        if (isConnected.value) {
          connectionStatus.value = 'connected';
        } else {
          // If not connected, reconnect
          initStream();
        }
      }
    };

    // Clear displayed logs
    const clearLogs = () => {
      logs.value = [];
    };

    // Copy logs to clipboard
    const copyLogs = () => {
      const logText = filteredLogs.value.map(log =>
        `[${formatTimestamp(log.timestamp)}] [${log.level}] ${log.message}`
      ).join('\n');

      navigator.clipboard.writeText(logText)
        .then(() => {
          // Success message could be shown here
          console.log('Logs copied to clipboard');
        })
        .catch(err => {
          console.error('Failed to copy logs:', err);
        });
    };

    // Format timestamp
    const formatTimestamp = (timestamp) => {
      if (!timestamp) return '';

      try {
        const date = new Date(timestamp);
        return date.toLocaleTimeString() + '.' + String(date.getMilliseconds()).padStart(3, '0');
      } catch (e) {
        return timestamp;
      }
    };

    // Highlight search term in log message
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

    // Scroll to bottom of log container
    const scrollToBottom = () => {
      nextTick(() => {
        if (logContainer.value) {
          logContainer.value.scrollTop = logContainer.value.scrollHeight;
        }
      });
    };

    // Handle scroll to detect if user has manually scrolled up
    const handleScroll = () => {
      if (!logContainer.value) return;

      const { scrollTop, scrollHeight, clientHeight } = logContainer.value;
      const isAtBottom = scrollTop + clientHeight >= scrollHeight - 50;

      // If user scrolls up, disable auto-scroll
      if (!isAtBottom && autoScroll.value) {
        autoScroll.value = false;
      }
    };

    // Watch for changes to execution ID
    watch(() => props.executionId, (newId) => {
      if (newId) {
        clearLogs();
        closeStream();
        initStream();
      } else {
        closeStream();
      }
    });

    // Lifecycle hooks
    onMounted(() => {
      if (props.executionId) {
        initStream();
      }
    });

    onBeforeUnmount(() => {
      closeStream();
      if (filterTimeout) {
        clearTimeout(filterTimeout);
      }
    });

    return {
      logContainer,
      logs,
      filteredLogs,
      logLevel,
      logLevels,
      searchText,
      autoScroll,
      isPaused,
      isConnected,
      isStreaming,
      connectionStatus,
      connectionStatusClass,
      connectionStatusText,
      logLevelCounts,
      toggleAutoScroll,
      togglePause,
      clearLogs,
      copyLogs,
      filterLogs,
      debouncedFilter,
      formatTimestamp,
      handleScroll,
      highlightSearchTerm
    };
  }
};
</script>

<style scoped>
.log-stream {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 400px;
}

.stream-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background-color: white;
  padding: 8px 12px;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.status-connected {
  background-color: #52c41a;
  box-shadow: 0 0 8px rgba(82, 196, 26, 0.6);
  animation: pulse 2s infinite;
}

.status-connecting {
  background-color: #1890ff;
  animation: blink 1s infinite;
}

.status-disconnected {
  background-color: #bfbfbf;
}

.status-error {
  background-color: #f5222d;
}

.status-text {
  font-size: 13px;
  color: #606266;
}

.filter-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.action-controls {
  display: flex;
  align-items: center;
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
  padding: 12px;
  flex: 1;
  overflow-y: auto;
  color: #d4d4d4;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.5;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.log-entry {
  padding: 3px 0;
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

.log-level-badge {
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: bold;
}

.log-level-info .log-level-badge {
  background-color: rgba(64, 158, 255, 0.1);
  color: #409EFF;
}

.log-level-warn .log-level-badge {
  background-color: rgba(230, 162, 60, 0.1);
  color: #E6A23C;
}

.log-level-error .log-level-badge {
  background-color: rgba(245, 108, 108, 0.1);
  color: #F56C6C;
}

.log-level-debug .log-level-badge {
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
  justify-content: center;
  height: 100%;
  color: #909399;
}

.connect-hint {
  margin-top: 8px;
  font-size: 12px;
  color: #6a9955;
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

.log-stats {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  background-color: white;
  border-radius: 4px;
  padding: 8px 12px;
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

.level-option {
  display: flex;
  align-items: center;
}

.log-level-indicator {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 8px;
}

.highlight {
  background-color: rgba(255, 255, 0, 0.3);
  color: #ffffff;
  border-radius: 2px;
  padding: 0 2px;
}

@keyframes pulse {
  0%, 100% {
    opacity: 0.6;
  }
  50% {
    opacity: 1;
  }
}

@keyframes blink {
  0%, 100% {
    opacity: 0.4;
  }
  50% {
    opacity: 1;
  }
}
</style>