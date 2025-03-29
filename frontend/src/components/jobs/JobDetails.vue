<template>
  <div class="job-details">
    <!-- Basic Info Section -->
    <el-card class="detail-section">
      <template #header>
        <div class="section-header">
          <h3>Basic Information</h3>
        </div>
      </template>

      <el-descriptions :column="2" border>
        <el-descriptions-item label="Job ID">
          <el-tag size="small">{{ job.id }}</el-tag>
        </el-descriptions-item>

        <el-descriptions-item label="Status">
          <StatusBadge :status="jobStatus" />
          {{ job.enabled ? '(Enabled)' : '(Disabled)' }}
        </el-descriptions-item>

        <el-descriptions-item label="Name">
          {{ job.name }}
        </el-descriptions-item>

        <el-descriptions-item label="Created">
          {{ formatDate(job.create_time) }}
        </el-descriptions-item>

        <el-descriptions-item label="Description" :span="2">
          {{ job.description || 'No description provided' }}
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- Command Section -->
    <el-card class="detail-section">
      <template #header>
        <div class="section-header">
          <h3>Command</h3>
        </div>
      </template>

      <el-descriptions :column="1" border>
        <el-descriptions-item label="Command">
          <div class="code-block">{{ job.command }}</div>
        </el-descriptions-item>

        <el-descriptions-item v-if="job.args && job.args.length > 0" label="Arguments">
          <div class="code-block">
            <div v-for="(arg, index) in job.args" :key="index" class="arg-line">
              {{ index + 1 }}. {{ arg }}
            </div>
          </div>
        </el-descriptions-item>

        <el-descriptions-item label="Working Directory">
          <div class="code-block">{{ job.work_dir || 'Default' }}</div>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- Schedule Section -->
    <el-card class="detail-section">
      <template #header>
        <div class="section-header">
          <h3>Schedule</h3>
        </div>
      </template>

      <el-descriptions :column="2" border>
        <el-descriptions-item label="Cron Expression">
          <span v-if="job.cron_expr" class="code-text">{{ job.cron_expr }}</span>
          <span v-else class="text-muted">Manual execution only</span>
        </el-descriptions-item>

        <el-descriptions-item label="Next Run">
          {{ formatDate(job.next_run_time) }}
        </el-descriptions-item>

        <el-descriptions-item label="Last Run">
          {{ formatDate(job.last_run_time) }}
        </el-descriptions-item>

        <el-descriptions-item label="Last Status">
          <StatusBadge v-if="job.last_status" :status="job.last_status" />
          <span v-else class="text-muted">Never executed</span>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- Environment Variables Section -->
    <el-card v-if="hasEnvVars" class="detail-section">
      <template #header>
        <div class="section-header">
          <h3>Environment Variables</h3>
        </div>
      </template>

      <el-table :data="envVarsArray" style="width: 100%" border stripe>
        <el-table-column prop="key" label="Key" width="180" />
        <el-table-column prop="value" label="Value" />
      </el-table>
    </el-card>

    <!-- Advanced Settings Section -->
    <el-card class="detail-section">
      <template #header>
        <div class="section-header">
          <h3>Advanced Settings</h3>
        </div>
      </template>

      <el-descriptions :column="2" border>
        <el-descriptions-item label="Timeout">
          {{ job.timeout ? `${job.timeout} seconds` : 'No timeout' }}
        </el-descriptions-item>

        <el-descriptions-item label="Max Retry">
          {{ job.max_retry || 'No retries' }}
        </el-descriptions-item>

        <el-descriptions-item label="Retry Delay">
          {{ job.retry_delay ? `${job.retry_delay} seconds` : 'Default' }}
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- Execution History Section -->
    <el-card class="detail-section">
      <template #header>
        <div class="section-header">
          <h3>Recent Executions</h3>
          <el-button size="small" type="primary" @click="$emit('view-logs')">
            View All Logs
          </el-button>
        </div>
      </template>

      <div v-if="!executions || executions.length === 0" class="no-data">
        <i class="el-icon-document"></i>
        <p>No execution history available</p>
      </div>

      <el-timeline v-else>
        <el-timeline-item
          v-for="execution in executions"
          :key="execution.id"
          :timestamp="formatDate(execution.start_time)"
          :type="getTimelineItemType(execution.status)"
        >
          <div class="execution-item">
            <div class="execution-header">
              <StatusBadge :status="execution.status" />
              <span class="execution-id">{{ execution.id }}</span>
            </div>

            <div class="execution-info">
              <div v-if="execution.end_time">
                Duration: {{ calculateDuration(execution.start_time, execution.end_time) }}
              </div>
              <div v-if="execution.exit_code !== undefined">
                Exit Code: {{ execution.exit_code }}
              </div>
            </div>

            <div class="execution-actions">
              <el-button
                size="small"
                type="text"
                @click="$emit('view-execution', execution.id)"
              >
                View Details
              </el-button>
            </div>
          </div>
        </el-timeline-item>
      </el-timeline>
    </el-card>
  </div>
</template>

<script>
import { computed } from 'vue';
import StatusBadge from '@/components/common/StatusBadge.vue';

export default {
  name: 'JobDetails',
  components: {
    StatusBadge
  },
  props: {
    job: {
      type: Object,
      required: true
    },
    executions: {
      type: Array,
      default: () => []
    }
  },
  emits: ['view-logs', 'view-execution'],
  setup(props) {
    // Computed properties
    const jobStatus = computed(() => {
      if (!props.job.enabled) {
        return 'disabled';
      }
      return props.job.status || 'pending';
    });

    const hasEnvVars = computed(() => {
      return props.job.env && Object.keys(props.job.env).length > 0;
    });

    const envVarsArray = computed(() => {
      if (!props.job.env) return [];

      return Object.entries(props.job.env).map(([key, value]) => ({
        key,
        value
      }));
    });

    // Methods
    const formatDate = (dateString) => {
      if (!dateString) return 'N/A';

      const date = new Date(dateString);
      return date.toLocaleString();
    };

    const calculateDuration = (startTime, endTime) => {
      if (!startTime || !endTime) return 'N/A';

      const start = new Date(startTime);
      const end = new Date(endTime);
      const durationMs = end - start;

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

    const getTimelineItemType = (status) => {
      switch (status) {
        case 'completed':
        case 'success':
          return 'success';
        case 'failed':
        case 'error':
          return 'danger';
        case 'running':
          return 'primary';
        default:
          return 'info';
      }
    };

    return {
      jobStatus,
      hasEnvVars,
      envVarsArray,
      formatDate,
      calculateDuration,
      getTimelineItemType
    };
  }
};
</script>

<style scoped>
.job-details {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.detail-section {
  margin-bottom: 0;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.section-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
}

.code-block {
  font-family: monospace;
  background-color: #f8f8f8;
  padding: 8px 12px;
  border-radius: 4px;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.code-text {
  font-family: monospace;
  background-color: #f8f8f8;
  padding: 2px 5px;
  border-radius: 3px;
}

.text-muted {
  color: #909399;
  font-style: italic;
}

.arg-line {
  margin-bottom: 4px;
}

.no-data {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 24px;
  color: #909399;
}

.no-data i {
  font-size: 32px;
  margin-bottom: 12px;
}

.execution-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.execution-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.execution-id {
  font-family: monospace;
  color: #606266;
}

.execution-info {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  color: #606266;
  font-size: 13px;
}

.execution-actions {
  margin-top: 4px;
}
</style>