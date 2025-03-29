<template>
  <div class="error-page">
    <div class="error-container">
      <div class="error-icon">⚠️</div>
      <h1 class="error-title">Something Went Wrong</h1>
      <p class="error-message">{{ errorMessage || 'An unexpected error occurred while processing your request.' }}</p>
      <div class="error-details" v-if="errorDetails">
        <h3>Error Details:</h3>
        <pre>{{ errorDetails }}</pre>
      </div>
      <div class="error-actions">
        <router-link to="/" class="home-button">Go to Dashboard</router-link>
        <button @click="reload" class="reload-button">Reload Page</button>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'ErrorView',
  data() {
    return {
      errorMessage: this.$route.query.message || '',
      errorDetails: this.$route.query.details || ''
    };
  },
  methods: {
    reload() {
      window.location.reload();
    }
  }
};
</script>

<style scoped>
.error-page {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 80vh;
}

.error-container {
  text-align: center;
  padding: 40px;
  background-color: white;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
  max-width: 600px;
  width: 100%;
}

.error-icon {
  font-size: 64px;
  margin-bottom: 16px;
}

.error-title {
  font-size: 28px;
  margin-bottom: 16px;
  color: #303133;
}

.error-message {
  font-size: 16px;
  color: #606266;
  margin-bottom: 24px;
}

.error-details {
  text-align: left;
  background-color: #f5f7fa;
  padding: 16px;
  border-radius: 4px;
  margin-bottom: 24px;
  overflow-x: auto;
}

.error-details h3 {
  margin-top: 0;
  margin-bottom: 8px;
  font-size: 16px;
  color: #303133;
}

.error-details pre {
  margin: 0;
  font-family: monospace;
  font-size: 14px;
  color: #606266;
  white-space: pre-wrap;
}

.error-actions {
  display: flex;
  justify-content: center;
  gap: 16px;
}

.home-button, .reload-button {
  padding: 10px 20px;
  border-radius: 4px;
  font-size: 16px;
  cursor: pointer;
  transition: all 0.3s;
}

.home-button {
  background-color: #1890ff;
  color: white;
  border: none;
  text-decoration: none;
}

.home-button:hover {
  background-color: #40a9ff;
}

.reload-button {
  background-color: transparent;
  color: #606266;
  border: 1px solid #dcdfe6;
}

.reload-button:hover {
  color: #1890ff;
  border-color: #1890ff;
}
</style>