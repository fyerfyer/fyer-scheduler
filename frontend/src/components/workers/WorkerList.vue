<template>
  <div class="worker-list-container">
    <!-- Filter controls -->
    <div class="worker-list-filters">
      <div class="status-filter">
        <el-radio-group v-model="statusFilter" size="small" @change="handleFilterChange">
          <el-radio-button label="">All</el-radio-button>
          <el-radio-button label="healthy">Healthy</el-radio-button>
          <el-radio-button label="unhealthy">Unhealthy</el-radio-button>
          <el-radio-button label="offline">Offline</el-radio-button>
        </el-radio-group>
      </div>

      <div class="search-filter">
        <el-input
          v-model="searchQuery"
          placeholder="Search workers by hostname or ID"
          prefix-icon="el-icon-search"
          clearable
          @input="handleSearchChange"
          size="small"
        />
      </div>
    </div>

    <!-- Loading state -->
    <LoadingSpinner v-if="loading" text="Loading workers..." />

    <!-- Empty state -->
    <div v-else-if="workers.length === 0" class="worker-list-empty">
      <div class="empty-message">
        <i class="el-icon-cpu"></i>
        <p>No workers found</p>
        <p v-if="statusFilter" class="empty-hint">
          Try changing your filter criteria or checking worker connectivity
        </p>
        <p v-else class="empty-hint">
          Start by connecting worker nodes to your scheduler master
        </p>
      </div>
    </div>

    <!-- Worker list -->
    <div v-else class="worker-list">
      <WorkerCard
        v-for="worker in workers"
        :key="worker.id"
        :worker="worker"
      />
    </div>

    <!-- Pagination -->
    <Pagination
      v-if="totalWorkers > 0"
      :currentPage="currentPage"
      :pageSize="pageSize"
      :total="totalWorkers"
      @change="handlePageChange"
      @update:pageSize="handlePageSizeChange"
    />
  </div>
</template>

<script>
import { ref, computed, onMounted, watch } from 'vue';
import { useStore } from 'vuex';
import { useRouter } from 'vue-router';
import WorkerCard from './WorkerCard.vue';
import LoadingSpinner from '@/components/common/LoadingSpinner.vue';
import Pagination from '@/components/common/Pagination.vue';

export default {
  name: 'WorkerList',
  components: {
    WorkerCard,
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
    const workers = computed(() => store.getters['workers/allWorkers']);
    const totalWorkers = computed(() => store.getters['workers/totalWorkersCount']);
    const loading = computed(() => store.getters['workers/isLoading']);
    const error = computed(() => store.getters['workers/error']);

    // Methods
    const loadWorkers = async () => {
      try {
        await store.dispatch('workers/fetchWorkers', {
          page: currentPage.value,
          pageSize: pageSize.value,
          status: statusFilter.value
        });
      } catch (err) {
        console.error('Failed to load workers:', err);
      }
    };

    const handlePageChange = (page) => {
      currentPage.value = page;
      loadWorkers();
    };

    const handlePageSizeChange = (size) => {
      pageSize.value = size;
      currentPage.value = 1; // Reset to first page when changing page size
      loadWorkers();
    };

    const handleFilterChange = () => {
      currentPage.value = 1; // Reset to first page when changing filter
      loadWorkers();

      // Update URL with status filter
      if (statusFilter.value) {
        router.push({ query: { status: statusFilter.value } });
      } else {
        router.push({ query: {} });
      }
    };

    const handleSearchChange = () => {
      // Debounce search to avoid too many requests
      if (debounceTimeout.value) {
        clearTimeout(debounceTimeout.value);
      }

      debounceTimeout.value = setTimeout(() => {
        currentPage.value = 1; // Reset to first page when searching
        loadWorkers();
      }, 300);
    };

    // Auto-refresh functionality
    const startAutoRefresh = () => {
      const refreshInterval = setInterval(() => {
        loadWorkers();
      }, 30000); // Refresh every 30 seconds

      return refreshInterval;
    };

    // Watch for prop changes
    watch(() => props.initialStatusFilter, (newValue) => {
      statusFilter.value = newValue;
      handleFilterChange();
    });

    // Load workers on component mount
    onMounted(() => {
      loadWorkers();

      // Set up auto-refresh and clean up on unmount
      const refreshInterval = startAutoRefresh();
      return () => clearInterval(refreshInterval);
    });

    return {
      workers,
      totalWorkers,
      loading,
      error,
      statusFilter,
      searchQuery,
      currentPage,
      pageSize,
      handlePageChange,
      handlePageSizeChange,
      handleFilterChange,
      handleSearchChange
    };
  }
};
</script>

<style scoped>
.worker-list-container {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.worker-list-filters {
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 16px;
}

.worker-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.worker-list-empty {
  display: flex;
  justify-content: center;
  padding: 48px 0;
  background-color: #fff;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
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
  margin-bottom: 8px;
}

.empty-hint {
  font-size: 14px;
  color: #c0c4cc;
  text-align: center;
}

.search-filter {
  width: 240px;
}

@media (max-width: 768px) {
  .worker-list-filters {
    flex-direction: column;
    align-items: stretch;
  }

  .search-filter {
    width: 100%;
  }
}
</style>