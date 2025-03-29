<template>
  <div class="job-edit-view">
    <div class="page-header">
      <h1>Edit Job</h1>
    </div>

    <!-- Error alert -->
    <el-alert
      v-if="error"
      :title="error"
      type="error"
      show-icon
      closable
      @close="clearError"
      class="alert-message"
    />

    <!-- Success message -->
    <el-alert
      v-if="successMessage"
      :title="successMessage"
      type="success"
      show-icon
      closable
      @close="clearSuccessMessage"
      class="alert-message"
    />

    <!-- Form component -->
    <JobForm
      :job="job"
      :isSubmitting="isSubmitting"
      submitButtonText="Save Changes"
      @submit="handleSubmit"
      @cancel="handleCancel"
    />
  </div>
</template>

<script>
import { ref, computed, onMounted } from 'vue';
import { useStore } from 'vuex';
import { useRouter, useRoute } from 'vue-router';
import JobForm from '@/components/jobs/JobForm.vue';

export default {
  name: 'JobEditView',
  components: {
    JobForm
  },
  setup() {
    const store = useStore();
    const router = useRouter();
    const route = useRoute();
    const isSubmitting = ref(false);

    // Get job ID from route
    const jobId = route.params.id;

    // Computed properties
    const job = computed(() => store.getters['jobs/currentJob']);
    const error = computed(() => store.getters['jobs/error']);
    const successMessage = computed(() => store.getters['jobs/successMessage']);

    // Load job data
    const loadJob = async () => {
      try {
        await store.dispatch('jobs/fetchJob', jobId);
      } catch (error) {
        console.error('Failed to load job:', error);
      }
    };

    // Clear messages
    const clearError = () => {
      store.commit('jobs/SET_ERROR', null);
    };

    const clearSuccessMessage = () => {
      store.commit('jobs/SET_SUCCESS_MESSAGE', null);
    };

    // Handle form submission
    const handleSubmit = async (jobData) => {
      isSubmitting.value = true;

      try {
        // Dispatch action to update job
        await store.dispatch('jobs/updateJob', {
          jobId: jobId,
          jobData: jobData
        });

        // Show success notification
        store.commit('jobs/SET_SUCCESS_MESSAGE', `Job "${jobData.name}" updated successfully`);

        // Navigate to job details
        router.push(`/jobs/${jobId}`);
      } catch (error) {
        console.error('Failed to update job:', error);
        // Error will be set in the store by the action
      } finally {
        isSubmitting.value = false;
      }
    };

    // Handle cancel button
    const handleCancel = () => {
      router.push(`/jobs/${jobId}`);
    };

    // Load job on mount
    onMounted(() => {
      loadJob();
    });

    return {
      job,
      isSubmitting,
      error,
      successMessage,
      clearError,
      clearSuccessMessage,
      handleSubmit,
      handleCancel
    };
  }
};
</script>

<style scoped>
.job-edit-view {
  padding-bottom: 40px;
}

.page-header {
  margin-bottom: 24px;
}

.alert-message {
  margin-bottom: 24px;
}
</style>