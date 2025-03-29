<template>
  <aside class="app-sidebar">
    <nav>
      <router-link to="/" class="nav-item" exact>
        <span class="nav-icon">📊</span>
        <span class="nav-text">Dashboard</span>
      </router-link>
      <router-link to="/jobs" class="nav-item">
        <span class="nav-icon">📋</span>
        <span class="nav-text">Jobs</span>
      </router-link>
      <router-link to="/logs" class="nav-item">
        <span class="nav-icon">📄</span>
        <span class="nav-text">Logs</span>
      </router-link>
      <router-link to="/workers" class="nav-item">
        <span class="nav-icon">🖥️</span>
        <span class="nav-text">Workers</span>
      </router-link>
    </nav>
    <div class="sidebar-footer">
      <div class="version">v{{ version }}</div>
    </div>
  </aside>
</template>

<script>
export default {
  name: 'AppSidebar',
  props: {
    version: {
      type: String,
      default: '1.0.0'
    }
  }
};
</script>

<style scoped>
.app-sidebar {
  width: 220px;
  background-color: #304156;
  color: white;
  padding: 20px 0;
  display: flex;
  flex-direction: column;
  height: 100%;
}

nav {
  flex: 1;
}

.nav-item {
  display: flex;
  align-items: center;
  padding: 12px 20px;
  color: #bfcbd9;
  text-decoration: none;
  transition: all 0.3s;
}

.nav-item:hover, .router-link-active {
  background-color: #263445;
  color: white;
}

.nav-icon {
  margin-right: 10px;
}

.sidebar-footer {
  padding: 16px;
  font-size: 0.75rem;
  color: #8a9cb5;
  border-top: 1px solid rgba(255, 255, 255, 0.07);
}
</style>