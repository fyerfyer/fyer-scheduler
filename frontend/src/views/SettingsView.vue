<template>
  <div class="settings-page">
    <div class="page-header">
      <h1>Settings</h1>
    </div>

    <el-card class="settings-card">
      <template #header>
        <div class="card-header">
          <h3>System Configuration</h3>
        </div>
      </template>

      <el-form :model="settingsForm" label-position="top" class="settings-form">
        <el-form-item label="System Name">
          <el-input v-model="settingsForm.systemName" placeholder="Enter a name for this system"></el-input>
        </el-form-item>

        <el-form-item label="Default Worker Tags">
          <el-input v-model="settingsForm.defaultWorkerTags" placeholder="tag1=value1,tag2=value2"></el-input>
          <div class="form-hint">Default tags applied to new workers (format: key1=value1,key2=value2)</div>
        </el-form-item>

        <el-form-item label="Log Retention Period (days)">
          <el-input-number v-model="settingsForm.logRetentionDays" :min="1" :max="365"></el-input-number>
          <div class="form-hint">Number of days to keep log files before automatic cleanup</div>
        </el-form-item>

        <el-form-item label="Master Endpoint">
          <el-input v-model="settingsForm.masterEndpoint" disabled></el-input>
          <div class="form-hint">System generated endpoint value</div>
        </el-form-item>

        <div class="form-actions">
          <el-button type="primary" @click="saveSettings" :loading="isSaving">Save Settings</el-button>
          <el-button @click="resetForm">Reset</el-button>
        </div>
      </el-form>
    </el-card>

    <el-card class="settings-card">
      <template #header>
        <div class="card-header">
          <h3>Notification Settings</h3>
        </div>
      </template>

      <p class="settings-info">Configure how and when you want to receive notifications about system events.</p>

      <el-form :model="notificationForm" label-position="top" class="settings-form">
        <el-form-item label="Email Notifications">
          <el-switch v-model="notificationForm.emailEnabled"></el-switch>
        </el-form-item>

        <el-form-item label="Email Recipients" v-if="notificationForm.emailEnabled">
          <el-input v-model="notificationForm.emailRecipients" placeholder="email1@example.com, email2@example.com"></el-input>
          <div class="form-hint">Comma separated list of email addresses</div>
        </el-form-item>

        <el-form-item label="Webhook Notifications">
          <el-switch v-model="notificationForm.webhookEnabled"></el-switch>
        </el-form-item>

        <el-form-item label="Webhook URL" v-if="notificationForm.webhookEnabled">
          <el-input v-model="notificationForm.webhookUrl" placeholder="https://example.com/webhook"></el-input>
        </el-form-item>

        <div class="form-actions">
          <el-button type="primary" @click="saveNotificationSettings" :loading="isSaving">Save Settings</el-button>
          <el-button @click="resetNotificationForm">Reset</el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<script>
import { ref } from 'vue';

export default {
  name: 'SettingsView',
  setup() {
    // Settings form
    const defaultSettings = {
      systemName: 'Fyer Scheduler',
      defaultWorkerTags: 'env=production,type=standard',
      logRetentionDays: 30,
      masterEndpoint: 'http://localhost:8080'
    };
    
    const settingsForm = ref({...defaultSettings});
    
    // Notification form
    const defaultNotificationSettings = {
      emailEnabled: false,
      emailRecipients: '',
      webhookEnabled: false,
      webhookUrl: ''
    };
    
    const notificationForm = ref({...defaultNotificationSettings});
    
    // Loading state
    const isSaving = ref(false);
    
    // Methods
    const saveSettings = async () => {
      isSaving.value = true;
      
      try {
        // API call would go here
        console.log('Saving settings:', settingsForm.value);
        
        // Simulate API delay
        await new Promise(resolve => setTimeout(resolve, 1000));
        
        // Show success message
        alert('Settings saved successfully!');
      } catch (error) {
        console.error('Failed to save settings:', error);
        alert('Failed to save settings');
      } finally {
        isSaving.value = false;
      }
    };
    
    const resetForm = () => {
      settingsForm.value = {...defaultSettings};
    };
    
    const saveNotificationSettings = async () => {
      isSaving.value = true;
      
      try {
        // API call would go here
        console.log('Saving notification settings:', notificationForm.value);
        
        // Simulate API delay
        await new Promise(resolve => setTimeout(resolve, 1000));
        
        // Show success message
        alert('Notification settings saved successfully!');
      } catch (error) {
        console.error('Failed to save notification settings:', error);
        alert('Failed to save notification settings');
      } finally {
        isSaving.value = false;
      }
    };
    
    const resetNotificationForm = () => {
      notificationForm.value = {...defaultNotificationSettings};
    };
    
    return {
      settingsForm,
      notificationForm,
      isSaving,
      saveSettings,
      resetForm,
      saveNotificationSettings,
      resetNotificationForm
    };
  }
};
</script>

<style scoped>
.settings-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
  max-width: 800px;
  margin: 0 auto;
}

.settings-card {
  margin-bottom: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
}

.settings-form {
  max-width: 600px;
}

.form-hint {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.form-actions {
  margin-top: 24px;
  display: flex;
  gap: 12px;
}

.settings-info {
  margin-bottom: 16px;
  color: #606266;
}
</style>