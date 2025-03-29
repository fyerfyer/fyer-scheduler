import { ElNotification } from 'element-plus';
import store from '@/store';

/**
 * Centralized error handling utility that provides consistent error handling
 * and improves UX with responsive error messages
 */
const ErrorHandler = {
    /**
     * Initialize error handler with global error catching
     */
    init() {
        // Add global error handler for uncaught exceptions
        window.addEventListener('error', (event) => {
            console.error('Uncaught error:', event.error);
            this.handleError({
                title: 'Unexpected Error',
                message: 'An unexpected error occurred. Please try again later.',
                error: event.error,
                silent: true // Don't show notification for every uncaught error
            });
        });

        // Add global promise rejection handler
        window.addEventListener('unhandledrejection', (event) => {
            console.error('Unhandled promise rejection:', event.reason);
            this.handleError({
                title: 'Operation Failed',
                message: 'Failed to complete the requested operation.',
                error: event.reason,
                silent: true
            });
        });

        // Add axios interceptor for network errors
        // This is commented out because it would require modifying api.js
        // but can be added there if needed
        /*
        api.interceptors.response.use(
          response => response,
          error => {
            this.handleApiError(error);
            return Promise.reject(error);
          }
        );
        */
    },

    /**
     * Handle API error responses with appropriate user feedback
     * @param {Error} error - The error object from API response
     * @param {Object} options - Additional options
     * @returns {Object} Formatted error object
     */
    handleApiError(error, options = {}) {
        // Default options
        const defaults = {
            showNotification: true,
            logToConsole: true,
            redirectOnAuthError: true
        };

        const config = { ...defaults, ...options };
        let title = 'Error';
        let message = 'An unexpected error occurred';
        let type = 'error';
        let errorCode = null;

        // Extract error details from API response if available
        if (error.response) {
            const status = error.response.status;
            errorCode = status;

            // Handle different HTTP status codes
            switch (status) {
                case 400:
                    title = 'Invalid Request';
                    message = error.response.data?.error?.message || 'The request could not be processed';
                    break;
                case 401:
                    title = 'Authentication Required';
                    message = 'Your session may have expired. Please log in again.';
                    // Could redirect to login page here
                    if (config.redirectOnAuthError) {
                        // Redirect to login page if needed
                        // window.location.href = '/login';
                    }
                    break;
                case 403:
                    title = 'Access Denied';
                    message = 'You do not have permission to perform this action';
                    break;
                case 404:
                    title = 'Not Found';
                    message = 'The requested resource was not found';
                    break;
                case 409:
                    title = 'Conflict';
                    message = 'This operation could not be completed due to a conflict';
                    break;
                case 429:
                    title = 'Too Many Requests';
                    message = 'Please try again later';
                    type = 'warning';
                    break;
                case 500:
                case 502:
                case 503:
                case 504:
                    title = 'Server Error';
                    message = 'The server encountered an error. Please try again later.';
                    break;
                default:
                    title = `Error (${status})`;
                    message = error.response.data?.error?.message || 'An error occurred while processing your request';
            }
        } else if (error.request) {
            // Request was made but no response received
            title = 'Network Error';
            message = 'Unable to connect to the server. Please check your internet connection.';
            type = 'warning';
        }

        // Log the error
        if (config.logToConsole) {
            console.error(`${title}: ${message}`, error);
        }

        // Store error in Vuex if needed
        if (error.response && error.response.data && error.response.data.error) {
            store.commit('SET_ERROR', {
                code: errorCode,
                message: message,
                details: error.response.data.error.details || error.message
            });
        }

        // Show notification
        if (config.showNotification) {
            this.showErrorNotification(title, message, type);
        }

        return {
            title,
            message,
            type,
            code: errorCode,
            originalError: error
        };
    },

    /**
     * General purpose error handler for all types of errors
     * @param {Object} options - Error handling options
     */
    handleError(options = {}) {
        const {
            title = 'Error',
            message = 'An error occurred',
            error = null,
            silent = false,
            type = 'error',
            duration = 4500,
            store = true
        } = options;

        // Log the error to console
        if (error) {
            console.error(`${title}: ${message}`, error);
        } else {
            console.error(`${title}: ${message}`);
        }

        // Optionally store in Vuex
        if (store && window.store) {
            window.store.commit('SET_ERROR', {
                message: message,
                details: error ? error.message : undefined
            });
        }

        // Show notification unless silent mode is enabled
        if (!silent) {
            this.showErrorNotification(title, message, type, duration);
        }

        return { title, message, error };
    },

    /**
     * Show an error notification to the user with responsive design
     * @param {String} title - Notification title
     * @param {String} message - Notification message
     * @param {String} type - Notification type (error, warning, info)
     * @param {Number} duration - Duration in milliseconds
     */
    showErrorNotification(title, message, type = 'error', duration = 4500) {
        // Adjust notification positioning and style based on screen size
        const isMobile = window.innerWidth < 768;

        // Configuration for responsive notification
        const config = {
            title,
            message,
            type,
            duration,
            position: isMobile ? 'bottom' : 'top-right',
            customClass: isMobile ? 'mobile-notification' : '',
            showClose: true,
            dangerouslyUseHTMLString: false,
            offset: isMobile ? 10 : 30
        };

        // Show the notification
        ElNotification(config);
    },

    /**
     * Capture and format error details for logging
     * @param {Error} error - Error object
     * @returns {Object} Formatted error details
     */
    captureErrorDetails(error) {
        return {
            message: error.message || 'Unknown error',
            stack: error.stack,
            timestamp: new Date().toISOString(),
            browser: navigator.userAgent,
            url: window.location.href
        };
    },

    /**
     * Check if a response has errors
     * @param {Object} response - API response object
     * @returns {Boolean} True if the response contains errors
     */
    hasErrors(response) {
        return response && !response.success && !!response.error;
    }
};

/**
 * Apply responsive styles for improved mobile experience
 * This adds responsive CSS variables and utility classes to the document
 */
function applyResponsiveStyles() {
    // Create a style element
    const styleElement = document.createElement('style');
    styleElement.type = 'text/css';

    // Define responsive CSS variables and utility classes
    const css = `
    :root {
      --content-padding: 20px;
      --card-border-radius: 4px;
      --font-size-mobile: 14px;
      --header-height: 60px;
      --sidebar-width: 220px;
    }
    
    @media (max-width: 768px) {
      :root {
        --content-padding: 10px;
        --card-border-radius: 2px;
      }
      
      /* Mobile-specific notification styling */
      .mobile-notification {
        width: 100% !important;
        max-width: 100% !important;
        padding: 8px !important;
        margin: 0 !important;
      }
      
      /* Responsive table improvements */
      .responsive-table {
        display: block;
        width: 100%;
        overflow-x: auto;
        -webkit-overflow-scrolling: touch;
      }
      
      /* Make cards full width on mobile */
      .el-card {
        margin-left: -5px !important;
        margin-right: -5px !important;
        border-radius: 0 !important;
      }
      
      /* Improve button touch targets on mobile */
      .el-button {
        padding: 10px 16px !important;
        min-height: 40px !important;
      }
      
      /* Stack form controls on mobile */
      .el-form-item {
        display: flex;
        flex-direction: column;
      }
      
      .el-form-item__label {
        text-align: left !important;
        width: 100% !important;
        padding: 0 0 8px !important;
      }
      
      /* Mobile grid system helpers */
      .mobile-stack {
        display: flex;
        flex-direction: column !important;
      }
      
      .mobile-full-width {
        width: 100% !important;
      }
      
      .mobile-center {
        text-align: center !important;
      }
      
      .mobile-hidden {
        display: none !important;
      }
      
      .mobile-visible {
        display: block !important;
      }
    }
    
    /* Additional responsive utility classes */
    .text-truncate {
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    
    .breakable {
      word-break: break-word;
    }
    
    /* Responsive spacing utilities */
    .responsive-margin {
      margin: var(--content-padding);
    }
    
    .responsive-padding {
      padding: var(--content-padding);
    }
  `;

    // Add the styles to the document
    styleElement.textContent = css;
    document.head.appendChild(styleElement);

    // Add mobile detection class to body
    function updateBodyClass() {
        if (window.innerWidth < 768) {
            document.body.classList.add('mobile-device');
        } else {
            document.body.classList.remove('mobile-device');
        }
    }

    // Update on resize
    window.addEventListener('resize', updateBodyClass);
    updateBodyClass();
}

// Add a method to initialize everything
ErrorHandler.setupApp = function() {
    // Initialize error handler
    this.init();

    // Apply responsive styles
    applyResponsiveStyles();

    // Return the initialized handler
    return this;
};

export default ErrorHandler;