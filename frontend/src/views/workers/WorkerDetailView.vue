<template>
  <div class="worker-detail-view">
    <!-- Page header -->
    <div class="page-header">
      <div class="page-title">
        <h1>Worker Details</h1>
        <div class="breadcrumb">
          <router-link to="/workers">Workers</router-link> /
          <span>{{ workerName }}</span>
        </div>
      </div>

      <div class="page-actions">
        <el-button
          type="primary"
          @click="refreshWorker"
          :loading="isLoading"
          icon="el-icon-refresh"
        >
          Refresh
        </el-button>

        <el-button
          v-if="!worker || !worker.is_active"
          type="success"
          @click="enableWorker"
          :loading="isEnabling"
        >
          Enable Worker
        </el-button>

        <el-button
          v-else
          type="danger"
          @click="confirmDisableWorker"
        >
          Disable Worker
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

    <!-- Loading state -->
    <LoadingSpinner v-if="isLoading && !worker" text="Loading worker details..." />

    <!-- Worker not found -->
    <div v-else-if="!worker && !isLoading" class="not-found">
      <h2>Worker Not Found</h2>
      <p>The requested worker could not be found.</p>
      <el-button @click="goToWorkersList">Back to Workers</el-button>
    </div>

    <!-- Worker details -->
    <WorkerDetails
      v-else-if="worker"
      :worker="worker"
      :running-jobs="runningJobs"
      :is-checking-health="isCheckingHealth"
      @check-health="checkWorkerHealth"
      @kill-job="handleKillJob"
      @update-tags="updateWorkerTags"
    />

    <!-- Confirm disable dialog -->
    <el-dialog
      title="Confirm Worker Disable"
      v-model="disableDialogVisible"
      width="500px"
    >
      <div class="disable-dialog-content">
        <i class="el-icon-warning-outline warning-icon"></i>
        <p>Are you sure you want to disable this worker?</p>
        <p class="warning-text">
          The worker will stop accepting new jobs and current running jobs will be allowed to complete.
        </p>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="disableDialogVisible = false">Cancel</el-button>
          <el-button type="danger" @click="disableWorker" :loading="isDisabling">Disable</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import { useStore } from 'vuex';
import { useRouter, useRoute } from 'vue-router';
import WorkerDetails from '@/components/workers/WorkerDetails.vue';
import LoadingSpinner from '@/components/common/LoadingSpinner.vue';

export default {
  name: 'WorkerDetailView',
  components: {
    WorkerDetails,
    LoadingSpinner
  },
  setup() {
    const store = useStore();
    const router = useRouter();
    const route = useRoute();

    // State
    const workerId = ref(route.params.id);
    const runningJobs = ref([]);
    const refreshTimer = ref(null);
    const disableDialogVisible = ref(false);
    const isEnabling = ref(false);
    const isDisabling = ref(false);
    const isCheckingHealth = ref(false);

    // Computed
    const worker = computed(() => store.getters['workers/currentWorker']);
    const isLoading = computed(() => store.getters['workers/isLoadingWorker']);
    const error = computed(() => store.getters['workers/error']);
    const successMessage = computed(() => store.getters['workers/successMessage']);

    const workerName = computed(() => {
      if (!worker.value) return workerId.value;
      return worker.value.hostname || worker.value.id;
    });

    // Methods
    const loadWorker = async () => {
      if (!workerId.value) return;

      try {
        await store.dispatch('workers/fetchWorker', workerId.value);
        await loadRunningJobs();
      } catch (err) {
        console.error('Failed to load worker:', err);
      }
    };

    const loadRunningJobs = async () => {
      // Mock implementation - in a real application you would fetch this from your API
      // For now, we'll simulate it with a static array
      runningJobs.value = [];

      // If worker has running_jobs count, we could simulate some data
      if (worker.value && worker.value.running_jobs > 0) {
        // Generate some mock running jobs
        // In a real implementation, you would fetch this from your API
        for (let i = 0; i < worker.value.running_jobs; i++) {
          runningJobs.value.push({
            id: `exec-${Math.random().toString(36).substring(2, 10)}`,
            job_id: `job-${Math.random().toString(36).substring(2, 10)}`,
            job_name: `Sample Job ${i+1}`,
            start_time: new Date(Date.now() - Math.random() * 3600000).toISOString(),
            worker_id: workerId.value
          });
        }
      }
    };

    const refreshWorker = () => {
      loadWorker();
    };

    const enableWorker = async () => {
      isEnabling.value = true;

      try {
        await store.dispatch('workers/enableWorker', workerId.value);
        // Success will be shown via the success message from the store
      } catch (err) {
        console.error('Failed to enable worker:', err);
      } finally {
        isEnabling.value = false;
      }
    };

    const confirmDisableWorker = () => {
      disableDialogVisible.value = true;
    };

    const disableWorker = async () => {
      isDisabling.value = true;

      try {
        await store.dispatch('workers/disableWorker', workerId.value);
        disableDialogVisible.value = false;
        // Success will be shown via the success message from the store
      } catch (err) {
        console.error('Failed to disable worker:', err);
      } finally {
        isDisabling.value = false;
      }
    };

    const checkWorkerHealth = async () => {
      isCheckingHealth.value = true;

      try {
        await store.dispatch('workers/checkWorkerHealth', workerId.value);
      } catch (err) {
        console.error('Failed to check worker health:', err);
      } finally {
        isCheckingHealth.value = false;
      }
    };

    const handleKillJob = async ({ executionId, jobId }) => {
      try {
        // In a real implementation, you would call your API to kill the job
        // await store.dispatch('jobs/killJob', jobId);

        // For now, we just mock the response and update the UI
        // Remove the job from the list
        runningJobs.value = runningJobs.value.filter(job => job.id !== executionId);

        // Show success message
        store.commit('workers/SET_SUCCESS_MESSAGE', 'Job terminated successfully');
      } catch (err) {
        console.error('Failed to kill job:', err);
        store.commit('workers/SET_ERROR', 'Failed to terminate job');
      }
    };

    const updateWorkerTags = async (tags) => {
      try {
        await store.dispatch('workers/updateWorkerLabels', {
          workerId: workerId.value,
          labels: tags
        });
        // Success will be shown via the success message from the store
      } catch (err) {
        console.error('Failed to update worker tags:', err);
      }
    };

    const goToWorkersList = () => {
      router.push('/workers');
    };

    const clearError = () => {
      store.commit('workers/SET_ERROR', null);
    };

    const clearSuccessMessage = () => {
      store.commit('workers/SET_SUCCESS_MESSAGE', null);
    };

    const setupAutoRefresh = () => {
      refreshTimer.value = setInterval(() => {
        refreshWorker();
      }, 30000); // Refresh every 30 seconds
    };

    const clearAutoRefresh = () => {
      if (refreshTimer.value) {
        clearInterval(refreshTimer.value);
        refreshTimer.value = null;
      }
    };

    // Lifecycle hooks
    onMounted(() => {
      loadWorker();
      setupAutoRefresh();
    });

    onBeforeUnmount(() => {
      clearAutoRefresh();
      store.dispatch('workers/clearCurrentWorker');
    });

    // Watch for route changes
    if (route.params.id !== workerId.value) {
      workerId.value = route.params.id;
      loadWorker();
    }

    return {
      workerId,
      worker,
      runningJobs,
      isLoading,
      error,
      successMessage,
      workerName,
      disableDialogVisible,
      isEnabling,
      isDisabling,
      isCheckingHealth,
      refreshWorker,
      enableWorker,
      confirmDisableWorker,
      disableWorker,
      checkWorkerHealth,
      handleKillJob,
      updateWorkerTags,
      goToWorkersList,
      clearError,
      clearSuccessMessage
    };
  }
};
</script>

<style scoped>
.worker-detail-view {
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

.page-actions {
  display: flex;
  gap: 12px;
}

.alert-message {
  margin-bottom: 0;
}

.not-found {
  text-align: center;
  padding: 40px;
  background-color: white;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.not-found h2 {
  margin-bottom: 16px;
}

.not-found p {
  margin-bottom: 24px;
  color: #606266;
}

.disable-dialog-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 0 24px;
}

.warning-icon {
  font-size: 48px;
  color: #E6A23C;
  margin-bottom: 16px;
}

.warning-text {
  color: #E6A23C;
  margin-top: 16px;
  font-size: 14px;
}
</style>