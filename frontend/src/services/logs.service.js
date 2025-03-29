import { logApi } from './api';

/**
 * Service for log-related operations
 */
export default {
    /**
     * Get logs for a specific job with pagination
     * @param {String} jobId Job ID
     * @param {Number} page Current page number
     * @param {Number} pageSize Number of items per page
     * @param {Object} timeRange Optional time range filter with startTime and endTime properties
     * @returns {Promise} Promise with logs and pagination data
     */
    getJobLogs(jobId, page = 1, pageSize = 10, timeRange = null) {
        const queryParams = new URLSearchParams();
        queryParams.append('page', page);
        queryParams.append('page_size', pageSize);

        if (timeRange && timeRange.startTime) {
            queryParams.append('start_time', timeRange.startTime);
        }

        if (timeRange && timeRange.endTime) {
            queryParams.append('end_time', timeRange.endTime);
        }

        return logApi.getJobLogs(jobId, page, pageSize)
            .then(response => {
                return {
                    logs: response.items || [],
                    pagination: {
                        total: response.total,
                        current: response.page,
                        pageSize: response.page_size
                    }
                };
            })
            .catch(error => {
                console.error(`Failed to fetch logs for job ${jobId}:`, error);
                throw error;
            });
    },

    /**
     * Get details for a specific execution
     * @param {String} executionId Execution ID
     * @returns {Promise} Promise with execution log details
     */
    getExecutionLog(executionId) {
        return logApi.getExecutionLog(executionId)
            .catch(error => {
                console.error(`Failed to fetch execution log ${executionId}:`, error);
                throw error;
            });
    },

    /**
     * Get a streaming connection for real-time execution logs
     * @param {String} executionId Execution ID
     * @returns {Promise} Promise with EventSource or WebSocket connection
     */
    streamExecutionLog(executionId) {
        const url = `${process.env.VUE_APP_API_URL || 'http://localhost:8080'}/api/v1/logs/executions/${executionId}/stream`;

        // Create an EventSource for server-sent events
        return new Promise((resolve, reject) => {
            try {
                const eventSource = new EventSource(url);
                resolve(eventSource);
            } catch (error) {
                console.error(`Failed to create log stream for execution ${executionId}:`, error);
                reject(error);
            }
        });
    },

    /**
     * Get log statistics for a job
     * @param {String} jobId Job ID
     * @returns {Promise} Promise with log statistics
     */
    getJobLogStats(jobId) {
        return logApi.getJobLogStats(jobId)
            .catch(error => {
                console.error(`Failed to fetch log stats for job ${jobId}:`, error);
                throw error;
            });
    },

    /**
     * Get system logs
     * @param {Number} page Current page number
     * @param {Number} pageSize Number of items per page
     * @returns {Promise} Promise with system logs
     */
    getSystemLogs(page = 1, pageSize = 20) {
        return logApi.getSystemLogs()
            .catch(error => {
                console.error('Failed to fetch system logs:', error);
                throw error;
            });
    },

    /**
     * Parse and format log content
     * @param {String} logContent Raw log content
     * @returns {Array} Array of parsed log entries
     */
    parseLogContent(logContent) {
        if (!logContent) return [];

        // Split log content by lines and parse each line
        return logContent.split('\n')
            .filter(line => line.trim() !== '')
            .map((line, index) => {
                // Try to parse as JSON if possible
                try {
                    const parsed = JSON.parse(line);
                    return {
                        id: index,
                        timestamp: parsed.time || new Date().toISOString(),
                        level: parsed.level || 'INFO',
                        message: parsed.message || line,
                        data: parsed
                    };
                } catch (e) {
                    // If not JSON, use basic parsing with regular expressions
                    const timestampMatch = line.match(/\[(.*?)\]/);
                    const levelMatch = line.match(/\[(INFO|WARN|ERROR|DEBUG)\]/i);

                    return {
                        id: index,
                        timestamp: timestampMatch ? timestampMatch[1] : new Date().toISOString(),
                        level: levelMatch ? levelMatch[1].toUpperCase() : 'INFO',
                        message: line,
                        data: null
                    };
                }
            });
    }
};