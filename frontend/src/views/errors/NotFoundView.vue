<template>
  <div class="not-found-page">
    <div class="error-container">
      <div class="error-code">404</div>
      <h1 class="error-title">Page Not Found</h1>
      <p class="error-message">The page you're looking for doesn't exist or has been moved.</p>
      <div class="error-actions">
        <router-link to="/" class="home-button">Go to Dashboard</router-link>
        <button @click="goBack" class="back-button">Go Back</button>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'NotFoundView',
  methods: {
    goBack() {
      this.$router.go(-1);
    }
  }
};
</script>

<style scoped>
.not-found-page {
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
  max-width: 500px;
  width: 100%;
}

.error-code {
  font-size: 80px;
  font-weight: bold;
  color: #1890ff;
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
  margin-bottom: 32px;
}

.error-actions {
  display: flex;
  justify-content: center;
  gap: 16px;
}

.home-button, .back-button {
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

.back-button {
  background-color: transparent;
  color: #606266;
  border: 1px solid #dcdfe6;
}

.back-button:hover {
  color: #1890ff;
  border-color: #1890ff;
}
</style>