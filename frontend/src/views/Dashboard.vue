<template>
  <div class="dashboard-page">
    <div class="page-header">
      <h1>Dashboard</h1>
      <div class="header-actions">
        <el-button
          type="primary"
          icon="el-icon-refresh"
          :loading="isLoading"
          @click="refreshDashboard"
          size="small"
        >
          Refresh
        </el-button>
      </div>
    </div>

    <!-- System Overview Cards -->
    <div class="overview-cards">
      <el-card class="stat-card">
        <div class="stat-card-content">
          <div class="stat-value">{{ totalJobs }}</div>
          <div class="stat-label">Total Jobs</div>
        </div>
        <div class="stat-icon">
          <i class="el-icon-tickets"></i>
        </div>
      </el-card>

      <el-card class="stat-card">
        <div class="stat-card-content">
          <div class="stat-value">{{ runningJobs }}</div>
          <div class="stat-label">Running Jobs</div>
        </div>
        <div class="stat-icon running-icon">
          <i class="el-icon-video-play"></i>
        </div>
      </el-card>

      <el-card class="stat-card">
        <div class="stat-card-content">
          <div class="stat-value">{{ activeWorkers }}</div>
          <div class="stat-label">Active Workers</div>
        </div>
        <div class="stat-icon worker-icon">
          <i class="el-icon-cpu"></i>
        </div>
      </el-card>

      <el-card class="stat-card">
        <div class="stat-card-content">
          <div class="stat-value">{{ successRate }}%</div>
          <div class="stat-label">Success Rate</div>
        </div>
        <div class="stat-icon success-icon">
          <i class="el-icon-check"></i>
        </div>
      </el-card>
    </div>

    <!-- System Health and Execution History -->
    <div class="dashboard-main">
      <div class="dashboard-section">
        <el-card class="execution-history">
          <template #header>
            <div class="section-header">
              <h3>Job Execution History</h3>
              <div class="time-range-selector">
                <el-select v-model="timeRange" size="small" @change="updateChart">
                  <el-option label="Last 7 days" value="7" />
                  <el-option label="Last 14 days" value="14" />
                  <el-option label="Last 30 days" value="30" />
                </el-select>
              </div>
            </div>
          </template>

          <div v-if="isLoading" class="chart-loading">
            <LoadingSpinner text="Loading chart data..." />
          </div>
          <div v-else class="chart-container">
            <div class="execution-stats">
              <div class="execution-stat-item">
                <div class="stat-label">Success</div>
                <div class="stat-value success-text">{{ executionStats.success }}</div>
              </div>
              <div class="execution-stat-item">
                <div class="stat-label">Failed</div>
                <div class="stat-value error-text">{{ executionStats.failed }}</div>
              </div>
              <div class="execution-stat-item">
                <div class="stat-label">Total</div>
                <div class="stat-value">{{ executionStats.total }}</div>
              </div>
              <div class="execution-stat-item">
                <div class="stat-label">Avg. Duration</div>
                <div class="stat-value">{{ executionStats.avgDuration }}</div>
              </div>
            </div>

            <div class="execution-chart" ref="executionChart">
              <!-- Chart will be rendered here -->
              <div v-if="noChartData" class="no-chart-data">
                <p>No execution data available for the selected period.</p>
              </div>
            </div>
          </div>
        </el-card>
      </div>

      <div class="dashboard-aside">
        <el-card class="system-health">
          <template #header>
            <div class="section-header">
              <h3>System Health</h3>
              <StatusBadge :status="systemHealth.status" size="small" />
            </div>
          </template>

          <div class="health-details">
            <div class="health-item">
              <span class="health-label">Server Uptime</span>
              <span class="health-value">{{ systemHealth.uptime }}</span>
            </div>
            <div class="health-item">
              <span class="health-label">Version</span>
              <span class="health-value">{{ systemHealth.version }}</span>
            </div>
            <div class="health-item">
              <span class="health-label">API Status</span>
              <span class="health-value">
                <StatusBadge :status="systemHealth.apiStatus" size="small" />
              </span>
            </div>
            <div class="health-item">
              <span class="health-label">Database Status</span>
              <span class="health-value">
                <StatusBadge :status="systemHealth.dbStatus" size="small" />
              </span>
            </div>
          </div>
        </el-card>

        <el-card class="worker-status">
          <template #header>
            <div class="section-header">
              <h3>Worker Status</h3>
              <router-link to="/workers" class="view-all-link">
                View All
              </router-link>
            </div>
          </template>

          <div class="worker-stats">
            <div class="worker-stat-item">
              <div class="worker-stat-circle" :class="'worker-healthy'">
                <span>{{ workerStats.healthy }}</span>
              </div>
              <div class="worker-stat-label">Healthy</div>
            </div>
            <div class="worker-stat-item">
              <div class="worker-stat-circle" :class="'worker-unhealthy'">
                <span>{{ workerStats.unhealthy }}</span>
              </div>
              <div class="worker-stat-label">Unhealthy</div>
            </div>
            <div class="worker-stat-item">
              <div class="worker-stat-circle" :class="'worker-offline'">
                <span>{{ workerStats.offline }}</span>
              </div>
              <div class="worker-stat-label">Offline</div>
            </div>
          </div>
        </el-card>
      </div>
    </div>

    <!-- Recent Activity -->
    <div class="dashboard-section">
      <el-card class="recent-activity">
        <template #header>
          <div class="section-header">
            <h3>Recent Activity</h3>
            <router-link to="/logs" class="view-all-link">
              View All Logs
            </router-link>
          </div>
        </template>

        <div v-if="isLoading" class="loading-activity">
          <LoadingSpinner text="Loading recent activity..." />
        </div>
        <div v-else-if="recentLogs.length === 0" class="no-activity">
          <i class="el-icon-info"></i>
          <p>No recent activity recorded.</p>
        </div>
        <div v-else class="activity-list">
          <div v-for="(log, index) in recentLogs" :key="index" class="activity-item">
            <div class="activity-time">{{ formatTime(log.timestamp) }}</div>
            <div class="activity-status">
              <StatusBadge :status="getStatusFromLevel(log.level)" size="small" />
            </div>
            <div class="activity-message">{{ log.message }}</div>
            <div class="activity-source">{{ log.source }}</div>
          </div>
        </div>
      </el-card>
    </div>

    <!-- Quick Actions -->
    <div class="quick-actions">
      <el-card class="action-card" shadow="hover" @click="goToCreateJob">
        <i class="el-icon-plus"></i>
        <span>Create New Job</span>
      </el-card>

      <el-card class="action-card" shadow="hover" @click="goToJobs">
        <i class="el-icon-tickets"></i>
        <span>Manage Jobs</span>
      </el-card>

      <el-card class="action-card" shadow="hover" @click="goToWorkers">
        <i class="el-icon-cpu"></i>
        <span>Manage Workers</span>
      </el-card>

      <el-card class="action-card" shadow="hover" @click="goToLogs">
        <i class="el-icon-document"></i>
        <span>View Logs</span>
      </el-card>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import { useStore } from 'vuex';
import { useRouter } from 'vue-router';
import StatusBadge from '@/components/common/StatusBadge.vue';
import LoadingSpinner from '@/components/common/LoadingSpinner.vue';

export default {
  name: 'Dashboard',
  components: {
    StatusBadge,
    LoadingSpinner
  },
  setup() {
    const store = useStore();
    const router = useRouter();
    const executionChart = ref(null);
    const isLoading = ref(true);
    const timeRange = ref('7');
    const autoRefreshTimer = ref(null);
    const noChartData = ref(false);

    // Dashboard statistics
    const totalJobs = ref(0);
    const runningJobs = ref(0);
    const activeWorkers = ref(0);
    const successRate = ref(0);

    // System health data
    const systemHealth = ref({
      status: 'loading',
      uptime: '0d 0h 0m',
      version: '1.0.0',
      apiStatus: 'loading',
      dbStatus: 'loading'
    });

    // Execution stats
    const executionStats = ref({
      success: 0,
      failed: 0,
      total: 0,
      avgDuration: '0s'
    });

    // Worker statistics
    const workerStats = ref({
      healthy: 0,
      unhealthy: 0,
      offline: 0
    });

    // Recent activity logs
    const recentLogs = ref([]);

    // Charts data
    const chartData = ref({
      dates: [],
      success: [],
      failed: []
    });

    // Get data from Vuex store (if available)
    const workersByStatus = computed(() => store.getters['workers/workersByStatus'] || {
      healthy: 0,
      unhealthy: 0,
      offline: 0,
      total: 0
    });

    const jobsByStatus = computed(() => store.getters['jobs/jobsByStatus'] || {
      enabled: 0,
      disabled: 0,
      running: 0,
      total: 0
    });

    // Methods
    const refreshDashboard = async () => {
      isLoading.value = true;

      try {
        // Load job statistics
        await store.dispatch('jobs/fetchJobs', { page: 1, pageSize: 1 });

        // Load worker statistics
        await store.dispatch('workers/fetchWorkers', { page: 1, pageSize: 1 });

        // In a real application, these would come from API calls
        // For now, we'll simulate with some mock data
        await fetchSystemStatus();
        await fetchExecutionHistory();
        await fetchRecentActivity();

        // Update statistics from store
        totalJobs.value = jobsByStatus.value.total || 0;
        runningJobs.value = jobsByStatus.value.running || 0;

        workerStats.value = {
          healthy: workersByStatus.value.healthy || 0,
          unhealthy: workersByStatus.value.unhealthy || 0,
          offline: workersByStatus.value.offline || 0
        };

        activeWorkers.value = workerStats.value.healthy;

        // Render chart with the new data
        renderChart();
      } catch (error) {
        console.error('Failed to refresh dashboard:', error);
      } finally {
        isLoading.value = false;
      }
    };

    const fetchSystemStatus = async () => {
      // In a real application, this would be an API call
      // For now, simulate with mock data
      systemHealth.value = {
        status: 'healthy',
        uptime: '3d 7h 42m',
        version: '1.0.0',
        apiStatus: 'healthy',
        dbStatus: 'healthy'
      };
    };

    const fetchExecutionHistory = async () => {
      // In a real application, this would be an API call
      // For now, simulate with mock data
      const days = parseInt(timeRange.value);

      const dates = [];
      const successData = [];
      const failedData = [];

      // Generate random data for the past N days
      const now = new Date();
      let totalSuccess = 0;
      let totalFailed = 0;

      for (let i = days - 1; i >= 0; i--) {
        const date = new Date(now);
        date.setDate(date.getDate() - i);
        dates.push(date.toLocaleDateString());

        const success = Math.floor(Math.random() * 20) + 5;
        const failed = Math.floor(Math.random() * 5);

        successData.push(success);
        failedData.push(failed);

        totalSuccess += success;
        totalFailed += failed;
      }

      chartData.value = {
        dates,
        success: successData,
        failed: failedData
      };

      // Update execution stats
      const total = totalSuccess + totalFailed;
      executionStats.value = {
        success: totalSuccess,
        failed: totalFailed,
        total: total,
        avgDuration: `${Math.floor(Math.random() * 50) + 10}s`
      };

      // Calculate success rate
      successRate.value = total > 0 ? Math.round((totalSuccess / total) * 100) : 0;

      noChartData.value = total === 0;
    };

    const fetchRecentActivity = async () => {
      // In a real application, this would be an API call
      // For now, simulate with mock data
      const activities = [
        {
          timestamp: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
          level: 'info',
          message: 'Job "Daily Backup" completed successfully',
          source: 'job-scheduler'
        },
        {
          timestamp: new Date(Date.now() - 1000 * 60 * 30).toISOString(),
          level: 'error',
          message: 'Job "Data Processing" failed - Exit code 1',
          source: 'job-executor'
        },
        {
          timestamp: new Date(Date.now() - 1000 * 60 * 55).toISOString(),
          level: 'info',
          message: 'New worker "worker-03" connected',
          source: 'worker-manager'
        },
        {
          timestamp: new Date(Date.now() - 1000 * 60 * 120).toISOString(),
          level: 'warn',
          message: 'Job "Analytics Report" took longer than expected',
          source: 'job-monitor'
        },
        {
          timestamp: new Date(Date.now() - 1000 * 60 * 240).toISOString(),
          level: 'info',
          message: 'System configuration updated',
          source: 'system'
        }
      ];

      recentLogs.value = activities;
    };

    const renderChart = () => {
      // In a real implementation, you would use a charting library like Chart.js
      // For this example, we'll just log the data that would be rendered
      console.log('Chart data:', chartData.value);

      // If you want to implement a real chart, you would do something like:
      // if (executionChart.value && window.Chart) {
      //   const ctx = executionChart.value.getContext('2d');
      //   new window.Chart(ctx, {
      //     type: 'bar',
      //     data: {
      //       labels: chartData.value.dates,
      //       datasets: [...]
      //     },
      //     options: { ... }
      //   });
      // }
    };

    const updateChart = () => {
      fetchExecutionHistory().then(() => {
        renderChart();
      });
    };

    const formatTime = (timestamp) => {
      if (!timestamp) return '';

      const date = new Date(timestamp);
      const now = new Date();
      const diff = now - date;

      // Format as relative time if recent, otherwise show date
      if (diff < 60 * 1000) {
        return 'Just now';
      } else if (diff < 60 * 60 * 1000) {
        const minutes = Math.floor(diff / (60 * 1000));
        return `${minutes} min ago`;
      } else if (diff < 24 * 60 * 60 * 1000) {
        const hours = Math.floor(diff / (60 * 60 * 1000));
        return `${hours} hours ago`;
      } else {
        return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
      }
    };

    const getStatusFromLevel = (level) => {
      switch (level) {
        case 'error':
          return 'error';
        case 'warn':
          return 'warning';
        case 'info':
          return 'info';
        case 'debug':
          return 'disabled';
        default:
          return 'info';
      }
    };

    // Navigation methods
    const goToCreateJob = () => {
      router.push('/jobs/create');
    };

    const goToJobs = () => {
      router.push('/jobs');
    };

    const goToWorkers = () => {
      router.push('/workers');
    };

    const goToLogs = () => {
      router.push('/logs');
    };

    // Set up auto-refresh
    const startAutoRefresh = () => {
      // Refresh dashboard every 30 seconds
      autoRefreshTimer.value = setInterval(() => {
        refreshDashboard();
      }, 30000);
    };

    const stopAutoRefresh = () => {
      if (autoRefreshTimer.value) {
        clearInterval(autoRefreshTimer.value);
        autoRefreshTimer.value = null;
      }
    };

    // Lifecycle hooks
    onMounted(async () => {
      await refreshDashboard();
      startAutoRefresh();
    });

    onBeforeUnmount(() => {
      stopAutoRefresh();
    });

    return {
      executionChart,
      isLoading,
      timeRange,
      totalJobs,
      runningJobs,
      activeWorkers,
      successRate,
      systemHealth,
      executionStats,
      workerStats,
      recentLogs,
      noChartData,
      refreshDashboard,
      updateChart,
      formatTime,
      getStatusFromLevel,
      goToCreateJob,
      goToJobs,
      goToWorkers,
      goToLogs
    };
  }
};
</script>

<style scoped>
.dashboard-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.overview-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 20px;
}

.stat-card {
  position: relative;
  height: 120px;
  display: flex;
  align-items: center;
  overflow: hidden;
}

.stat-card-content {
  flex: 1;
  z-index: 1;
}

.stat-value {
  font-size: 36px;
  font-weight: 600;
  margin-bottom: 8px;
}

.stat-label {
  font-size: 14px;
  color: #606266;
}

.stat-icon {
  position: absolute;
  right: 20px;
  font-size: 64px;
  opacity: 0.2;
  color: #409EFF;
}

.running-icon {
  color: #67C23A;
}

.worker-icon {
  color: #409EFF;
}

.success-icon {
  color: #67C23A;
}

.dashboard-main {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 24px;
}

@media (max-width: 1200px) {
  .dashboard-main {
    grid-template-columns: 1fr;
  }
}

.dashboard-section {
  margin-bottom: 0;
}

.dashboard-aside {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.section-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
}

.view-all-link {
  font-size: 14px;
  color: #409EFF;
  text-decoration: none;
}

.view-all-link:hover {
  text-decoration: underline;
}

.execution-stats {
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  margin-bottom: 20px;
  gap: 16px;
}

.execution-stat-item {
  text-align: center;
  min-width: 100px;
}

.execution-chart {
  height: 240px;
  background-color: #f9f9f9;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.no-chart-data {
  color: #909399;
  text-align: center;
  padding: 40px 0;
}

.health-details {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.health-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.health-label {
  color: #606266;
}

.health-value {
  font-weight: 500;
}

.worker-stats {
  display: flex;
  justify-content: space-around;
  margin: 16px 0;
}

.worker-stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.worker-stat-circle {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  font-weight: 600;
  color: white;
}

.worker-healthy {
  background-color: #67C23A;
}

.worker-unhealthy {
  background-color: #E6A23C;
}

.worker-offline {
  background-color: #909399;
}

.worker-stat-label {
  font-size: 14px;
  color: #606266;
}

.activity-list {
  max-height: 300px;
  overflow-y: auto;
}

.activity-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid #EBEEF5;
}

.activity-item:last-child {
  border-bottom: none;
}

.activity-time {
  width: 120px;
  font-size: 13px;
  color: #909399;
}

.activity-message {
  flex: 1;
}

.activity-source {
  font-size: 13px;
  color: #909399;
  width: 120px;
  text-align: right;
}

.no-activity {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 40px 0;
  color: #909399;
}

.no-activity i {
  font-size: 32px;
  margin-bottom: 16px;
}

.loading-activity, .chart-loading {
  padding: 40px 0;
  display: flex;
  justify-content: center;
}

.quick-actions {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 24px;
}

.action-card {
  height: 100px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: transform 0.2s;
}

.action-card:hover {
  transform: translateY(-5px);
}

.action-card i {
  font-size: 32px;
  margin-bottom: 12px;
  color: #409EFF;
}

.success-text {
  color: #67C23A;
}

.error-text {
  color: #F56C6C;
}

.time-range-selector {
  width: 130px;
}
</style>