<template>
  <div class="log-filter-panel">
    <el-form :model="filterForm" @submit.prevent class="filter-form">
      <!-- Time range filter -->
      <el-form-item label="Time Range">
        <el-date-picker
          v-model="filterForm.timeRange"
          type="datetimerange"
          range-separator="to"
          start-placeholder="Start time"
          end-placeholder="End time"
          :shortcuts="dateShortcuts"
          format="YYYY-MM-DD HH:mm:ss"
          value-format="YYYY-MM-DD HH:mm:ss"
          size="small"
          @change="handleTimeRangeChange"
        />
      </el-form-item>

      <!-- Log level filter -->
      <el-form-item label="Log Level">
        <el-select
          v-model="filterForm.level"
          placeholder="Select log level"
          size="small"
          @change="handleLevelChange"
          clearable
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
      </el-form-item>

      <!-- Search filter -->
      <el-form-item label="Search">
        <el-input
          v-model="filterForm.searchText"
          placeholder="Search in logs"
          size="small"
          clearable
          @input="debouncedSearch"
        >
          <template #prefix>
            <i class="el-icon-search"></i>
          </template>
        </el-input>
      </el-form-item>

      <!-- Filter actions -->
      <el-form-item>
        <el-button
          type="primary"
          size="small"
          @click="applyFilters"
          :loading="isFiltering"
        >
          Apply Filters
        </el-button>
        <el-button
          size="small"
          @click="resetFilters"
          :disabled="isFiltering"
        >
          Reset
        </el-button>
      </el-form-item>
    </el-form>

    <!-- Active filters display -->
    <div v-if="hasActiveFilters" class="active-filters">
      <span class="active-filters-label">Active Filters:</span>
      <el-tag
        v-if="filterForm.timeRange && filterForm.timeRange.length"
        size="small"
        closable
        @close="clearTimeRange"
      >
        Time: {{ formatTimeRange(filterForm.timeRange) }}
      </el-tag>
      <el-tag
        v-if="filterForm.level"
        size="small"
        closable
        @close="clearLevel"
        :class="`level-tag-${filterForm.level.toLowerCase()}`"
      >
        Level: {{ filterForm.level }}
      </el-tag>
      <el-tag
        v-if="filterForm.searchText"
        size="small"
        closable
        @close="clearSearch"
      >
        Search: {{ filterForm.searchText }}
      </el-tag>
    </div>
  </div>
</template>

<script>
import { ref, computed, watch } from 'vue';

export default {
  name: 'LogFilter',
  props: {
    initialTimeRange: {
      type: Array,
      default: () => []
    },
    initialLevel: {
      type: String,
      default: ''
    },
    initialSearchText: {
      type: String,
      default: ''
    },
    isFiltering: {
      type: Boolean,
      default: false
    }
  },
  emits: ['update:timeRange', 'update:level', 'update:searchText', 'filter', 'reset'],
  setup(props, { emit }) {
    // Filter form state
    const filterForm = ref({
      timeRange: props.initialTimeRange.length ? [...props.initialTimeRange] : [],
      level: props.initialLevel,
      searchText: props.initialSearchText
    });

    // Log level options
    const logLevels = [
      { value: 'INFO', label: 'Information' },
      { value: 'WARN', label: 'Warning' },
      { value: 'ERROR', label: 'Error' },
      { value: 'DEBUG', label: 'Debug' }
    ];

    // Date shortcut options
    const dateShortcuts = [
      {
        text: 'Last hour',
        value: () => {
          const end = new Date();
          const start = new Date();
          start.setTime(start.getTime() - 3600 * 1000);
          return [start, end];
        }
      },
      {
        text: 'Last 24 hours',
        value: () => {
          const end = new Date();
          const start = new Date();
          start.setTime(start.getTime() - 3600 * 1000 * 24);
          return [start, end];
        }
      },
      {
        text: 'Last 7 days',
        value: () => {
          const end = new Date();
          const start = new Date();
          start.setTime(start.getTime() - 3600 * 1000 * 24 * 7);
          return [start, end];
        }
      }
    ];

    // Check if there are any active filters
    const hasActiveFilters = computed(() => {
      return (
        (filterForm.value.timeRange && filterForm.value.timeRange.length) ||
        filterForm.value.level ||
        filterForm.value.searchText
      );
    });

    // Debounce search to avoid too many requests
    let searchTimeout = null;
    const debouncedSearch = () => {
      if (searchTimeout) {
        clearTimeout(searchTimeout);
      }

      searchTimeout = setTimeout(() => {
        emit('update:searchText', filterForm.value.searchText);
      }, 300);
    };

    // Handler for time range changes
    const handleTimeRangeChange = (range) => {
      emit('update:timeRange', range);
    };

    // Handler for level changes
    const handleLevelChange = (level) => {
      emit('update:level', level);
    };

    // Apply all filters
    const applyFilters = () => {
      emit('filter', {
        timeRange: filterForm.value.timeRange,
        level: filterForm.value.level,
        searchText: filterForm.value.searchText
      });
    };

    // Reset all filters
    const resetFilters = () => {
      filterForm.value.timeRange = [];
      filterForm.value.level = '';
      filterForm.value.searchText = '';

      emit('update:timeRange', []);
      emit('update:level', '');
      emit('update:searchText', '');
      emit('reset');
    };

    // Clear individual filters
    const clearTimeRange = () => {
      filterForm.value.timeRange = [];
      emit('update:timeRange', []);
      applyFilters();
    };

    const clearLevel = () => {
      filterForm.value.level = '';
      emit('update:level', '');
      applyFilters();
    };

    const clearSearch = () => {
      filterForm.value.searchText = '';
      emit('update:searchText', '');
      applyFilters();
    };

    // Format time range for display
    const formatTimeRange = (range) => {
      if (!range || range.length !== 2) return '';

      const formatDate = (dateStr) => {
        const date = new Date(dateStr);
        return date.toLocaleString();
      };

      return `${formatDate(range[0])} - ${formatDate(range[1])}`;
    };

    // Watch for prop changes
    watch(() => props.initialTimeRange, (newVal) => {
      filterForm.value.timeRange = newVal.length ? [...newVal] : [];
    });

    watch(() => props.initialLevel, (newVal) => {
      filterForm.value.level = newVal;
    });

    watch(() => props.initialSearchText, (newVal) => {
      filterForm.value.searchText = newVal;
    });

    return {
      filterForm,
      logLevels,
      dateShortcuts,
      hasActiveFilters,
      debouncedSearch,
      handleTimeRangeChange,
      handleLevelChange,
      applyFilters,
      resetFilters,
      clearTimeRange,
      clearLevel,
      clearSearch,
      formatTimeRange
    };
  }
};
</script>

<style scoped>
.log-filter-panel {
  background-color: white;
  border-radius: 4px;
  padding: 16px;
  margin-bottom: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.filter-form {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.el-form-item {
  margin-bottom: 8px;
  margin-right: 16px;
}

.active-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  margin-top: 12px;
  gap: 8px;
}

.active-filters-label {
  font-size: 13px;
  color: #606266;
  margin-right: 8px;
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

.level-info {
  background-color: #409EFF;
}

.level-warn {
  background-color: #E6A23C;
}

.level-error {
  background-color: #F56C6C;
}

.level-debug {
  background-color: #909399;
}

.level-tag-info {
  background-color: rgba(64, 158, 255, 0.1);
  color: #409EFF;
  border-color: rgba(64, 158, 255, 0.2);
}

.level-tag-warn {
  background-color: rgba(230, 162, 60, 0.1);
  color: #E6A23C;
  border-color: rgba(230, 162, 60, 0.2);
}

.level-tag-error {
  background-color: rgba(245, 108, 108, 0.1);
  color: #F56C6C;
  border-color: rgba(245, 108, 108, 0.2);
}

.level-tag-debug {
  background-color: rgba(144, 147, 153, 0.1);
  color: #909399;
  border-color: rgba(144, 147, 153, 0.2);
}
</style>