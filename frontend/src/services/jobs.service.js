import {jobApi} from './api';

/**
 * Service for job-related operations
 */
export default {
    /**
     * Get a list of jobs with pagination
     * @param {Number} page Current page number
     * @param {Number} pageSize Number of items per page
     * @param {String} status Optional status filter
     * @returns {Promise} Promise with jobs and pagination data
     */
    getJobs(page = 1, pageSize = 10, status = '') {
        const queryParams = new URLSearchParams();
        queryParams.append('page', page);
        queryParams.append('page_size', pageSize);

        if (status) {
            queryParams.append('status', status);
        }

        return jobApi.getJobs(page, pageSize)
            .then(response => {
                return {
                    jobs: response.items || [],
                    pagination: {
                        total: response.total,
                        current: response.page,
                        pageSize: response.page_size
                    }
                };
            })
            .catch(error => {
                console.error('Failed to fetch jobs:', error);
                throw error;
            });
    },

    /**
     * Get a single job by ID
     * @param {String} jobId Job ID
     * @returns {Promise} Promise with job details
     */
    getJob(jobId) {
        return jobApi.getJob(jobId)
            .catch(error => {
                console.error(`Failed to fetch job ${jobId}:`, error);
                throw error;
            });
    },

    /**
     * Create a new job
     * @param {Object} jobData Job data
     * @returns {Promise} Promise with created job
     */
    createJob(jobData) {
        return jobApi.createJob(jobData)
            .catch(error => {
                console.error('Failed to create job:', error);
                throw error;
            });
    },

    /**
     * Update an existing job
     * @param {String} jobId Job ID
     * @param {Object} jobData Updated job data
     * @returns {Promise} Promise with updated job
     */
    updateJob(jobId, jobData) {
        return jobApi.updateJob(jobId, jobData)
            .catch(error => {
                console.error(`Failed to update job ${jobId}:`, error);
                throw error;
            });
    },

    /**
     * Delete a job
     * @param {String} jobId Job ID
     * @returns {Promise} Promise with delete result
     */
    deleteJob(jobId) {
        return jobApi.deleteJob(jobId)
            .catch(error => {
                console.error(`Failed to delete job ${jobId}:`, error);
                throw error;
            });
    },

    /**
     * Trigger immediate execution of a job
     * @param {String} jobId Job ID
     * @returns {Promise} Promise with execution details
     */
    triggerJob(jobId) {
        return jobApi.triggerJob(jobId)
            .catch(error => {
                console.error(`Failed to trigger job ${jobId}:`, error);
                throw error;
            });
    },

    /**
     * Kill a running job
     * @param {String} jobId Job ID
     * @returns {Promise} Promise with kill result
     */
    killJob(jobId) {
        return jobApi.killJob(jobId)
            .catch(error => {
                console.error(`Failed to kill job ${jobId}:`, error);
                throw error;
            });
    },

    /**
     * Enable a job
     * @param {String} jobId Job ID
     * @returns {Promise} Promise with enable result
     */
    enableJob(jobId) {
        return jobApi.enableJob(jobId)
            .catch(error => {
                console.error(`Failed to enable job ${jobId}:`, error);
                throw error;
            });
    },

    /**
     * Disable a job
     * @param {String} jobId Job ID
     * @returns {Promise} Promise with disable result
     */
    disableJob(jobId) {
        return jobApi.disableJob(jobId)
            .catch(error => {
                console.error(`Failed to disable job ${jobId}:`, error);
                throw error;
            });
    },

    /**
     * Get job statistics
     * @param {String} jobId Job ID
     * @returns {Promise} Promise with job statistics
     */
    getJobStats(jobId) {
        return jobApi.getJobStats(jobId)
            .catch(error => {
                console.error(`Failed to get stats for job ${jobId}:`, error);
                throw error;
            });
    }
};