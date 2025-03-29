import { workerApi } from './api';

/**
 * Service for worker-related operations
 */
export default {
    /**
     * Get a list of workers with pagination
     * @param {Number} page Current page number
     * @param {Number} pageSize Number of items per page
     * @returns {Promise} Promise with workers and pagination data
     */
    getWorkers(page = 1, pageSize = 10) {
        return workerApi.getWorkers(page, pageSize)
            .then(response => {
                return {
                    workers: response.items || [],
                    pagination: {
                        total: response.total,
                        current: response.page,
                        pageSize: response.page_size
                    }
                };
            })
            .catch(error => {
                console.error('Failed to fetch workers:', error);
                throw error;
            });
    },

    /**
     * Get a single worker by ID
     * @param {String} workerId Worker ID
     * @returns {Promise} Promise with worker details
     */
    getWorker(workerId) {
        return workerApi.getWorker(workerId)
            .catch(error => {
                console.error(`Failed to fetch worker ${workerId}:`, error);
                throw error;
            });
    },

    /**
     * Get active workers
     * @returns {Promise} Promise with list of active workers
     */
    getActiveWorkers() {
        return workerApi.getActiveWorkers()
            .catch(error => {
                console.error('Failed to fetch active workers:', error);
                throw error;
            });
    },

    /**
     * Enable a worker
     * @param {String} workerId Worker ID
     * @returns {Promise} Promise with enable result
     */
    enableWorker(workerId) {
        return workerApi.enableWorker(workerId)
            .catch(error => {
                console.error(`Failed to enable worker ${workerId}:`, error);
                throw error;
            });
    },

    /**
     * Disable a worker
     * @param {String} workerId Worker ID
     * @returns {Promise} Promise with disable result
     */
    disableWorker(workerId) {
        return workerApi.disableWorker(workerId)
            .catch(error => {
                console.error(`Failed to disable worker ${workerId}:`, error);
                throw error;
            });
    },

    /**
     * Check worker health
     * @param {String} workerId Worker ID
     * @returns {Promise} Promise with health check result
     */
    checkWorkerHealth(workerId) {
        return workerApi.checkWorkerHealth(workerId)
            .catch(error => {
                console.error(`Failed to check health for worker ${workerId}:`, error);
                throw error;
            });
    },

    /**
     * Update worker labels
     * @param {String} workerId Worker ID
     * @param {Object} labels Key-value pairs of labels
     * @returns {Promise} Promise with update result
     */
    updateWorkerLabels(workerId, labels) {
        return workerApi.updateWorkerLabels(workerId, { labels })
            .catch(error => {
                console.error(`Failed to update labels for worker ${workerId}:`, error);
                throw error;
            });
    },

    /**
     * Format worker status text from worker health data
     * @param {Object} worker Worker object
     * @returns {String} Status text ("healthy", "unhealthy", or "offline")
     */
    getWorkerStatus(worker) {
        if (!worker) return 'unknown';
        if (!worker.is_active) return 'offline';
        return worker.is_healthy ? 'healthy' : 'unhealthy';
    },

    /**
     * Calculate worker resource usage percentages
     * @param {Object} worker Worker object with resources data
     * @returns {Object} Object with CPU and memory usage percentages
     */
    calculateResourceUsage(worker) {
        if (!worker || !worker.resources) {
            return { cpu: 0, memory: 0, disk: 0 };
        }

        const resources = worker.resources;

        // Calculate CPU usage percentage (assuming resources.cpu_usage is a percentage)
        const cpuUsage = resources.cpu_usage || 0;

        // Calculate memory usage percentage
        const memoryTotal = resources.memory_total || 1;
        const memoryUsed = resources.memory_used || 0;
        const memoryUsage = (memoryUsed / memoryTotal) * 100;

        // Calculate disk usage percentage
        const diskTotal = resources.disk_total || 1;
        const diskUsed = resources.disk_used || 0;
        const diskUsage = (diskUsed / diskTotal) * 100;

        return {
            cpu: parseFloat(cpuUsage.toFixed(1)),
            memory: parseFloat(memoryUsage.toFixed(1)),
            disk: parseFloat(diskUsage.toFixed(1))
        };
    },

    /**
     * Format uptime duration from seconds to human-readable string
     * @param {Number} uptimeSeconds Uptime in seconds
     * @returns {String} Formatted uptime string
     */
    formatUptime(uptimeSeconds) {
        if (!uptimeSeconds || uptimeSeconds < 0) {
            return 'N/A';
        }

        const days = Math.floor(uptimeSeconds / 86400);
        const hours = Math.floor((uptimeSeconds % 86400) / 3600);
        const minutes = Math.floor((uptimeSeconds % 3600) / 60);

        if (days > 0) {
            return `${days}d ${hours}h ${minutes}m`;
        } else if (hours > 0) {
            return `${hours}h ${minutes}m`;
        } else {
            return `${minutes}m`;
        }
    },

    /**
     * Parse worker tags from string format
     * @param {String} tagsString Tags string in format "key1=value1,key2=value2"
     * @returns {Object} Object with key-value pairs
     */
    parseWorkerTags(tagsString) {
        if (!tagsString) return {};

        const tags = {};
        tagsString.split(',').forEach(tag => {
            const [key, value] = tag.split('=');
            if (key && value) {
                tags[key.trim()] = value.trim();
            }
        });

        return tags;
    },

    /**
     * Format worker tags into string format
     * @param {Object} tags Object with key-value pairs
     * @returns {String} Tags string in format "key1=value1,key2=value2"
     */
    formatWorkerTags(tags) {
        if (!tags || typeof tags !== 'object') return '';

        return Object.entries(tags)
            .map(([key, value]) => `${key}=${value}`)
            .join(',');
    }
};