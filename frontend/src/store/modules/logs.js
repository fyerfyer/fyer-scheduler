import logsService from '@/services/logs.service';

// Initial state
const state = {
    // Job logs with pagination
    jobLogs: [],
    totalJobLogs: 0,
    currentPage: 1,
    pageSize: 20,

    // Current selected execution log
    currentExecution: null,
    executionLogEntries: [],

    // Streaming state
    streamConnection: null,
    streamingStatus: null,

    // Loading states
    loading: false,
    loadingExecution: false,

    // Error and success states
    error: null,
    successMessage: null,

    // Filters
    timeRangeFilter: null,
    levelFilter: null,

    // System logs
    systemLogs: [],

    // Currently selected job ID for logs
    selectedJobId: null
};

// Mutations to change state
const mutations = {
    SET_JOB_LOGS(state, { logs, total }) {
        state.jobLogs = logs;
        state.totalJobLogs = total;
    },

    SET_CURRENT_EXECUTION(state, execution) {
        state.currentExecution = execution;
    },

    SET_EXECUTION_LOG_ENTRIES(state, entries) {
        state.executionLogEntries = entries;
    },

    APPEND_LOG_ENTRY(state, entry) {
        state.executionLogEntries.push(entry);
    },

    SET_STREAM_CONNECTION(state, connection) {
        state.streamConnection = connection;
    },

    SET_STREAMING_STATUS(state, status) {
        state.streamingStatus = status;
    },

    SET_LOADING(state, isLoading) {
        state.loading = isLoading;
    },

    SET_LOADING_EXECUTION(state, isLoading) {
        state.loadingExecution = isLoading;
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

    SET_TIME_RANGE_FILTER(state, timeRange) {
        state.timeRangeFilter = timeRange;
    },

    SET_LEVEL_FILTER(state, level) {
        state.levelFilter = level;
    },

    SET_SYSTEM_LOGS(state, logs) {
        state.systemLogs = logs;
    },

    SET_SELECTED_JOB_ID(state, jobId) {
        state.selectedJobId = jobId;
    },

    CLEAR_LOGS(state) {
        state.jobLogs = [];
        state.totalJobLogs = 0;
    },

    CLEAR_EXECUTION(state) {
        state.currentExecution = null;
        state.executionLogEntries = [];
    },

    CLEAR_ERROR_AND_SUCCESS(state) {
        state.error = null;
        state.successMessage = null;
    }
};

// Actions to perform async operations
const actions = {
    // Fetch logs for a specific job with pagination
    async fetchJobLogs({ commit, state }, { jobId, page, pageSize, timeRange } = {}) {
        commit('SET_LOADING', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        const currentPage = page || state.currentPage;
        const currentPageSize = pageSize || state.pageSize;
        const currentTimeRange = timeRange || state.timeRangeFilter;

        try {
            const { logs, pagination } = await logsService.getJobLogs(
                jobId,
                currentPage,
                currentPageSize,
                currentTimeRange
            );

            commit('SET_JOB_LOGS', {
                logs: logs,
                total: pagination.total
            });

            commit('SET_PAGINATION', {
                page: pagination.current,
                pageSize: pagination.pageSize
            });

            commit('SET_SELECTED_JOB_ID', jobId);

            if (timeRange) {
                commit('SET_TIME_RANGE_FILTER', timeRange);
            }

            return logs;
        } catch (error) {
            commit('SET_ERROR', `Failed to load logs for job ${jobId}. Please try again.`);
            throw error;
        } finally {
            commit('SET_LOADING', false);
        }
    },

    // Fetch a single execution log by ID
    async fetchExecutionLog({ commit }, executionId) {
        commit('SET_LOADING_EXECUTION', true);
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            const execution = await logsService.getExecutionLog(executionId);
            commit('SET_CURRENT_EXECUTION', execution);

            // Parse log content if available
            if (execution && execution.log_content) {
                const parsedEntries = logsService.parseLogContent(execution.log_content);
                commit('SET_EXECUTION_LOG_ENTRIES', parsedEntries);
            } else {
                commit('SET_EXECUTION_LOG_ENTRIES', []);
            }

            return execution;
        } catch (error) {
            commit('SET_ERROR', `Failed to load execution log ${executionId}. Please try again.`);
            throw error;
        } finally {
            commit('SET_LOADING_EXECUTION', false);
        }
    },

    // Start streaming logs for an execution
    async startLogStream({ commit, dispatch }, executionId) {
        commit('CLEAR_ERROR_AND_SUCCESS');

        try {
            // Close any existing stream
            dispatch('stopLogStream');

            // First load the execution details
            await dispatch('fetchExecutionLog', executionId);

            // Create a new stream connection
            const eventSource = await logsService.streamExecutionLog(executionId);

            // Set up event handlers
            eventSource.onopen = () => {
                commit('SET_STREAMING_STATUS', 'connected');
            };

            eventSource.onmessage = (event) => {
                try {
                    const logData = JSON.parse(event.data);
                    const parsedEntry = {
                        id: state.executionLogEntries.length,
                        timestamp: logData.time || new Date().toISOString(),
                        level: logData.level || 'INFO',
                        message: logData.message || event.data,
                        data: logData
                    };

                    commit('APPEND_LOG_ENTRY', parsedEntry);
                } catch (e) {
                    // If can't parse as JSON, append as plain text
                    commit('APPEND_LOG_ENTRY', {
                        id: state.executionLogEntries.length,
                        timestamp: new Date().toISOString(),
                        level: 'INFO',
                        message: event.data,
                        data: null
                    });
                }
            };

            eventSource.onerror = (error) => {
                console.error('Log stream error:', error);
                commit('SET_STREAMING_STATUS', 'error');

                // Close the connection on error
                eventSource.close();
                commit('SET_STREAM_CONNECTION', null);
            };

            // Store the stream connection
            commit('SET_STREAM_CONNECTION', eventSource);
            commit('SET_STREAMING_STATUS', 'connecting');

            return eventSource;
        } catch (error) {
            commit('SET_ERROR', `Failed to start log stream for execution ${executionId}. Please try again.`);
            commit('SET_STREAMING_STATUS', 'error');
            throw error;
        }
    },

    // Stop streaming logs
    stopLogStream({ commit, state }) {
        if (state.streamConnection) {
            state.streamConnection.close();
            commit('SET_STREAM_CONNECTION', null);
            commit('SET_STREAMING_STATUS', null);
        }
    },

    // Get log statistics for a job
    async fetchJobLogStats({ commit }, jobId) {
        try {
            const stats = await logsService.getJobLogStats(jobId);
            return stats;
        } catch (error) {
            console.error(`Failed to fetch log stats for job ${jobId}:`, error);
            return null;
        }
    },

    // Get system logs
    async fetchSystemLogs({ commit }) {
        commit('SET_LOADING', true);

        try {
            const logs = await logsService.getSystemLogs();
            commit('SET_SYSTEM_LOGS', logs);
            return logs;
        } catch (error) {
            commit('SET_ERROR', 'Failed to load system logs. Please try again.');
            throw error;
        } finally {
            commit('SET_LOADING', false);
        }
    },

    // Set time range filter
    setTimeRangeFilter({ commit, dispatch, state }, timeRange) {
        commit('SET_TIME_RANGE_FILTER', timeRange);

        // If we have a selected job, reload logs with the new filter
        if (state.selectedJobId) {
            dispatch('fetchJobLogs', {
                jobId: state.selectedJobId,
                page: 1 // Reset to first page on filter change
            });
        }
    },

    // Set log level filter
    setLevelFilter({ commit }, level) {
        commit('SET_LEVEL_FILTER', level);
    },

    // Clear current execution
    clearCurrentExecution({ commit }) {
        commit('CLEAR_EXECUTION');
    },

    // Clear job logs
    clearJobLogs({ commit }) {
        commit('CLEAR_LOGS');
    },

    // Reset all filters
    resetFilters({ commit, dispatch, state }) {
        commit('SET_TIME_RANGE_FILTER', null);
        commit('SET_LEVEL_FILTER', null);

        // If we have a selected job, reload logs without filters
        if (state.selectedJobId) {
            dispatch('fetchJobLogs', {
                jobId: state.selectedJobId,
                page: 1 // Reset to first page
            });
        }
    }
};

// Getters to get computed values from state
const getters = {
    // Get all job logs
    allJobLogs: state => state.jobLogs,

    // Get current execution details
    currentExecution: state => state.currentExecution,

    // Get execution log entries
    executionLogEntries: state => state.executionLogEntries,

    // Get filtered log entries based on level filter
    filteredLogEntries: state => {
        if (!state.levelFilter) {
            return state.executionLogEntries;
        }

        return state.executionLogEntries.filter(entry =>
            entry.level.toUpperCase() === state.levelFilter.toUpperCase()
        );
    },

    // Check if logs are streaming
    isStreaming: state => state.streamConnection !== null,

    // Get streaming status
    streamingStatus: state => state.streamingStatus,

    // Check if logs are loading
    isLoading: state => state.loading,

    // Check if execution is loading
    isLoadingExecution: state => state.loadingExecution,

    // Get current error message
    error: state => state.error,

    // Get success message
    successMessage: state => state.successMessage,

    // Get pagination info
    pagination: state => ({
        current: state.currentPage,
        pageSize: state.pageSize,
        total: state.totalJobLogs
    }),

    // Get time range filter
    timeRangeFilter: state => state.timeRangeFilter,

    // Get level filter
    levelFilter: state => state.levelFilter,

    // Get system logs
    systemLogs: state => state.systemLogs,

    // Get currently selected job ID
    selectedJobId: state => state.selectedJobId,

    // Get execution status from current execution
    executionStatus: state => {
        if (!state.currentExecution) return null;
        return state.currentExecution.status;
    },

    // Get execution statistics from current execution
    executionStats: state => {
        if (!state.currentExecution) return null;

        const stats = {
            startTime: state.currentExecution.start_time,
            endTime: state.currentExecution.end_time,
            duration: null,
            exitCode: state.currentExecution.exit_code
        };

        // Calculate duration if both times are available
        if (stats.startTime && stats.endTime) {
            const start = new Date(stats.startTime);
            const end = new Date(stats.endTime);
            stats.duration = end - start;
        }

        return stats;
    },

    // Count log entries by level
    logLevelCounts: state => {
        const counts = {
            INFO: 0,
            WARN: 0,
            ERROR: 0,
            DEBUG: 0
        };

        state.executionLogEntries.forEach(entry => {
            const level = entry.level.toUpperCase();
            if (counts[level] !== undefined) {
                counts[level]++;
            }
        });

        return counts;
    }
};

export default {
    namespaced: true,
    state,
    mutations,
    actions,
    getters
};