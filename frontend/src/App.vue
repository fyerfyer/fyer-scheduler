<template>
  <div class="app-container">
    <AppHeader />

    <div class="main-container">
      <AppSidebar :version="version" />

      <main class="app-main">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script>
import AppHeader from './components/common/AppHeader.vue';
import AppSidebar from './components/common/AppSidebar.vue';

export default {
  name: 'App',
  components: {
    AppHeader,
    AppSidebar
  },
  data() {
    return {
      version: '1.0.0'
    };
  }
};
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
  background-color: #f5f7fa;
  color: #333;
}

.app-container {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

.main-container {
  display: flex;
  flex: 1;
}

.app-main {
  flex: 1;
  padding: 20px;
  overflow-y: auto;
}

@media (max-width: 768px) {
  .main-container {
    flex-direction: column;
  }
  
.app-main {
    padding: 10px;
  }
}
</style>