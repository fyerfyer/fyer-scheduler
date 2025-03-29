import workersService from '@/services/workers.service';

// Initial state
const state = {
    // Workers list with pagination
    workers: [],
    totalWorkers: 0,
    currentPage: 1,
    pageSize: 10,

    // Current selected worker
    currentWorker: null,

    // Active workers
    activeWorkers: [],

    // Loading states
    loading: false,
    loadingWorker: false,
    loadingHealth: false,
    loadingActiveWorkers: false,

    // Operation statuses
    error: null,
    successMessage: null,

    // Health check status & results
    healthCheckResults: {},

    // Filter states
    statusFilter: ''
};

// Mutations to change state
const mutations = {
    SET_WORKERS(state, { workers, total }) {
        state.workers = workers;
        state.totalWorkers = total;
    },

    SET_CURRENT_WORKER(state, worker) {
        state.currentWorker = worker;
    },

    SET_ACTIVE_WORKERS(state, workers) {
        state.activeWorkers = workers;
    },

    SET_LOADING(state, isLoading) {
        state.loading = isLoading;
    },

    SET_LOADING_WORKER(state, isLoading) {
        state.loadingWorker = isLoading;
    },

    SET_LOADING_HEALTH(state, isLoading) {
        state.loadingHealth = isLoading;
    },

    SET_LOADING_ACTIVE_WORKERS(state, isLoading) {
        state.loadingActiveWorkers = isLoading;
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

    SET_STATUS_FILTER(state, status) {
        state.statusFilter = status;
    },

    SET_HEALTH_CHECK_RESULTS(state, { workerId, results }) {
        state.healthCheckResults = {
            ...state.healthCheckResults,
            [workerId]: results
        };
    },

    UPDATE_WORKER_IN_LIST(state, updatedWorker) {
        const index = state.workers.findIndex(worker => worker.id === updatedWorker.id);
        if (index !== -1) {
            state.workers.splice(index, 1, updatedWorker);
        }
    },

    CLEAR_CURRENT_WORKER(state) {
        state.currentWorker = null;
    },

    CLEAR_ERROR_AND_SUCCESS(state) {
        state.error = null;
        state.successMessage = null;
    }
};

// Actions to perform async operations
const actions = {
    // Fetch workers list with pagination
    async fetchWorkers({ commit, state }, { page, pageSize } = {}) {
        commit('SET_LOADING', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        const currentPage = page || state.currentPage;
        const currentPageSize = pageSize || state.pageSize;

        try {
            const { workers, pagination } = await workersService.getWorkers(
                currentPage,
                currentPageSize
            );

            commit('SET_WORKERS', {
                workers: workers,
                total: pagination.total
            });

            commit('SET_PAGINATION', {
                page: pagination.current,
                pageSize: pagination.pageSize
            });

            return workers;
        } catch (error) {
            commit('SET_ERROR', 'Failed to fetch workers. Please try again.');
            throw error;
        } finally {
            commit('SET_LOADING', false);
        }
    },

    // Fetch a single worker by ID
    async fetchWorker({ commit }, workerId) {
        commit('SET_LOADING_WORKER', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            const worker = await workersService.getWorker(workerId);
            commit('SET_CURRENT_WORKER', worker);
            return worker;
        } catch (error) {
            commit('SET_ERROR', `Failed to fetch worker ${workerId}. Please try again.`);
            throw error;
        } finally {
            commit('SET_LOADING_WORKER', false);
        }
    },

    // Fetch active workers
    async fetchActiveWorkers({ commit }) {
        commit('SET_LOADING_ACTIVE_WORKERS', true);

        try {
            const workers = await workersService.getActiveWorkers();
            commit('SET_ACTIVE_WORKERS', workers);
            return workers;
        } catch (error) {
            console.error('Failed to fetch active workers:', error);
            commit('SET_ERROR', 'Failed to fetch active workers. Please try again.');
            throw error;
        } finally {
            commit('SET_LOADING_ACTIVE_WORKERS', false);
        }
    },

    // Enable a worker
    async enableWorker({ commit, dispatch }, workerId) {
        commit('SET_LOADING', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            await workersService.enableWorker(workerId);

            // If current worker is being enabled, update it
            const currentWorker = await workersService.getWorker(workerId);
            commit('SET_CURRENT_WORKER', currentWorker);
            commit('UPDATE_WORKER_IN_LIST', currentWorker);

            commit('SET_SUCCESS_MESSAGE', 'Worker enabled successfully.');
            return currentWorker;
        } catch (error) {
            commit('SET_ERROR', `Failed to enable worker ${workerId}. Please try again.`);
            throw error;
        } finally {
            commit('SET_LOADING', false);
        }
    },

    // Disable a worker
    async disableWorker({ commit, dispatch }, workerId) {
        commit('SET_LOADING', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            await workersService.disableWorker(workerId);

            // If current worker is being disabled, update it
            const currentWorker = await workersService.getWorker(workerId);
            commit('SET_CURRENT_WORKER', currentWorker);
            commit('UPDATE_WORKER_IN_LIST', currentWorker);

            commit('SET_SUCCESS_MESSAGE', 'Worker disabled successfully.');
            return currentWorker;
        } catch (error) {
            commit('SET_ERROR', `Failed to disable worker ${workerId}. Please try again.`);
            throw error;
        } finally {
            commit('SET_LOADING', false);
        }
    },

    // Check worker health
    async checkWorkerHealth({ commit }, workerId) {
        commit('SET_LOADING_HEALTH', true);

        try {
            const healthResult = await workersService.checkWorkerHealth(workerId);
            commit('SET_HEALTH_CHECK_RESULTS', {
                workerId,
                results: healthResult
            });
            return healthResult;
        } catch (error) {
            console.error(`Failed to check health for worker ${workerId}:`, error);
            throw error;
        } finally {
            commit('SET_LOADING_HEALTH', false);
        }
    },

    // Update worker labels
    async updateWorkerLabels({ commit, dispatch }, { workerId, labels }) {
        commit('SET_LOADING', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            await workersService.updateWorkerLabels(workerId, labels);

            // Update worker after labels update
            const updatedWorker = await workersService.getWorker(workerId);
            commit('SET_CURRENT_WORKER', updatedWorker);
            commit('UPDATE_WORKER_IN_LIST', updatedWorker);

            commit('SET_SUCCESS_MESSAGE', 'Worker labels updated successfully.');
            return updatedWorker;
        } catch (error) {
            commit('SET_ERROR', `Failed to update worker labels. Please try again.`);
            throw error;
        } finally {
            commit('SET_LOADING', false);
        }
    },

    // Filter workers by status
    setStatusFilter({ commit, dispatch }, status) {
        commit('SET_STATUS_FILTER', status);
        dispatch('fetchWorkers', { page: 1 }); // Reset to first page with new filter
    },

    // Clear current worker selection
    clearCurrentWorker({ commit }) {
        commit('CLEAR_CURRENT_WORKER');
    },

    // Clear error and success messages
    clearMessages({ commit }) {
        commit('CLEAR_ERROR_AND_SUCCESS');
    }
};

// Getters to get computed values from state
const getters = {
    allWorkers: state => state.workers,
    totalWorkersCount: state => state.totalWorkers,
    currentWorker: state => state.currentWorker,
    activeWorkers: state => state.activeWorkers,
    healthCheckResults: state => state.healthCheckResults,

    pagination: state => ({
        current: state.currentPage,
        pageSize: state.pageSize,
        total: state.totalWorkers
    }),

    isLoading: state => state.loading,
    isLoadingWorker: state => state.loadingWorker,
    isLoadingHealth: state => state.loadingHealth,
    isLoadingActiveWorkers: state => state.loadingActiveWorkers,

    error: state => state.error,
    successMessage: state => state.successMessage,
    statusFilter: state => state.statusFilter,

    // Get worker health status with calculated values
    workerHealth: state => workerId => {
        const result = state.healthCheckResults[workerId];
        if (!result) return null;

        return {
            isHealthy: result.is_healthy || false,
            cpuUsage: result.resources?.cpu_usage || 0,
            memoryUsage: result.resources?.memory_used ?
                (result.resources.memory_used / result.resources.memory_total) * 100 : 0,
            diskUsage: result.resources?.disk_used ?
                (result.resources.disk_used / result.resources.disk_total) * 100 : 0,
            lastChecked: new Date(result.check_time || Date.now()).toLocaleString()
        };
    },

    // Group workers by status
    workersByStatus: state => {
        const grouped = {
            healthy: 0,
            unhealthy: 0,
            offline: 0,
            total: state.totalWorkers
        };

        state.workers.forEach(worker => {
            const status = workersService.getWorkerStatus(worker);
            if (grouped[status] !== undefined) {
                grouped[status]++;
            }
        });

        return grouped;
    }
};

export default {
    namespaced: true,
    state,
    mutations,
    actions,
    getters
};