<template>
  <div class="job-detail-view">
    <!-- Loading state -->
    <LoadingSpinner v-if="loading" text="Loading job details..." />

    <!-- Error state -->
    <el-alert
      v-else-if="error"
      :title="error"
      type="error"
      show-icon
      closable
      @close="clearError"
      class="alert-message"
    />

    <!-- Job not found -->
    <div v-else-if="!job" class="not-found">
      <h2>Job Not Found</h2>
      <p>The requested job could not be found.</p>
      <el-button @click="goToJobsList">Back to Jobs List</el-button>
    </div>

    <!-- Job details -->
    <div v-else>
      <div class="page-header">
        <div class="page-title">
          <h1>Job: {{ job.name }}</h1>
          <StatusBadge :status="job.status" size="large" />
        </div>

        <div class="page-actions">
          <!-- Action buttons -->
          <el-button-group>
            <el-button
              v-if="job.enabled"
              @click="handleDisableJob"
              type="warning"
              :loading="isSubmitting"
            >
              Disable
            </el-button>
            <el-button
              v-else
              @click="handleEnableJob"
              type="success"
              :loading="isSubmitting"
            >
              Enable
            </el-button>
            <el-button
              @click="handleRunJob"
              type="primary"
              :disabled="!canRunJob"
              :loading="isSubmitting"
            >
              Run Now
            </el-button>
          </el-button-group>

          <el-dropdown @command="handleCommand" trigger="click">
            <el-button>
              More Actions <i class="el-icon-arrow-down"></i>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="edit">Edit Job</el-dropdown-item>
                <el-dropdown-item command="logs">View Logs</el-dropdown-item>
                <el-dropdown-item command="delete" divided>Delete Job</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>

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

      <!-- Job details component -->
      <JobDetails
        :job="job"
        :executions="recentExecutions"
        @view-logs="viewAllLogs"
        @view-execution="viewExecutionDetails"
      />
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue';
import { useStore } from 'vuex';
import { useRouter, useRoute } from 'vue-router';
import StatusBadge from '@/components/common/StatusBadge.vue';
import LoadingSpinner from '@/components/common/LoadingSpinner.vue';
import JobDetails from '@/components/jobs/JobDetails.vue';

export default {
  name: 'JobDetailView',
  components: {
    StatusBadge,
    LoadingSpinner,
    JobDetails
  },
  setup() {
    const store = useStore();
    const router = useRouter();
    const route = useRoute();
    const isSubmitting = ref(false);
    const recentExecutions = ref([]);

    // Get job ID from route
    const jobId = route.params.id;

    // Computed properties
    const job = computed(() => store.getters['jobs/currentJob']);
    const loading = computed(() => store.getters['jobs/isLoadingJob']);
    const error = computed(() => store.getters['jobs/error']);
    const successMessage = computed(() => store.getters['jobs/successMessage']);
    const hasEnvVars = computed(() => job.value && job.value.env && Object.keys(job.value.env).length > 0);
    const canRunJob = computed(() =>
      job.value &&
      job.value.enabled &&
      job.value.status !== 'running'
    );

    // Load job data
    const loadJob = async () => {
      try {
        await store.dispatch('jobs/fetchJob', jobId);
        // Load recent executions (mock data for now)
        loadRecentExecutions();
      } catch (err) {
        console.error('Failed to load job:', err);
      }
    };

    // Load recent executions (this would come from an API in a real implementation)
    const loadRecentExecutions = async () => {
      // Mock data for now - in a real app, this would be fetched from an API
      recentExecutions.value = [
        {
          id: 'exec-001',
          status: 'completed',
          start_time: new Date(Date.now() - 3600000).toISOString(), // 1 hour ago
          end_time: new Date(Date.now() - 3595000).toISOString(),   // 5 minutes later
          exit_code: 0
        },
        {
          id: 'exec-002',
          status: 'failed',
          start_time: new Date(Date.now() - 7200000).toISOString(), // 2 hours ago
          end_time: new Date(Date.now() - 7195000).toISOString(),   // 5 minutes later
          exit_code: 1
        }
      ];
    };

    // Format date
    const formatDate = (dateString) => {
      if (!dateString) return 'Never';

      const date = new Date(dateString);
      return date.toLocaleString();
    };

    // Clear messages
    const clearError = () => {
      store.commit('jobs/SET_ERROR', null);
    };

    const clearSuccessMessage = () => {
      store.commit('jobs/SET_SUCCESS_MESSAGE', null);
    };

    // Action handlers
    const handleRunJob = async () => {
      isSubmitting.value = true;

      try {
        await store.dispatch('jobs/triggerJob', jobId);
        // Reload job to get updated status
        await loadJob();
      } catch (err) {
        console.error('Failed to trigger job:', err);
      } finally {
        isSubmitting.value = false;
      }
    };

    const handleEnableJob = async () => {
      isSubmitting.value = true;

      try {
        await store.dispatch('jobs/enableJob', jobId);
        // Reload job to get updated status
        await loadJob();
      } catch (err) {
        console.error('Failed to enable job:', err);
      } finally {
        isSubmitting.value = false;
      }
    };

    const handleDisableJob = async () => {
      isSubmitting.value = true;

      try {
        await store.dispatch('jobs/disableJob', jobId);
        // Reload job to get updated status
        await loadJob();
      } catch (err) {
        console.error('Failed to disable job:', err);
      } finally {
        isSubmitting.value = false;
      }
    };

    const handleCommand = (command) => {
      switch (command) {
        case 'edit':
          router.push(`/jobs/${jobId}/edit`);
          break;
        case 'logs':
          router.push(`/logs/jobs/${jobId}`);
          break;
        case 'delete':
          confirmDelete();
          break;
      }
    };

    const confirmDelete = () => {
      // Show confirmation dialog
      if (confirm('Are you sure you want to delete this job?')) {
        deleteJob();
      }
    };

    const deleteJob = async () => {
      isSubmitting.value = true;

      try {
        await store.dispatch('jobs/deleteJob', jobId);
        // Navigate back to jobs list
        router.push('/jobs');
      } catch (err) {
        console.error('Failed to delete job:', err);
      } finally {
        isSubmitting.value = false;
      }
    };

    const goToJobsList = () => {
      router.push('/jobs');
    };

    const viewAllLogs = () => {
      router.push(`/logs/jobs/${jobId}`);
    };

    const viewExecutionDetails = (executionId) => {
      router.push(`/logs/execution/${executionId}`);
    };

    // Load job on mount
    onMounted(() => {
      loadJob();
    });

    return {
      job,
      loading,
      error,
      successMessage,
      isSubmitting,
      hasEnvVars,
      canRunJob,
      recentExecutions,
      formatDate,
      clearError,
      clearSuccessMessage,
      handleRunJob,
      handleEnableJob,
      handleDisableJob,
      handleCommand,
      goToJobsList,
      viewAllLogs,
      viewExecutionDetails
    };
  }
};
</script>

<style scoped>
.job-detail-view {
  padding-bottom: 40px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.alert-message {
  margin-bottom: 20px;
}

.not-found {
  text-align: center;
  padding: 40px;
}
</style>