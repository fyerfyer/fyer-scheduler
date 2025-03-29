<template>
  <div class="job-list-container">
    <!-- Filter controls -->
    <div class="job-list-filters">
      <div class="status-filter">
        <el-radio-group v-model="statusFilter" size="small" @change="handleFilterChange">
          <el-radio-button label="">All</el-radio-button>
          <el-radio-button label="running">Running</el-radio-button>
          <el-radio-button label="completed">Completed</el-radio-button>
          <el-radio-button label="failed">Failed</el-radio-button>
          <el-radio-button label="pending">Pending</el-radio-button>
          <el-radio-button label="disabled">Disabled</el-radio-button>
        </el-radio-group>
      </div>

      <div class="search-filter">
        <el-input
          v-model="searchQuery"
          placeholder="Search jobs..."
          clearable
          prefix-icon="el-icon-search"
          @input="handleSearchChange"
        />
      </div>
    </div>

    <!-- Loading state -->
    <LoadingSpinner v-if="loading" text="Loading jobs..." />

    <!-- Empty state -->
    <div v-else-if="jobs.length === 0" class="job-list-empty">
      <div class="empty-message">
        <i class="el-icon-document"></i>
        <p>No jobs found</p>
        <el-button type="primary" size="small" @click="handleCreateJob">Create Job</el-button>
      </div>
    </div>

    <!-- Job list -->
    <div v-else class="job-list">
      <JobCard
        v-for="job in jobs"
        :key="job.id"
        :job="job"
      />
    </div>

    <!-- Pagination -->
    <Pagination
      v-if="jobs.length > 0"
      :current-page="currentPage"
      :page-size="pageSize"
      :total="totalJobs"
      @update:currentPage="handlePageChange"
      @update:pageSize="handlePageSizeChange"
    />
  </div>
</template>

<script>
import { ref, computed, onMounted, watch } from 'vue';
import { useStore } from 'vuex';
import { useRouter } from 'vue-router';
import JobCard from './JobCard.vue';
import LoadingSpinner from '@/components/common/LoadingSpinner.vue';
import Pagination from '@/components/common/Pagination.vue';

export default {
  name: 'JobList',
  components: {
    JobCard,
    LoadingSpinner,
    Pagination
  },
  props: {
    initialStatusFilter: {
      type: String,
      default: ''
    }
  },
  setup(props) {
    const store = useStore();
    const router = useRouter();

    // State
    const statusFilter = ref(props.initialStatusFilter);
    const searchQuery = ref('');
    const currentPage = ref(1);
    const pageSize = ref(10);
    const debounceTimeout = ref(null);

    // Computed properties
    const jobs = computed(() => store.getters['jobs/allJobs']);
    const totalJobs = computed(() => store.getters['jobs/totalJobsCount']);
    const loading = computed(() => store.getters['jobs/isLoading']);
    const error = computed(() => store.getters['jobs/error']);

    // Methods
    const loadJobs = async () => {
      try {
        await store.dispatch('jobs/fetchJobs', {
          page: currentPage.value,
          pageSize: pageSize.value,
          status: statusFilter.value
        });
      } catch (err) {
        console.error('Failed to load jobs:', err);
      }
    };

    const handlePageChange = (page) => {
      currentPage.value = page;
      loadJobs();
    };

    const handlePageSizeChange = (size) => {
      pageSize.value = size;
      currentPage.value = 1; // Reset to first page when changing page size
      loadJobs();
    };

    const handleFilterChange = () => {
      currentPage.value = 1; // Reset to first page when filtering
      store.dispatch('jobs/setStatusFilter', statusFilter.value);
      loadJobs();
    };

    const handleSearchChange = () => {
      // Debounce search to avoid excessive API calls
      if (debounceTimeout.value) {
        clearTimeout(debounceTimeout.value);
      }

      debounceTimeout.value = setTimeout(() => {
        currentPage.value = 1; // Reset to first page when searching
        loadJobs();
      }, 300);
    };

    const handleCreateJob = () => {
      router.push('/jobs/create');
    };

    // Watch for prop changes
    watch(() => props.initialStatusFilter, (newValue) => {
      statusFilter.value = newValue;
      handleFilterChange();
    });

    // Load jobs on component mount
    onMounted(() => {
      loadJobs();
    });

    return {
      statusFilter,
      searchQuery,
      currentPage,
      pageSize,
      jobs,
      totalJobs,
      loading,
      error,
      handlePageChange,
      handlePageSizeChange,
      handleFilterChange,
      handleSearchChange,
      handleCreateJob
    };
  }
};
</script>

<style scoped>
.job-list-container {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.job-list-filters {
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 16px;
}

.job-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.job-list-empty {
  display: flex;
  justify-content: center;
  padding: 48px 0;
  background-color: #fff;
  border-radius: 4px;
}

.empty-message {
  display: flex;
  flex-direction: column;
  align-items: center;
  color: #909399;
}

.empty-message i {
  font-size: 48px;
  margin-bottom: 16px;
}

.empty-message p {
  margin-bottom: 16px;
}
</style>