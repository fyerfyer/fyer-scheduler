import { ElNotification } from 'element-plus';

/**
 * Centralized error handling utility that provides consistent error handling
 * and improves UX with responsive error messages
 */
const ErrorHandler = {
    /**
     * Initialize error handler with global error catching
     */
    init() {
        // Global error handling
        window.addEventListener('error', (event) => {
            this.handleError({
                error: event.error,
                message: 'Unexpected application error occurred',
                silent: false
            });
            // Prevent default browser error handling
            event.preventDefault();
        });

        // Unhandled promise rejection handling
        window.addEventListener('unhandledrejection', (event) => {
            this.handleError({
                error: event.reason,
                message: 'Unhandled promise rejection',
                silent: false
            });
            // Prevent default browser error handling
            event.preventDefault();
        });
    },

    /**
     * Handle API error responses with appropriate user feedback
     * @param {Error} error - The error object from API response
     * @param {Object} options - Additional options
     * @returns {Object} Formatted error object
     */
    handleApiError(error, options = {}) {
        const formattedError = {
            message: error.message || 'An unexpected error occurred',
            status: error.status || 500,
            details: this.captureErrorDetails(error)
        };

        if (!options.silent) {
            this.showErrorNotification(
                options.title || 'API Error',
                formattedError.message,
                'error',
                options.duration
            );
        }

        // Log error to console in development
        if (process.env.NODE_ENV !== 'production') {
            console.error('API Error:', error);
        }

        // Store in vuex if available
        if (window.store && !options.noStore) {
            window.store.commit('SET_ERROR', formattedError.message);
        }

        return formattedError;
    },

    /**
     * General purpose error handler for all types of errors
     * @param {Object} options - Error handling options
     */
    handleError(options = {}) {
        const error = options.error || new Error(options.message || 'Unknown error');
        const message = options.message || error.message || 'An unexpected error occurred';

        if (!options.silent) {
            this.showErrorNotification(
                options.title || 'Error',
                message,
                options.type || 'error',
                options.duration
            );
        }

        // Log error to console in development
        if (process.env.NODE_ENV !== 'production') {
            console.error('Error:', error);
        }

        // Store in vuex if available
        if (window.store && !options.noStore) {
            window.store.commit('SET_ERROR', message);
        }
    },

    /**
     * Show an error notification to the user with responsive design
     * @param {String} title - Notification title
     * @param {String} message - Notification message
     * @param {String} type - Notification type (error, warning, info)
     * @param {Number} duration - Duration in milliseconds
     */
    showErrorNotification(title, message, type = 'error', duration = 4500) {
        // Apply responsive styles for the notification
        applyResponsiveStyles();

        // Use Element Plus notification
        ElNotification({
            title,
            message,
            type,
            duration,
            customClass: 'responsive-notification'
        });
    },

    /**
     * Capture and format error details for logging
     * @param {Error} error - Error object
     * @returns {Object} Formatted error details
     */
    captureErrorDetails(error) {
        return {
            message: error.message,
            stack: error.stack,
            code: error.code || 'UNKNOWN',
            timestamp: new Date().toISOString(),
            userAgent: navigator.userAgent,
            url: window.location.href
        };
    },

    /**
     * Check if a response has errors
     * @param {Object} response - API response object
     * @returns {Boolean} True if the response contains errors
     */
    hasErrors(response) {
        return !!(response && 
            (response.error || 
             (response.errors && response.errors.length) || 
             response.status === 'error'));
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
    styleElement.textContent = `
    .responsive-notification {
        max-width: 90vw;
        word-break: break-word;
    }
    
    @media (max-width: 768px) {
        .responsive-notification {
            max-width: 95vw;
            font-size: 0.9rem;
        }
    }
    `;

    // Append to document head if not already present
    if (!document.querySelector('style[data-responsive-styles]')) {
        styleElement.setAttribute('data-responsive-styles', 'true');
        document.head.appendChild(styleElement);
    }
}

// Add a method to initialize everything
ErrorHandler.setupApp = function() {
    this.init();
    applyResponsiveStyles();
};

export default ErrorHandler;