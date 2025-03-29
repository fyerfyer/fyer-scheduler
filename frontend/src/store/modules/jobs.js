import jobsService from '@/services/jobs.service';

// Initial state
const state = {
    // Jobs list with pagination
    jobs: [],
    totalJobs: 0,
    currentPage: 1,
    pageSize: 10,

    // Current selected job
    currentJob: null,

    // Loading states
    loading: false,
    loadingJob: false,

    // Operation statuses
    error: null,
    successMessage: null,

    // Stats
    jobStats: null,
    loadingStats: false,

    // Filter states
    statusFilter: ''
};

// Mutations to change state
const mutations = {
    SET_JOBS(state, { jobs, total }) {
        state.jobs = jobs;
        state.totalJobs = total;
    },

    SET_CURRENT_JOB(state, job) {
        state.currentJob = job;
    },

    SET_LOADING(state, isLoading) {
        state.loading = isLoading;
    },

    SET_LOADING_JOB(state, isLoading) {
        state.loadingJob = isLoading;
    },

    SET_ERROR(state, error) {
        state.error = error;
    },

    SET_SUCCESS_MESSAGE(state, message) {
        state.successMessage = message;
    },

    SET_PAGINATION(state, { page, pageSize }) {
        state.currentPage = page;
        state.pageSize = pageSize;
    },

    SET_JOB_STATS(state, stats) {
        state.jobStats = stats;
    },

    SET_LOADING_STATS(state, isLoading) {
        state.loadingStats = isLoading;
    },

    SET_STATUS_FILTER(state, status) {
        state.statusFilter = status;
    },

    UPDATE_JOB_IN_LIST(state, updatedJob) {
        const index = state.jobs.findIndex(job => job.id === updatedJob.id);
        if (index !== -1) {
            state.jobs.splice(index, 1, updatedJob);
        }
    },

    REMOVE_JOB_FROM_LIST(state, jobId) {
        state.jobs = state.jobs.filter(job => job.id !== jobId);
    },

    CLEAR_CURRENT_JOB(state) {
        state.currentJob = null;
    },

    CLEAR_ERROR_AND_SUCCESS(state) {
        state.error = null;
        state.successMessage = null;
    }
};

// Actions to perform async operations
const actions = {
    // Fetch jobs list with pagination
    async fetchJobs({ commit, state }, { page, pageSize, status } = {}) {
        commit('SET_LOADING', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        const currentPage = page || state.currentPage;
        const currentPageSize = pageSize || state.pageSize;
        const currentStatus = status !== undefined ? status : state.statusFilter;

        try {
            const { jobs, pagination } = await jobsService.getJobs(currentPage, currentPageSize, currentStatus);

            commit('SET_JOBS', {
                jobs: jobs,
                total: pagination.total
            });

            commit('SET_PAGINATION', {
                page: pagination.current,
                pageSize: pagination.pageSize
            });

            if (status !== undefined) {
                commit('SET_STATUS_FILTER', status);
            }

            return jobs;
        } catch (error) {
            commit('SET_ERROR', 'Failed to load jobs. Please try again.');
            throw error;
        } finally {
            commit('SET_LOADING', false);
        }
    },

    // Fetch a single job by ID
    async fetchJob({ commit }, jobId) {
        commit('SET_LOADING_JOB', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            const job = await jobsService.getJob(jobId);
            commit('SET_CURRENT_JOB', job);
            return job;
        } catch (error) {
            commit('SET_ERROR', `Failed to load job ${jobId}. Please try again.`);
            throw error;
        } finally {
            commit('SET_LOADING_JOB', false);
        }
    },

    // Create a new job
    async createJob({ commit, dispatch }, jobData) {
        commit('SET_LOADING', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            const newJob = await jobsService.createJob(jobData);
            commit('SET_SUCCESS_MESSAGE', 'Job created successfully');

            // Refresh the jobs list after creating
            await dispatch('fetchJobs');

            return newJob;
        } catch (error) {
            commit('SET_ERROR', 'Failed to create job. Please check your input and try again.');
            throw error;
        } finally {
            commit('SET_LOADING', false);
        }
    },

    // Update an existing job
    async updateJob({ commit, state }, { jobId, jobData }) {
        commit('SET_LOADING', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            const updatedJob = await jobsService.updateJob(jobId, jobData);
            commit('SET_SUCCESS_MESSAGE', 'Job updated successfully');

            // Update the job in list if it exists there
            commit('UPDATE_JOB_IN_LIST', updatedJob);

            // If this is the current selected job, update it too
            if (state.currentJob && state.currentJob.id === jobId) {
                commit('SET_CURRENT_JOB', updatedJob);
            }

            return updatedJob;
        } catch (error) {
            commit('SET_ERROR', 'Failed to update job. Please try again.');
            throw error;
        } finally {
            commit('SET_LOADING', false);
        }
    },

    // Delete a job
    async deleteJob({ commit, dispatch }, jobId) {
        commit('SET_LOADING', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            await jobsService.deleteJob(jobId);
            commit('SET_SUCCESS_MESSAGE', 'Job deleted successfully');
            commit('REMOVE_JOB_FROM_LIST', jobId);

            // If the deleted job was the current job, clear it
            commit('CLEAR_CURRENT_JOB');

            return true;
        } catch (error) {
            commit('SET_ERROR', 'Failed to delete job. Please try again.');
            throw error;
        } finally {
            commit('SET_LOADING', false);
        }
    },

    // Trigger a job to run immediately
    async triggerJob({ commit }, jobId) {
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            const result = await jobsService.triggerJob(jobId);
            commit('SET_SUCCESS_MESSAGE', 'Job triggered successfully');
            return result;
        } catch (error) {
            commit('SET_ERROR', 'Failed to trigger job. Please try again.');
            throw error;
        }
    },

    // Kill a running job
    async killJob({ commit }, jobId) {
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            await jobsService.killJob(jobId);
            commit('SET_SUCCESS_MESSAGE', 'Job terminated successfully');
            return true;
        } catch (error) {
            commit('SET_ERROR', 'Failed to terminate job. Please try again.');
            throw error;
        }
    },

    // Enable a job
    async enableJob({ commit, dispatch, state }, jobId) {
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            await jobsService.enableJob(jobId);
            commit('SET_SUCCESS_MESSAGE', 'Job enabled successfully');

            // Refresh the job to get updated state
            if (state.currentJob && state.currentJob.id === jobId) {
                dispatch('fetchJob', jobId);
            }

            // Update job in list if it exists
            const jobIndex = state.jobs.findIndex(job => job.id === jobId);
            if (jobIndex !== -1) {
                const updatedJobs = [...state.jobs];
                updatedJobs[jobIndex] = { ...updatedJobs[jobIndex], enabled: true };
                commit('SET_JOBS', { jobs: updatedJobs, total: state.totalJobs });
            }

            return true;
        } catch (error) {
            commit('SET_ERROR', 'Failed to enable job. Please try again.');
            throw error;
        }
    },

    // Disable a job
    async disableJob({ commit, dispatch, state }, jobId) {
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            await jobsService.disableJob(jobId);
            commit('SET_SUCCESS_MESSAGE', 'Job disabled successfully');

            // Refresh the job to get updated state
            if (state.currentJob && state.currentJob.id === jobId) {
                dispatch('fetchJob', jobId);
            }

            // Update job in list if it exists
            const jobIndex = state.jobs.findIndex(job => job.id === jobId);
            if (jobIndex !== -1) {
                const updatedJobs = [...state.jobs];
                updatedJobs[jobIndex] = { ...updatedJobs[jobIndex], enabled: false };
                commit('SET_JOBS', { jobs: updatedJobs, total: state.totalJobs });
            }

            return true;
        } catch (error) {
            commit('SET_ERROR', 'Failed to disable job. Please try again.');
            throw error;
        }
    },

    // Get job statistics
    async fetchJobStats({ commit }, jobId) {
        commit('SET_LOADING_STATS', true);

        try {
            const stats = await jobsService.getJobStats(jobId);
            commit('SET_JOB_STATS', stats);
            return stats;
        } catch (error) {
            console.error('Failed to fetch job stats:', error);
            // Not setting an error state since stats are supplementary
            return null;
        } finally {
            commit('SET_LOADING_STATS', false);
        }
    },

    // Clear current job selection
    clearCurrentJob({ commit }) {
        commit('CLEAR_CURRENT_JOB');
    },

    // Set job filter
    setStatusFilter({ commit, dispatch }, status) {
        commit('SET_STATUS_FILTER', status);
        dispatch('fetchJobs', { page: 1 }); // Reset to first page when filtering
    },

    // Reset all filters
    resetFilters({ commit, dispatch }) {
        commit('SET_STATUS_FILTER', '');
        dispatch('fetchJobs', { page: 1 });
    }
};

// Getters to get computed values from state
const getters = {
    // Get all jobs
    allJobs: state => state.jobs,

    // Get total jobs count
    totalJobsCount: state => state.totalJobs,

    // Get current job details
    currentJob: state => state.currentJob,

    // Get pagination info
    pagination: state => ({
        current: state.currentPage,
        pageSize: state.pageSize,
        total: state.totalJobs
    }),

    // Check if jobs are loading
    isLoading: state => state.loading,

    // Check if current job is loading
    isLoadingJob: state => state.loadingJob,

    // Get current error message
    error: state => state.error,

    // Get success message
    successMessage: state => state.successMessage,

    // Get job statistics
    jobStats: state => state.jobStats,

    // Check if job stats are loading
    isLoadingStats: state => state.loadingStats,

    // Get active status filter
    statusFilter: state => state.statusFilter,

    // Get jobs grouped by status
    jobsByStatus: state => {
        const result = {
            running: 0,
            pending: 0,
            failed: 0,
            completed: 0,
            disabled: 0
        };

        state.jobs.forEach(job => {
            if (!job.enabled) {
                result.disabled++;
                return;
            }

            const status = job.status.toLowerCase();
            if (result[status] !== undefined) {
                result[status]++;
            }
        });

        return result;
    }
};

export default {
    namespaced: true,
    state,
    mutations,
    actions,
    getters
};