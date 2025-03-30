<template>
  <div class="job-form">
    <el-form
      ref="jobFormRef"
      :model="jobForm"
      :rules="rules"
      label-position="top"
      @submit.prevent="submitForm"
    >
      <!-- Basic information section -->
      <el-card class="form-section">
        <template #header>
          <div class="card-header">
            <h3>Basic Information</h3>
          </div>
        </template>

        <el-form-item label="Job Name" prop="name">
          <el-input v-model="jobForm.name" placeholder="Enter job name"></el-input>
        </el-form-item>

        <el-form-item label="Description" prop="description">
          <el-input
            v-model="jobForm.description"
            type="textarea"
            :rows="3"
            placeholder="Job description (optional)"
          ></el-input>
        </el-form-item>

        <el-form-item label="Enabled">
          <el-switch v-model="jobForm.enabled"></el-switch>
          <span class="form-hint">{{ jobForm.enabled ? 'Job will be scheduled to run' : 'Job will not run until enabled' }}</span>
        </el-form-item>
      </el-card>

      <!-- Command section -->
      <el-card class="form-section">
        <template #header>
          <div class="card-header">
            <h3>Command</h3>
          </div>
        </template>

        <el-form-item label="Command" prop="command">
          <el-input v-model="jobForm.command" placeholder="Enter command to execute"></el-input>
        </el-form-item>

        <el-form-item label="Arguments">
          <div class="args-container">
            <div v-for="(arg, index) in jobForm.args" :key="index" class="arg-item">
              <el-input
                v-model="jobForm.args[index]"
                placeholder="Argument"
              >
                <template #append>
                  <el-button @click="removeArg(index)" type="danger" icon="el-icon-delete"></el-button>
                </template>
              </el-input>
            </div>
            <el-button @click="addArg" type="primary" plain>Add Argument</el-button>
          </div>
        </el-form-item>

        <el-form-item label="Working Directory" prop="workDir">
          <el-input v-model="jobForm.workDir" placeholder="Working directory (optional)"></el-input>
        </el-form-item>
      </el-card>

      <!-- Schedule section -->
      <el-card class="form-section">
        <template #header>
          <div class="card-header">
            <h3>Schedule</h3>
          </div>
        </template>

        <el-form-item label="Cron Expression" prop="cronExpr">
          <el-input v-model="jobForm.cronExpr" placeholder="e.g. 0 0 * * * (daily at midnight)">
            <template #append>
              <el-popover placement="top" width="400" trigger="click">
                <template #reference>
                  <el-button>Help</el-button>
                </template>
                <h4>Cron Expression Format</h4>
                <p>Cron expressions use the following format: <code>* * * * *</code></p>
                <table class="cron-help-table">
                  <thead>
                    <tr>
                      <th>Field</th>
                      <th>Values</th>
                      <th>Example</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr>
                      <td>Minute</td>
                      <td>0-59</td>
                      <td>0 = top of the hour</td>
                    </tr>
                    <tr>
                      <td>Hour</td>
                      <td>0-23</td>
                      <td>0 = midnight</td>
                    </tr>
                    <tr>
                      <td>Day of Month</td>
                      <td>1-31</td>
                      <td>15 = 15th day</td>
                    </tr>
                    <tr>
                      <td>Month</td>
                      <td>1-12</td>
                      <td>1 = January</td>
                    </tr>
                    <tr>
                      <td>Day of Week</td>
                      <td>0-6</td>
                      <td>0 = Sunday</td>
                    </tr>
                  </tbody>
                </table>
                <p>Common examples:</p>
                <ul>
                  <li><code>0 0 * * *</code> - Daily at midnight</li>
                  <li><code>0 */2 * * *</code> - Every 2 hours</li>
                  <li><code>*/15 * * * *</code> - Every 15 minutes</li>
                  <li><code>0 9 * * 1-5</code> - Weekdays at 9 AM</li>
                </ul>
                <p>Leave empty for manual execution only.</p>
              </el-popover>
            </template>
          </el-input>
          <span class="form-hint">Leave empty for manual execution only</span>
        </el-form-item>
      </el-card>

      <!-- Environment variables section -->
      <el-card class="form-section">
        <template #header>
          <div class="card-header">
            <h3>Environment Variables</h3>
          </div>
        </template>

        <div v-for="(value, key, index) in jobForm.env" :key="index" class="env-var-item">
          <div class="env-var-row">
            <el-input
              v-model="envKeys[index]"
              @input="updateEnvKey(index, key)"
              placeholder="Key"
              class="env-key-input"
            ></el-input>
            <span class="env-equals">=</span>
            <el-input
              v-model="jobForm.env[envKeys[index]]"
              placeholder="Value"
              class="env-value-input"
            ></el-input>
            <el-button @click="removeEnvVar(key)" type="danger" circle icon="el-icon-delete"></el-button>
          </div>
        </div>
        <el-button @click="addEnvVar" type="primary" plain class="add-env-btn">Add Environment Variable</el-button>
      </el-card>

      <!-- Advanced settings section -->
      <el-card class="form-section">
        <template #header>
          <div class="card-header">
            <h3>Advanced Settings</h3>
          </div>
        </template>

        <el-form-item label="Timeout (seconds)" prop="timeout">
          <el-input-number
            v-model="jobForm.timeout"
            :min="0"
            :controls="false"
            placeholder="No timeout"
          ></el-input-number>
          <span class="form-hint">0 means no timeout</span>
        </el-form-item>

        <el-form-item label="Max Retry Attempts" prop="maxRetry">
          <el-input-number
            v-model="jobForm.maxRetry"
            :min="0"
            :controls="false"
            placeholder="No retries"
          ></el-input-number>
          <span class="form-hint">0 means no retries</span>
        </el-form-item>

        <el-form-item label="Retry Delay (seconds)" prop="retryDelay">
          <el-input-number
            v-model="jobForm.retryDelay"
            :min="0"
            :controls="false"
            placeholder="Default delay"
          ></el-input-number>
          <span class="form-hint">Time to wait between retries</span>
        </el-form-item>
      </el-card>

      <!-- Form actions -->
      <div class="form-actions">
        <el-button @click="cancel">Cancel</el-button>
        <el-button type="primary" native-type="submit" :loading="isSubmitting">{{ submitButtonText }}</el-button>
      </div>
    </el-form>
  </div>
</template>

<script>
import { ref, reactive, computed, watch, onMounted } from 'vue';
import { useRouter } from 'vue-router';

export default {
  name: 'JobForm',
  props: {
    job: {
      type: Object,
      default: null
    },
    submitButtonText: {
      type: String,
      default: 'Create Job'
    },
    isSubmitting: {
      type: Boolean,
      default: false
    }
  },
  emits: ['submit', 'cancel'],
  setup(props, { emit }) {
    const router = useRouter();
    const jobFormRef = ref(null);

    // Initialize with default values or provided job
    const jobForm = reactive({
      name: '',
      description: '',
      command: '',
      args: [],
      cronExpr: '',
      workDir: '',
      env: {},
      timeout: 0,
      maxRetry: 0,
      retryDelay: 0,
      enabled: true
    });

    // For tracking environment variable keys
    const envKeys = ref([]);

    // Validation rules
    const rules = {
      name: [
        { required: true, message: 'Please enter a job name', trigger: 'blur' },
        { min: 2, max: 64, message: 'Length should be 2 to 64 characters', trigger: 'blur' }
      ],
      command: [
        { required: true, message: 'Please enter a command', trigger: 'blur' }
      ],
      timeout: [
        { type: 'number', min: 0, message: 'Timeout must be a positive number', trigger: 'blur' }
      ],
      maxRetry: [
        { type: 'number', min: 0, message: 'Max retry must be a positive number', trigger: 'blur' }
      ],
      retryDelay: [
        { type: 'number', min: 0, message: 'Retry delay must be a positive number', trigger: 'blur' }
      ]
    };

    // Load job data if editing
    const loadJobData = () => {
      if (props.job) {
        // Copy basic properties
        jobForm.name = props.job.name || '';
        jobForm.description = props.job.description || '';
        jobForm.command = props.job.command || '';
        jobForm.args = [...(props.job.args || [])];
        jobForm.cronExpr = props.job.cron_expr || '';
        jobForm.workDir = props.job.work_dir || '';
        jobForm.timeout = props.job.timeout || 0;
        jobForm.maxRetry = props.job.max_retry || 0;
        jobForm.retryDelay = props.job.retry_delay || 0;
        jobForm.enabled = props.job.enabled !== undefined ? props.job.enabled : true;

        // Copy environment variables
        jobForm.env = { ...(props.job.env || {}) };

        // Update env keys
        updateEnvKeysList();
      }
    };

    // Update environment variable keys array
    const updateEnvKeysList = () => {
      envKeys.value = Object.keys(jobForm.env);
    };

    // Add a new argument
    const addArg = () => {
      jobForm.args.push('');
    };

    // Remove an argument
    const removeArg = (index) => {
      jobForm.args.splice(index, 1);
    };

    // Add a new environment variable
    const addEnvVar = () => {
      const newKey = `ENV_VAR_${envKeys.value.length + 1}`;
      jobForm.env[newKey] = '';
      updateEnvKeysList();
    };

    // Remove an environment variable
    const removeEnvVar = (key) => {
      delete jobForm.env[key];
      updateEnvKeysList();
    };

    // Update environment variable key
    const updateEnvKey = (index, oldKey) => {
      const newKey = envKeys.value[index];

      // If the key hasn't changed, do nothing
      if (newKey === oldKey) return;

      // If the new key already exists, revert the change
      if (jobForm.env.hasOwnProperty(newKey) && newKey !== oldKey) {
        envKeys.value[index] = oldKey;
        return;
      }

      // Move the value to the new key and delete the old key
      const value = jobForm.env[oldKey];
      jobForm.env[newKey] = value;
      delete jobForm.env[oldKey];
    };

    // Submit the form
    const submitForm = async () => {
      if (!jobFormRef.value) return;

      await jobFormRef.value.validate(async (valid) => {
        if (!valid) {
          return false;
        }

        // Filter out empty args
        const filteredArgs = jobForm.args.filter(arg => arg.trim() !== '');

        // Prepare the job data for submission
        const jobData = {
          name: jobForm.name,
          description: jobForm.description,
          command: jobForm.command,
          args: filteredArgs,
          cron_expr: jobForm.cronExpr,
          work_dir: jobForm.workDir,
          env: { ...jobForm.env }, // Make a copy to avoid reactive issues
          timeout: jobForm.timeout,
          max_retry: jobForm.maxRetry,
          retry_delay: jobForm.retryDelay,
          enabled: jobForm.enabled
        };

        // Emit the submit event with the job data
        emit('submit', jobData);
      });
    };

    // Cancel and go back
    const cancel = () => {
      emit('cancel');
    };

    // Watch for changes in the job prop to reload data
    watch(() => props.job, () => {
      loadJobData();
    });

    // Initialize the form
    onMounted(() => {
      loadJobData();
    });

    return {
      jobFormRef,
      jobForm,
      rules,
      envKeys,
      addArg,
      removeArg,
      addEnvVar,
      removeEnvVar,
      updateEnvKey,
      submitForm,
      cancel
    };
  }
};
</script>

<style scoped>
.job-form {
  max-width: 800px;
  margin: 0 auto;
}

.form-section {
  margin-bottom: 20px;
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

.form-hint {
  margin-left: 8px;
  font-size: 12px;
  color: #909399;
}

.args-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.arg-item {
  display: flex;
  align-items: center;
}

.env-var-row {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
  gap: 8px;
}

.env-key-input {
  flex: 2;
}

.env-equals {
  margin: 0 4px;
  color: #909399;
}

.env-value-input {
  flex: 3;
}

.add-env-btn {
  margin-top: 8px;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 24px;
  gap: 12px;
}

.cron-help-table {
  width: 100%;
  border-collapse: collapse;
  margin: 10px 0;
}

.cron-help-table th,
.cron-help-table td {
  border: 1px solid #dcdfe6;
  padding: 8px;
  text-align: left;
}

.cron-help-table th {
  background-color: #f5f7fa;
}
</style>