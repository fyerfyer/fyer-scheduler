import axios from 'axios';

// Create axios instance
const api = axios.create({
    baseURL: process.env.VUE_APP_API_URL || 'http://localhost:8080/api/v1',
    timeout: 10000,
    headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json'
    }
});

// Request interceptor
api.interceptors.request.use(
    config => {
        // You could add API key here if required
        // const apiKey = localStorage.getItem('apiKey');
        // if (apiKey) {
        //   config.headers['X-API-Key'] = apiKey;
        // }
        return config;
    },
    error => {
        return Promise.reject(error);
    }
);

// Response interceptor
api.interceptors.response.use(
    response => {
        // Extract data from the API's response wrapper
        if (response.data && response.data.success) {
            return response.data.data;
        }
        return response.data;
    },
    error => {
        const errorResponse = error.response;

        if (errorResponse && errorResponse.status) {
            switch (errorResponse.status) {
                case 401:
                    console.error('Unauthorized access');
                    break;
                case 404:
                    console.error('Resource not found');
                    break;
                case 500:
                    console.error('Server error');
                    break;
                default:
                    console.error('API error:', errorResponse.data);
            }
        } else {
            console.error('Network error or API service unavailable');
        }

        return Promise.reject(error);
    }
);

// API endpoints for jobs
export const jobApi = {
    getJobs: (page = 1, pageSize = 10) => api.get(`/jobs?page=${page}&page_size=${pageSize}`),
    getJob: (id) => api.get(`/jobs/${id}`),
    createJob: (jobData) => api.post('/jobs', jobData),
    updateJob: (id, jobData) => api.put(`/jobs/${id}`, jobData),
    deleteJob: (id) => api.delete(`/jobs/${id}`),
    triggerJob: (id) => api.post(`/jobs/${id}/run`),
    killJob: (id) => api.post(`/jobs/${id}/kill`),
    enableJob: (id) => api.put(`/jobs/${id}/enable`),
    disableJob: (id) => api.put(`/jobs/${id}/disable`),
    getJobStats: (id) => api.get(`/jobs/${id}/stats`)
};

// API endpoints for logs
export const logApi = {
    getJobLogs: (jobId, page = 1, pageSize = 10) => api.get(`/logs/jobs/${jobId}?page=${page}&page_size=${pageSize}`),
    getExecutionLog: (executionId) => api.get(`/logs/executions/${executionId}`),
    streamExecutionLog: (executionId) => api.get(`/logs/executions/${executionId}/stream`),
    getJobLogStats: (jobId) => api.get(`/logs/jobs/${jobId}/stats`),
    getSystemLogs: () => api.get('/logs/system')
};

// API endpoints for workers
export const workerApi = {
    getWorkers: (page = 1, pageSize = 10) => api.get(`/workers?page=${page}&page_size=${pageSize}`),
    getWorker: (id) => api.get(`/workers/${id}`),
    getActiveWorkers: () => api.get('/workers/active'),
    enableWorker: (id) => api.put(`/workers/${id}/enable`),
    disableWorker: (id) => api.put(`/workers/${id}/disable`),
    checkWorkerHealth: (id) => api.get(`/workers/${id}/health`)
};

// API endpoints for system
export const systemApi = {
    getSystemStatus: () => api.get('/system/status'),
    getSystemStats: () => api.get('/system/stats')
};

export default api;