<template>
  <div class="jobs-page">
    <!-- Page header with stats -->
    <div class="page-header">
      <h1>Jobs</h1>
      <el-button type="primary" @click="handleCreateJob">
        <i class="el-icon-plus"></i> Create Job
      </el-button>
    </div>

    <!-- Stats cards -->
    <div class="stats-cards">
      <div class="stat-card" v-for="(count, status) in jobsByStatus" :key="status">
        <div class="stat-card-value">{{ count }}</div>
        <div class="stat-card-label">
          <StatusBadge :status="status" />
          <span>{{ formatStatusLabel(status) }}</span>
        </div>
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
    />

    <!-- Success message -->
    <el-alert
      v-if="successMessage"
      :title="successMessage"
      type="success"
      show-icon
      closable
      @close="clearSuccessMessage"
    />

    <!-- Job list -->
    <div class="job-list-wrapper">
      <JobList :initial-status-filter="statusFilter" />
    </div>
  </div>
</template>

<script>
import { computed, ref, onMounted } from 'vue';
import { useStore } from 'vuex';
import { useRouter, useRoute } from 'vue-router';
import StatusBadge from '@/components/common/StatusBadge.vue';
import JobList from '@/components/jobs/JobList.vue';

export default {
  name: 'JobsView',
  components: {
    StatusBadge,
    JobList
  },
  setup() {
    const store = useStore();
    const router = useRouter();
    const route = useRoute();

    // Get status filter from query parameter if present
    const statusFilter = ref(route.query.status || '');

    // Computed properties
    const jobsByStatus = computed(() => store.getters['jobs/jobsByStatus']);
    const error = computed(() => store.getters['jobs/error']);
    const successMessage = computed(() => store.getters['jobs/successMessage']);

    // Methods
    const handleCreateJob = () => {
      router.push('/jobs/create');
    };

    const formatStatusLabel = (status) => {
      const labels = {
        running: 'Running',
        completed: 'Completed',
        failed: 'Failed',
        pending: 'Pending',
        disabled: 'Disabled'
      };

      return labels[status] || status;
    };

    const clearError = () => {
      store.commit('jobs/SET_ERROR', null);
    };

    const clearSuccessMessage = () => {
      store.commit('jobs/SET_SUCCESS_MESSAGE', null);
    };

    // Initialize
    onMounted(() => {
      // Set status filter from URL if present
      if (statusFilter.value) {
        store.dispatch('jobs/setStatusFilter', statusFilter.value);
      }
    });

    return {
      jobsByStatus,
      error,
      successMessage,
      statusFilter,
      handleCreateJob,
      formatStatusLabel,
      clearError,
      clearSuccessMessage
    };
  }
};
</script>

<style scoped>
.jobs-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
  margin-bottom: 8px;
}

.stat-card {
  background-color: white;
  border-radius: 4px;
  padding: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  display: flex;
  flex-direction: column;
  transition: transform 0.2s;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
}

.stat-card-value {
  font-size: 28px;
  font-weight: 600;
  margin-bottom: 8px;
}

.stat-card-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #606266;
}

.job-list-wrapper {
  background-color: white;
  border-radius: 4px;
  padding: 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}
</style>