<template>
  <div class="quick-actions-container">
    <div class="quick-actions-header" v-if="title">
      <h3>{{ title }}</h3>
    </div>

    <div class="quick-actions-grid">
      <!-- Create Job action -->
      <el-card
        class="action-card"
        shadow="hover"
        @click="navigateTo('/jobs/create')"
        v-if="showCreateJob"
      >
        <div class="action-icon">
          <i class="el-icon-plus"></i>
        </div>
        <div class="action-content">
          <span class="action-title">Create Job</span>
          <span class="action-description">Create a new scheduled job</span>
        </div>
      </el-card>

      <!-- Manage Jobs action -->
      <el-card
        class="action-card"
        shadow="hover"
        @click="navigateTo('/jobs')"
        v-if="showManageJobs"
      >
        <div class="action-icon jobs-icon">
          <i class="el-icon-tickets"></i>
        </div>
        <div class="action-content">
          <span class="action-title">Manage Jobs</span>
          <span class="action-description">View and manage all jobs</span>
        </div>
      </el-card>

      <!-- Manage Workers action -->
      <el-card
        class="action-card"
        shadow="hover"
        @click="navigateTo('/workers')"
        v-if="showManageWorkers"
      >
        <div class="action-icon workers-icon">
          <i class="el-icon-cpu"></i>
        </div>
        <div class="action-content">
          <span class="action-title">Manage Workers</span>
          <span class="action-description">View and manage worker nodes</span>
        </div>
      </el-card>

      <!-- View Logs action -->
      <el-card
        class="action-card"
        shadow="hover"
        @click="navigateTo('/logs')"
        v-if="showViewLogs"
      >
        <div class="action-icon logs-icon">
          <i class="el-icon-document"></i>
        </div>
        <div class="action-content">
          <span class="action-title">View Logs</span>
          <span class="action-description">Check execution logs</span>
        </div>
      </el-card>

      <!-- Trigger Job action -->
      <el-card
        class="action-card"
        shadow="hover"
        @click="showJobSelector"
        v-if="showTriggerJob"
      >
        <div class="action-icon trigger-icon">
          <i class="el-icon-video-play"></i>
        </div>
        <div class="action-content">
          <span class="action-title">Trigger Job</span>
          <span class="action-description">Run a job manually</span>
        </div>
      </el-card>

      <!-- System Status action -->
      <el-card
        class="action-card"
        shadow="hover"
        @click="navigateTo('/')"
        v-if="showSystemStatus"
      >
        <div class="action-icon status-icon">
          <i class="el-icon-data-analysis"></i>
        </div>
        <div class="action-content">
          <span class="action-title">System Status</span>
          <span class="action-description">View system dashboard</span>
        </div>
      </el-card>

      <!-- Custom actions -->
      <el-card
        v-for="(action, index) in customActions"
        :key="index"
        class="action-card"
        shadow="hover"
        @click="handleCustomAction(action)"
      >
        <div class="action-icon" :class="action.iconClass">
          <i :class="action.icon"></i>
        </div>
        <div class="action-content">
          <span class="action-title">{{ action.title }}</span>
          <span class="action-description">{{ action.description }}</span>
        </div>
      </el-card>
    </div>

    <!-- Job selector dialog -->
    <el-dialog
      title="Select Job to Run"
      v-model="jobSelectorVisible"
      width="500px"
    >
      <div v-if="isLoadingJobs" class="dialog-loading">
        <i class="el-icon-loading"></i>
        <span>Loading jobs...</span>
      </div>

      <div v-else-if="availableJobs.length === 0" class="dialog-empty">
        <i class="el-icon-warning-outline"></i>
        <p>No available jobs found.</p>
        <p>Create a job first to trigger manual execution.</p>
      </div>

      <el-select
        v-else
        v-model="selectedJobId"
        placeholder="Select a job"
        style="width: 100%"
      >
        <el-option
          v-for="job in availableJobs"
          :key="job.id"
          :label="job.name"
          :value="job.id"
        >
          <div class="job-option">
            <span>{{ job.name }}</span>
            <span class="job-description" v-if="job.description">{{ job.description }}</span>
          </div>
        </el-option>
      </el-select>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="jobSelectorVisible = false">Cancel</el-button>
          <el-button
            type="primary"
            @click="triggerSelectedJob"
            :disabled="!selectedJobId || isTriggering"
            :loading="isTriggering"
          >
            Run Now
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { ref, computed } from 'vue';
import { useRouter } from 'vue-router';
import { useStore } from 'vuex';

export default {
  name: 'QuickActions',
  props: {
    // Optional title for the quick actions section
    title: {
      type: String,
      default: ''
    },
    // Control which standard actions to show
    showCreateJob: {
      type: Boolean,
      default: true
    },
    showManageJobs: {
      type: Boolean,
      default: true
    },
    showManageWorkers: {
      type: Boolean,
      default: true
    },
    showViewLogs: {
      type: Boolean,
      default: true
    },
    showTriggerJob: {
      type: Boolean,
      default: true
    },
    showSystemStatus: {
      type: Boolean,
      default: true
    },
    // Custom actions to add
    customActions: {
      type: Array,
      default: () => []
    }
  },
  emits: ['action-clicked', 'job-triggered'],
  setup(props, { emit }) {
    const router = useRouter();
    const store = useStore();

    // State for job selector dialog
    const jobSelectorVisible = ref(false);
    const selectedJobId = ref('');
    const isLoadingJobs = ref(false);
    const isTriggering = ref(false);
    const availableJobs = ref([]);

    // Navigate to a specific route
    const navigateTo = (route) => {
      router.push(route);
      emit('action-clicked', { type: 'navigation', route });
    };

    // Handle custom action click
    const handleCustomAction = (action) => {
      if (action.route) {
        router.push(action.route);
      }

      if (action.handler && typeof action.handler === 'function') {
        action.handler();
      }

      emit('action-clicked', {
        type: 'custom',
        action
      });
    };

    // Show job selector dialog
    const showJobSelector = async () => {
      jobSelectorVisible.value = true;
      selectedJobId.value = '';
      isLoadingJobs.value = true;

      try {
        // Load enabled jobs from the store
        await store.dispatch('jobs/fetchJobs', {
          status: 'enabled',
          page: 1,
          pageSize: 100
        });

        // Get all jobs from the store
        const jobs = store.getters['jobs/allJobs'] || [];

        // Filter only enabled jobs
        availableJobs.value = jobs.filter(job => job.enabled);
      } catch (error) {
        console.error('Failed to load jobs:', error);
        availableJobs.value = [];
      } finally {
        isLoadingJobs.value = false;
      }
    };

    // Trigger the selected job
    const triggerSelectedJob = async () => {
      if (!selectedJobId.value) return;

      isTriggering.value = true;

      try {
        // Trigger the job using the store
        const result = await store.dispatch('jobs/triggerJob', selectedJobId.value);

        // Close the dialog
        jobSelectorVisible.value = false;

        // Notify parent component
        emit('job-triggered', {
          jobId: selectedJobId.value,
          executionId: result.execution_id
        });

        // Show success message
        store.commit('jobs/SET_SUCCESS_MESSAGE', 'Job triggered successfully');

        // Navigate to execution logs if we have an execution ID
        if (result && result.execution_id) {
          router.push(`/logs/execution/${result.execution_id}`);
        }
      } catch (error) {
        console.error('Failed to trigger job:', error);
      } finally {
        isTriggering.value = false;
      }
    };

    return {
      jobSelectorVisible,
      selectedJobId,
      isLoadingJobs,
      isTriggering,
      availableJobs,
      navigateTo,
      handleCustomAction,
      showJobSelector,
      triggerSelectedJob
    };
  }
};
</script>

<style scoped>
.quick-actions-container {
  margin-bottom: 20px;
}

.quick-actions-header {
  margin-bottom: 16px;
}

.quick-actions-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
  color: #606266;
}

.quick-actions-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 20px;
}

.action-card {
  display: flex;
  align-items: center;
  padding: 16px;
  height: 100px;
  cursor: pointer;
  transition: all 0.3s;
}

.action-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.action-icon {
  font-size: 28px;
  width: 50px;
  height: 50px;
  border-radius: 50%;
  background-color: rgba(64, 158, 255, 0.1);
  color: #409EFF;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 16px;
  flex-shrink: 0;
}

.jobs-icon {
  background-color: rgba(103, 194, 58, 0.1);
  color: #67C23A;
}

.workers-icon {
  background-color: rgba(144, 147, 153, 0.1);
  color: #909399;
}

.logs-icon {
  background-color: rgba(230, 162, 60, 0.1);
  color: #E6A23C;
}

.trigger-icon {
  background-color: rgba(245, 108, 108, 0.1);
  color: #F56C6C;
}

.status-icon {
  background-color: rgba(144, 147, 153, 0.1);
  color: #909399;
}

.action-content {
  display: flex;
  flex-direction: column;
  flex: 1;
}

.action-title {
  font-weight: 500;
  font-size: 16px;
  margin-bottom: 4px;
  color: #303133;
}

.action-description {
  font-size: 12px;
  color: #909399;
}

.dialog-loading, .dialog-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px 0;
  color: #909399;
}

.dialog-loading i, .dialog-empty i {
  font-size: 32px;
  margin-bottom: 16px;
}

.dialog-empty p {
  margin: 4px 0;
  text-align: center;
}

.job-option {
  display: flex;
  flex-direction: column;
}

.job-description {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
</style>