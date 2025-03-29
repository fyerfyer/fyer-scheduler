<template>
  <div class="job-create-view">
    <div class="page-header">
      <h1>Create New Job</h1>
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

    <!-- Form component -->
    <JobForm
      :isSubmitting="isSubmitting"
      submitButtonText="Create Job"
      @submit="handleSubmit"
      @cancel="handleCancel"
    />
  </div>
</template>

<script>
import { ref, computed } from 'vue';
import { useStore } from 'vuex';
import { useRouter } from 'vue-router';
import JobForm from '@/components/jobs/JobForm.vue';

export default {
  name: 'JobCreateView',
  components: {
    JobForm
  },
  setup() {
    const store = useStore();
    const router = useRouter();
    const isSubmitting = ref(false);

    // Get error from store
    const error = computed(() => store.getters['jobs/error']);

    // Clear error
    const clearError = () => {
      store.commit('jobs/SET_ERROR', null);
    };

    // Handle form submission
    const handleSubmit = async (jobData) => {
      isSubmitting.value = true;

      try {
        // Dispatch action to create job
        const createdJob = await store.dispatch('jobs/createJob', jobData);

        // Show success notification
        store.commit('jobs/SET_SUCCESS_MESSAGE', `Job "${jobData.name}" created successfully`);

        // Navigate to job list or job detail
        router.push('/jobs');
      } catch (error) {
        console.error('Failed to create job:', error);
        // Error will be set in the store by the action
      } finally {
        isSubmitting.value = false;
      }
    };

    // Handle cancel button
    const handleCancel = () => {
      router.push('/jobs');
    };

    return {
      isSubmitting,
      error,
      clearError,
      handleSubmit,
      handleCancel
    };
  }
};
</script>

<style scoped>
.job-create-view {
  padding-bottom: 40px;
}

.page-header {
  margin-bottom: 24px;
}

.alert-message {
  margin-bottom: 24px;
}
</style>