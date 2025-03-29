<template>
  <div class="job-card" :class="{ 'job-disabled': !job.enabled }">
    <div class="job-card-header">
      <div class="job-name-container">
        <StatusBadge :status="jobStatus" size="small" />
        <router-link :to="`/jobs/${job.id}`" class="job-name">{{ job.name }}</router-link>
      </div>
      <div class="job-actions">
        <el-dropdown trigger="click" @command="handleAction">
          <span class="job-action-button">
            <i class="el-icon-more"></i>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item :command="'run'" :disabled="!canRun">Run Now</el-dropdown-item>
              <el-dropdown-item :command="'edit'">Edit</el-dropdown-item>
              <el-dropdown-item :command="'log'">View Logs</el-dropdown-item>
              <el-dropdown-item :command="'toggle'" :disabled="job.status === 'running'">
                {{ job.enabled ? 'Disable' : 'Enable' }}
              </el-dropdown-item>
              <el-dropdown-item :command="'delete'" :disabled="job.status === 'running'">
                Delete
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div class="job-card-body">
      <div class="job-description" v-if="job.description">{{ job.description }}</div>
      <div class="job-command">
        <code>{{ job.command }} {{ job.args ? job.args.join(' ') : '' }}</code>
      </div>

      <div class="job-meta">
        <div class="job-timing">
          <i class="el-icon-time"></i>
          <span v-if="job.cron_expr">{{ job.cron_expr }}</span>
          <span v-else>Manual trigger only</span>
        </div>

        <div class="job-last-run" v-if="job.last_execution">
          <i class="el-icon-date"></i>
          Last run: {{ formatDate(job.last_execution.start_time) }}
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import StatusBadge from '@/components/common/StatusBadge.vue';
import { computed } from 'vue';
import { useRouter } from 'vue-router';
import { useStore } from 'vuex';

export default {
  name: 'JobCard',
  components: {
    StatusBadge
  },
  props: {
    job: {
      type: Object,
      required: true
    }
  },
  setup(props) {
    const store = useStore();
    const router = useRouter();

    // Computed properties
    const jobStatus = computed(() => {
      if (!props.job.enabled) {
        return 'disabled';
      }
      return props.job.status || 'pending';
    });

    const canRun = computed(() => {
      return props.job.enabled && props.job.status !== 'running';
    });

    // Methods
    const formatDate = (dateString) => {
      if (!dateString) return 'Never';

      const date = new Date(dateString);
      return date.toLocaleString();
    };

    const handleAction = async (command) => {
      switch (command) {
        case 'run':
          try {
            await store.dispatch('jobs/triggerJob', props.job.id);
            // Show success notification
          } catch (error) {
            // Show error notification
            console.error('Failed to trigger job', error);
          }
          break;

        case 'edit':
          router.push(`/jobs/${props.job.id}/edit`);
          break;

        case 'log':
          router.push(`/logs/jobs/${props.job.id}`);
          break;

        case 'toggle':
          try {
            if (props.job.enabled) {
              await store.dispatch('jobs/disableJob', props.job.id);
            } else {
              await store.dispatch('jobs/enableJob', props.job.id);
            }
            // Show success notification
          } catch (error) {
            // Show error notification
            console.error('Failed to toggle job', error);
          }
          break;

        case 'delete':
          // Show confirmation dialog
          if (confirm('Are you sure you want to delete this job?')) {
            try {
              await store.dispatch('jobs/deleteJob', props.job.id);
              // Show success notification
            } catch (error) {
              // Show error notification
              console.error('Failed to delete job', error);
            }
          }
          break;
      }
    };

    return {
      jobStatus,
      canRun,
      formatDate,
      handleAction
    };
  }
};
</script>

<style scoped>
.job-card {
  background-color: #fff;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  padding: 16px;
  margin-bottom: 16px;
  transition: all 0.3s;
}

.job-card:hover {
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
}

.job-disabled {
  opacity: 0.7;
}

.job-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.job-name-container {
  display: flex;
  align-items: center;
}

.job-name {
  margin-left: 8px;
  font-weight: 500;
  font-size: 16px;
  color: #303133;
  text-decoration: none;
}

.job-name:hover {
  color: #1890ff;
  text-decoration: underline;
}

.job-action-button {
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
}

.job-action-button:hover {
  background-color: #f5f5f5;
}

.job-card-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.job-description {
  color: #606266;
  font-size: 14px;
  margin-bottom: 8px;
}

.job-command {
  background-color: #f5f7fa;
  padding: 8px 12px;
  border-radius: 4px;
  margin-bottom: 8px;
  font-size: 13px;
  overflow-x: auto;
}

.job-meta {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #909399;
}

.job-timing, .job-last-run {
  display: flex;
  align-items: center;
  gap: 4px;
}
</style>