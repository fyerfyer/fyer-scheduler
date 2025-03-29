<template>
  <div class="worker-card" :class="{ 'worker-disabled': !worker.is_active }">
    <div class="worker-card-header">
      <div class="worker-name-container">
        <StatusBadge :status="workerStatus" />
        <router-link :to="`/workers/${worker.id}`" class="worker-name">
          {{ worker.hostname || worker.id }}
        </router-link>
        <span v-if="worker.tags && Object.keys(worker.tags).length > 0" class="worker-tags">
          <el-tag
            v-for="(value, key) in worker.tags"
            :key="key"
            size="small"
            class="tag-item"
          >
            {{ key }}={{ value }}
          </el-tag>
        </span>
      </div>

      <div class="worker-actions">
        <el-dropdown @command="handleAction" trigger="click">
          <el-button type="text" size="small">
            <i class="el-icon-more"></i>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="view">View Details</el-dropdown-item>
              <el-dropdown-item
                command="enable"
                v-if="!worker.is_active"
                :disabled="isLoading"
              >
                Enable Worker
              </el-dropdown-item>
              <el-dropdown-item
                command="disable"
                v-if="worker.is_active"
                :disabled="isLoading"
              >
                Disable Worker
              </el-dropdown-item>
              <el-dropdown-item command="healthCheck" :disabled="isLoading">
                Check Health
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div class="worker-card-body">
      <div class="worker-info">
        <div class="worker-id">
          <span class="info-label">ID:</span>
          <span class="info-value">{{ truncateId(worker.id) }}</span>
        </div>
        <div class="worker-address">
          <span class="info-label">Address:</span>
          <span class="info-value">{{ worker.address || 'N/A' }}</span>
        </div>
        <div class="worker-uptime" v-if="worker.uptime_seconds">
          <span class="info-label">Uptime:</span>
          <span class="info-value">{{ formatUptime(worker.uptime_seconds) }}</span>
        </div>
      </div>

      <div class="worker-resources" v-if="worker.resources">
        <div class="resource-usage">
          <span class="resource-label">CPU:</span>
          <el-progress
            :percentage="resourceUsage.cpu"
            :status="getCpuStatus(resourceUsage.cpu)"
            :stroke-width="8"
            :show-text="false"
          />
          <span class="resource-value">{{ resourceUsage.cpu.toFixed(1) }}%</span>
        </div>

        <div class="resource-usage">
          <span class="resource-label">Memory:</span>
          <el-progress
            :percentage="resourceUsage.memory"
            :status="getMemoryStatus(resourceUsage.memory)"
            :stroke-width="8"
            :show-text="false"
          />
          <span class="resource-value">{{ resourceUsage.memory.toFixed(1) }}%</span>
        </div>

        <div class="resource-usage">
          <span class="resource-label">Disk:</span>
          <el-progress
            :percentage="resourceUsage.disk"
            :status="getDiskStatus(resourceUsage.disk)"
            :stroke-width="8"
            :show-text="false"
          />
          <span class="resource-value">{{ resourceUsage.disk.toFixed(1) }}%</span>
        </div>
      </div>

      <div class="worker-meta">
        <div class="worker-jobs">
          <span class="meta-label">Running Jobs:</span>
          <span class="meta-value">{{ worker.running_jobs || 0 }}</span>
        </div>
        <div class="worker-lastcheck">
          <span class="meta-label">Last Check:</span>
          <span class="meta-value">{{ formatLastChecked(worker.last_heartbeat) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useStore } from 'vuex';
import StatusBadge from '@/components/common/StatusBadge.vue';
import workersService from '@/services/workers.service';

export default {
  name: 'WorkerCard',
  components: {
    StatusBadge
  },
  props: {
    worker: {
      type: Object,
      required: true
    }
  },
  setup(props) {
    const store = useStore();
    const router = useRouter();
    const isLoading = ref(false);

    // Computed properties
    const workerStatus = computed(() => {
      if (!props.worker.is_active) {
        return 'disabled';
      }
      return props.worker.is_healthy ? 'healthy' : 'unhealthy';
    });

    const resourceUsage = computed(() => {
      return workersService.calculateResourceUsage(props.worker);
    });

    // Methods
    const truncateId = (id) => {
      if (!id) return 'N/A';
      if (id.length <= 8) return id;
      return `${id.substring(0, 8)}...`;
    };

    const formatUptime = (uptimeSeconds) => {
      return workersService.formatUptime(uptimeSeconds);
    };

    const formatLastChecked = (timestamp) => {
      if (!timestamp) return 'Never';

      const date = new Date(timestamp);
      const now = new Date();
      const diffMs = now - date;
      const diffSec = Math.floor(diffMs / 1000);

      if (diffSec < 60) {
        return `${diffSec} sec ago`;
      } else if (diffSec < 3600) {
        return `${Math.floor(diffSec / 60)} min ago`;
      } else if (diffSec < 86400) {
        return `${Math.floor(diffSec / 3600)} hours ago`;
      } else {
        return date.toLocaleDateString();
      }
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

    const handleAction = async (command) => {
      try {
        isLoading.value = true;

        switch (command) {
          case 'view':
            router.push(`/workers/${props.worker.id}`);
            break;

          case 'enable':
            await store.dispatch('workers/enableWorker', props.worker.id);
            break;

          case 'disable':
            await store.dispatch('workers/disableWorker', props.worker.id);
            break;

          case 'healthCheck':
            await store.dispatch('workers/checkWorkerHealth', props.worker.id);
            break;
        }
      } catch (error) {
        console.error(`Worker action ${command} failed:`, error);
      } finally {
        isLoading.value = false;
      }
    };

    return {
      workerStatus,
      resourceUsage,
      isLoading,
      truncateId,
      formatUptime,
      formatLastChecked,
      getCpuStatus,
      getMemoryStatus,
      getDiskStatus,
      handleAction
    };
  }
};
</script>

<style scoped>
.worker-card {
  background-color: #fff;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  padding: 16px;
  margin-bottom: 16px;
  transition: all 0.3s;
}

.worker-card:hover {
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
}

.worker-disabled {
  opacity: 0.7;
  border-left: 3px solid #8c8c8c;
}

.worker-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.worker-name-container {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.worker-name {
  margin-left: 8px;
  font-weight: 500;
  font-size: 16px;
  color: #303133;
  text-decoration: none;
}

.worker-name:hover {
  color: #1890ff;
  text-decoration: underline;
}

.worker-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.tag-item {
  font-size: 10px;
  height: 20px;
  line-height: 18px;
}

.worker-card-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.worker-info {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  font-size: 13px;
}

.info-label {
  color: #909399;
  margin-right: 4px;
}

.worker-resources {
  margin-top: 8px;
}

.resource-usage {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
}

.resource-label {
  width: 60px;
  font-size: 12px;
  color: #606266;
}

.resource-value {
  width: 50px;
  text-align: right;
  font-size: 12px;
  color: #606266;
}

.el-progress {
  flex: 1;
  margin: 0 8px;
}

.worker-meta {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #909399;
  margin-top: 8px;
}

.meta-label {
  margin-right: 4px;
}

.meta-value {
  font-weight: 500;
  color: #606266;
}
</style>