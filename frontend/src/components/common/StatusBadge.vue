<template>
  <div class="status-badge" :class="badgeClass">
    {{ displayText }}
  </div>
</template>

<script>
export default {
  name: 'StatusBadge',
  props: {
    status: {
      type: String,
      required: true,
      validator: (value) => [
        'success', 'error', 'warning', 'info', 'running',
        'pending', 'completed', 'failed', 'healthy',
        'unhealthy', 'disabled', 'loading'
      ].includes(value)
    },
    text: {
      type: String,
      default: ''
    },
    size: {
      type: String,
      default: 'medium',
      validator: (value) => ['small', 'medium', 'large'].includes(value)
    }
  },
  computed: {
    badgeClass() {
      const statusClasses = {
        success: 'badge-success',
        running: 'badge-success',
        completed: 'badge-success',
        healthy: 'badge-success',
        error: 'badge-error',
        failed: 'badge-error',
        unhealthy: 'badge-error',
        warning: 'badge-warning',
        pending: 'badge-warning',
        info: 'badge-info',
        disabled: 'badge-disabled',
        loading: 'badge-loading'
      };

      return [
        statusClasses[this.status] || 'badge-default',
        `badge-${this.size}`
      ];
    },
    displayText() {
      if (this.text) {
        return this.text;
      }

      const statusLabels = {
        success: 'Success',
        running: 'Running',
        completed: 'Completed',
        healthy: 'Healthy',
        error: 'Error',
        failed: 'Failed',
        unhealthy: 'Unhealthy',
        warning: 'Warning',
        pending: 'Pending',
        info: 'Info',
        disabled: 'Disabled',
        loading: 'Loading'
      };

      return statusLabels[this.status] || this.status;
    }
  }
};
</script>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  font-weight: 500;
  text-transform: capitalize;
}

.badge-small {
  font-size: 0.65rem;
  padding: 2px 6px;
}

.badge-medium {
  font-size: 0.75rem;
  padding: 3px 8px;
}

.badge-large {
  font-size: 0.85rem;
  padding: 4px 10px;
}

.badge-success {
  background-color: rgba(82, 196, 26, 0.15);
  color: #52c41a;
}

.badge-error {
  background-color: rgba(245, 34, 45, 0.15);
  color: #f5222d;
}

.badge-warning {
  background-color: rgba(250, 173, 20, 0.15);
  color: #faad14;
}

.badge-info {
  background-color: rgba(24, 144, 255, 0.15);
  color: #1890ff;
}

.badge-disabled {
  background-color: rgba(140, 140, 140, 0.15);
  color: #8c8c8c;
}

.badge-loading {
  background-color: rgba(24, 144, 255, 0.15);
  color: #1890ff;
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
  100% {
    opacity: 1;
  }
}
</style>