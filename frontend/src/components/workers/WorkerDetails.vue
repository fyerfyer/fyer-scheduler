<template>
  <div class="worker-details">
    <!-- Basic Info Section -->
    <el-card class="detail-section">
      <template #header>
        <div class="section-header">
          <h3>Worker Information</h3>
          <div class="worker-status">
            <StatusBadge :status="workerStatus" />
            <span class="status-text">{{ worker.is_active ? (worker.is_healthy ? 'Healthy' : 'Unhealthy') : 'Disabled' }}</span>
          </div>
        </div>
      </template>

      <div class="info-grid">
        <div class="info-item">
          <div class="info-label">Worker ID</div>
          <div class="info-value code-text">{{ worker.id }}</div>
        </div>

        <div class="info-item">
          <div class="info-label">Hostname</div>
          <div class="info-value">{{ worker.hostname || 'N/A' }}</div>
        </div>

        <div class="info-item">
          <div class="info-label">IP Address</div>
          <div class="info-value">{{ worker.address || 'N/A' }}</div>
        </div>

        <div class="info-item">
          <div class="info-label">Uptime</div>
          <div class="info-value">{{ formatUptime(worker.uptime_seconds) }}</div>
        </div>

        <div class="info-item">
          <div class="info-label">First Seen</div>
          <div class="info-value">{{ formatDate(worker.first_seen) }}</div>
        </div>

        <div class="info-item">
          <div class="info-label">Last Heartbeat</div>
          <div class="info-value">{{ formatDate(worker.last_heartbeat) }}</div>
        </div>
      </div>
    </el-card>

    <!-- Resource Usage Section -->
    <el-card class="detail-section" v-if="worker.resources">
      <template #header>
        <div class="section-header">
          <h3>Resource Usage</h3>
          <span class="last-updated">Last updated: {{ formatDate(worker.last_heartbeat) }}</span>
        </div>
      </template>

      <div class="resource-grid">
        <div class="resource-item">
          <div class="resource-header">
            <span class="resource-label">CPU Usage</span>
            <span class="resource-value" :class="getCpuStatusClass(resourceUsage.cpu)">
              {{ resourceUsage.cpu.toFixed(1) }}%
            </span>
          </div>
          <el-progress
            :percentage="resourceUsage.cpu"
            :status="getCpuStatus(resourceUsage.cpu)"
            :stroke-width="10"
          />
          <div class="resource-details" v-if="worker.resources.cpu_details">
            {{ worker.resources.cpu_details }}
          </div>
        </div>

        <div class="resource-item">
          <div class="resource-header">
            <span class="resource-label">Memory Usage</span>
            <span class="resource-value" :class="getMemoryStatusClass(resourceUsage.memory)">
              {{ resourceUsage.memory.toFixed(1) }}%
            </span>
          </div>
          <el-progress
            :percentage="resourceUsage.memory"
            :status="getMemoryStatus(resourceUsage.memory)"
            :stroke-width="10"
          />
          <div class="resource-details" v-if="worker.resources.memory_total">
            {{ formatBytes(worker.resources.memory_used) }} / {{ formatBytes(worker.resources.memory_total) }}
          </div>
        </div>

        <div class="resource-item">
          <div class="resource-header">
            <span class="resource-label">Disk Usage</span>
            <span class="resource-value" :class="getDiskStatusClass(resourceUsage.disk)">
              {{ resourceUsage.disk.toFixed(1) }}%
            </span>
          </div>
          <el-progress
            :percentage="resourceUsage.disk"
            :status="getDiskStatus(resourceUsage.disk)"
            :stroke-width="10"
          />
          <div class="resource-details" v-if="worker.resources.disk_total">
            {{ formatBytes(worker.resources.disk_used) }} / {{ formatBytes(worker.resources.disk_total) }}
          </div>
        </div>
      </div>
    </el-card>

    <!-- Tags/Labels Section -->
    <el-card class="detail-section" v-if="hasTags">
      <template #header>
        <div class="section-header">
          <h3>Tags</h3>
          <el-button
            type="primary"
            size="small"
            plain
            @click="editTags"
            icon="el-icon-edit"
          >
            Edit Tags
          </el-button>
        </div>
      </template>

      <div class="tags-container">
        <el-tag
          v-for="(value, key) in worker.tags"
          :key="key"
          class="tag-item"
        >
          {{ key }}={{ value }}
        </el-tag>

        <div class="no-tags" v-if="!hasTags">
          No tags defined for this worker.
        </div>
      </div>
    </el-card>

    <!-- Running Jobs Section -->
    <el-card class="detail-section">
      <template #header>
        <div class="section-header">
          <h3>Running Jobs</h3>
          <span class="job-count">{{ runningJobs.length }} job(s)</span>
        </div>
      </template>

      <div class="jobs-container">
        <el-table
          v-if="runningJobs.length > 0"
          :data="runningJobs"
          style="width: 100%"
          size="small"
        >
          <el-table-column prop="id" label="Execution ID" width="180">
            <template #default="scope">
              <router-link :to="`/logs/execution/${scope.row.id}`" class="job-link">
                {{ truncateId(scope.row.id) }}
              </router-link>
            </template>
          </el-table-column>
          <el-table-column prop="job_name" label="Job" width="180">
            <template #default="scope">
              <router-link :to="`/jobs/${scope.row.job_id}`" class="job-link">
                {{ scope.row.job_name }}
              </router-link>
            </template>
          </el-table-column>
          <el-table-column prop="start_time" label="Start Time">
            <template #default="scope">
              {{ formatDate(scope.row.start_time) }}
            </template>
          </el-table-column>
          <el-table-column label="Actions" width="120">
            <template #default="scope">
              <el-button
                size="mini"
                type="danger"
                @click="killJob(scope.row.id, scope.row.job_id)"
              >
                Kill
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="no-jobs" v-else>
          <i class="el-icon-tickets"></i>
          <p>No jobs currently running on this worker.</p>
        </div>
      </div>
    </el-card>

    <!-- Health Check Section -->
    <el-card class="detail-section">
      <template #header>
        <div class="section-header">
          <h3>Health Check</h3>
          <el-button
            type="primary"
            size="small"
            @click="checkHealth"
            :loading="isCheckingHealth"
          >
            Check Health
          </el-button>
        </div>
      </template>

      <div class="health-container">
        <div v-if="healthResult" class="health-result">
          <div class="health-status">
            <StatusBadge :status="healthResult.is_healthy ? 'healthy' : 'unhealthy'" />
            <span class="health-text">{{ healthResult.is_healthy ? 'Healthy' : 'Unhealthy' }}</span>
          </div>

          <div class="health-info">
            <div class="health-item">
              <span class="health-label">Check Time:</span>
              <span class="health-value">{{ formatDate(healthResult.check_time) }}</span>
            </div>

            <div class="health-item" v-if="healthResult.message">
              <span class="health-label">Message:</span>
              <span class="health-value">{{ healthResult.message }}</span>
            </div>
          </div>
        </div>

        <div v-else-if="isCheckingHealth" class="health-checking">
          Checking worker health...
        </div>

        <div v-else class="health-no-data">
          <i class="el-icon-warning-outline"></i>
          <p>No recent health check data available.</p>
          <p>Click "Check Health" to perform a health check.</p>
        </div>
      </div>
    </el-card>

    <!-- Edit Tags Dialog -->
    <el-dialog
      title="Edit Worker Tags"
      v-model="tagsDialogVisible"
      width="500px"
    >
      <div class="tags-form">
        <div
          v-for="(tag, index) in tagsArray"
          :key="index"
          class="tag-input-row"
        >
          <el-input
            v-model="tag.key"
            placeholder="Key"
            class="tag-key-input"
          />
          <span class="equals-sign">=</span>
          <el-input
            v-model="tag.value"
            placeholder="Value"
            class="tag-value-input"
          />
          <el-button
            type="danger"
            icon="el-icon-delete"
            circle
            size="mini"
            @click="removeTag(index)"
          />
        </div>

        <div class="add-tag-button">
          <el-button
            type="primary"
            icon="el-icon-plus"
            size="small"
            @click="addTag"
          >
            Add Tag
          </el-button>
        </div>
      </div>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="tagsDialogVisible = false">Cancel</el-button>
          <el-button type="primary" @click="saveTags" :loading="isSavingTags">Save</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { ref, computed, defineComponent } from 'vue';
import { useStore } from 'vuex';
import StatusBadge from '@/components/common/StatusBadge.vue';
import workersService from '@/services/workers.service';

export default defineComponent({
  name: 'WorkerDetails',
  components: {
    StatusBadge
  },
  props: {
    worker: {
      type: Object,
      required: true
    },
    runningJobs: {
      type: Array,
      default: () => []
    },
    isCheckingHealth: {
      type: Boolean,
      default: false
    }
  },
  emits: ['check-health', 'kill-job', 'update-tags'],
  setup(props, { emit }) {
    const store = useStore();

    // State
    const tagsDialogVisible = ref(false);
    const tagsArray = ref([]);
    const isSavingTags = ref(false);

    // Computed
    const workerStatus = computed(() => {
      if (!props.worker.is_active) return 'disabled';
      return props.worker.is_healthy ? 'healthy' : 'unhealthy';
    });

    const resourceUsage = computed(() => {
      return workersService.calculateResourceUsage(props.worker);
    });

    const hasTags = computed(() => {
      return props.worker.tags && Object.keys(props.worker.tags).length > 0;
    });

    const healthResult = computed(() => {
      return store.getters['workers/workerHealth'](props.worker.id);
    });

    // Methods
    const formatUptime = (uptimeSeconds) => {
      return workersService.formatUptime(uptimeSeconds);
    };

    const formatDate = (dateString) => {
      if (!dateString) return 'N/A';

      try {
        const date = new Date(dateString);
        return date.toLocaleString();
      } catch (e) {
        return dateString;
      }
    };

    const formatBytes = (bytes) => {
      if (!bytes || isNaN(bytes)) return 'N/A';

      const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
      if (bytes === 0) return '0 Bytes';

      const i = parseInt(Math.floor(Math.log(bytes) / Math.log(1024)), 10);
      if (i === 0) return `${bytes} ${sizes[i]}`;

      return `${(bytes / (1024 ** i)).toFixed(2)} ${sizes[i]}`;
    };

    const truncateId = (id) => {
      if (!id) return 'N/A';
      if (id.length <= 8) return id;
      return `${id.substring(0, 8)}...`;
    };

    const getCpuStatus = (usage) => {
      if (usage >= 90) return 'exception';
      if (usage >= 70) return 'warning';
      return 'success';
    };

    const getMemoryStatus = (usage) => {
      if (usage >= 90) return 'exception';
      if (usage >= 80) return 'warning';
      return 'success';
    };

    const getDiskStatus = (usage) => {
      if (usage >= 95) return 'exception';
      if (usage >= 85) return 'warning';
      return 'success';
    };

    const getCpuStatusClass = (usage) => {
      if (usage >= 90) return 'status-danger';
      if (usage >= 70) return 'status-warning';
      return 'status-success';
    };

    const getMemoryStatusClass = (usage) => {
      if (usage >= 90) return 'status-danger';
      if (usage >= 80) return 'status-warning';
      return 'status-success';
    };

    const getDiskStatusClass = (usage) => {
      if (usage >= 95) return 'status-danger';
      if (usage >= 85) return 'status-warning';
      return 'status-success';
    };

    const checkHealth = () => {
      emit('check-health');
    };

    const killJob = (executionId, jobId) => {
      emit('kill-job', { executionId, jobId });
    };

    const editTags = () => {
      // Convert object to array for editing
      tagsArray.value = Object.entries(props.worker.tags || {}).map(([key, value]) => ({ key, value }));

      // Add an empty row if no tags
      if (tagsArray.value.length === 0) {
        tagsArray.value.push({ key: '', value: '' });
      }

      tagsDialogVisible.value = true;
    };

    const addTag = () => {
      tagsArray.value.push({ key: '', value: '' });
    };

    const removeTag = (index) => {
      tagsArray.value.splice(index, 1);
    };

    const saveTags = async () => {
      isSavingTags.value = true;

      try {
        // Convert array back to object
        const tagsObject = {};
        tagsArray.value.forEach(tag => {
          if (tag.key && tag.key.trim() !== '') {
            tagsObject[tag.key.trim()] = tag.value.trim();
          }
        });

        emit('update-tags', tagsObject);
        tagsDialogVisible.value = false;
      } catch (error) {
        console.error('Failed to save tags:', error);
      } finally {
        isSavingTags.value = false;
      }
    };

    return {
      tagsDialogVisible,
      tagsArray,
      isSavingTags,
      workerStatus,
      resourceUsage,
      hasTags,
      healthResult,
      formatUptime,
      formatDate,
      formatBytes,
      truncateId,
      getCpuStatus,
      getMemoryStatus,
      getDiskStatus,
      getCpuStatusClass,
      getMemoryStatusClass,
      getDiskStatusClass,
      checkHealth,
      killJob,
      editTags,
      addTag,
      removeTag,
      saveTags
    };
  }
});
</script>

<style scoped>
.worker-details {
  display: flex;
  flex-direction: column;
  gap: 24px;
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

.worker-status {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-text {
  font-size: 14px;
  font-weight: 500;
}

.code-text {
  font-family: monospace;
  background-color: #f8f8f8;
  padding: 2px 5px;
  border-radius: 3px;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 16px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-label {
  font-size: 12px;
  color: #909399;
}

.info-value {
  font-size: 14px;
}

.resource-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 24px;
}

.resource-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.resource-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.resource-label {
  font-size: 14px;
  color: #606266;
}

.resource-value {
  font-size: 14px;
  font-weight: 500;
}

.resource-details {
  font-size: 12px;
  color: #909399;
  text-align: right;
}

.status-success {
  color: #67c23a;
}

.status-warning {
  color: #e6a23c;
}

.status-danger {
  color: #f56c6c;
}

.last-updated {
  font-size: 12px;
  color: #909399;
}

.tags-container {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  min-height: 40px;
}

.tag-item {
  margin-right: 0;
}

.no-tags, .no-jobs {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px;
  color: #909399;
  font-style: italic;
}

.no-jobs i, .health-no-data i {
  font-size: 24px;
  margin-bottom: 8px;
}

.jobs-container {
  min-height: 100px;
}

.job-link {
  color: #409EFF;
  text-decoration: none;
}

.job-link:hover {
  text-decoration: underline;
}

.job-count {
  font-size: 14px;
  color: #606266;
}

.health-container {
  min-height: 100px;
}

.health-result {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.health-status {
  display: flex;
  align-items: center;
  gap: 8px;
}

.health-text {
  font-size: 14px;
  font-weight: 500;
}

.health-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.health-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.health-label {
  font-size: 14px;
  color: #606266;
  font-weight: 500;
  min-width: 100px;
}

.health-value {
  font-size: 14px;
}

.health-no-data {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px;
  color: #909399;
  text-align: center;
}

.health-no-data p {
  margin-bottom: 8px;
}

.tag-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.tag-key-input {
  flex: 1;
}

.equals-sign {
  font-weight: bold;
  color: #909399;
}

.tag-value-input {
  flex: 2;
}

.add-tag-button {
  margin-top: 16px;
}
</style>