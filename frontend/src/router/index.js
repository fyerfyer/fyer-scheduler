import { createRouter, createWebHistory } from 'vue-router';
import store from '@/store';

// Routes configuration
const routes = [
    {
        path: '/',
        name: 'Dashboard',
        component: () => import('../views/Dashboard.vue'),
        meta: {
            title: 'Dashboard',
            requiresInit: true
        }
    },

    // Job routes
    {
        path: '/jobs',
        name: 'Jobs',
        component: () => import('../views/jobs/JobsView.vue'),
        meta: {
            title: 'Jobs',
            requiresInit: true
        }
    },
    {
        path: '/jobs/create',
        name: 'JobCreate',
        component: () => import('../views/jobs/JobCreateView.vue'),
        meta: {
            title: 'Create Job',
            requiresInit: true
        }
    },
    {
        path: '/jobs/:id',
        name: 'JobDetail',
        component: () => import('../views/jobs/JobDetailView.vue'),
        props: true,
        meta: {
            title: 'Job Details',
            requiresInit: true
        }
    },
    {
        path: '/jobs/:id/edit',
        name: 'JobEdit',
        component: () => import('../views/jobs/JobEditView.vue'),
        props: true,
        meta: {
            title: 'Edit Job',
            requiresInit: true
        }
    },

    // Log routes
    {
        path: '/logs',
        name: 'Logs',
        component: () => import('../views/logs/LogsView.vue'),
        meta: {
            title: 'System Logs',
            requiresInit: true
        }
    },
    {
        path: '/logs/execution/:id',
        name: 'ExecutionLog',
        component: () => import('../views/logs/ExecutionLogView.vue'),
        props: true,
        meta: {
            title: 'Execution Logs',
            requiresInit: true
        }
    },
    {
        path: '/logs/job/:jobId',
        name: 'JobLogs',
        component: () => import('../views/logs/JobLogsView.vue'),
        props: true,
        meta: {
            title: 'Job Logs',
            requiresInit: true
        }
    },

    // Worker routes
    {
        path: '/workers',
        name: 'Workers',
        component: () => import('../views/workers/WorkersView.vue'),
        meta: {
            title: 'Workers',
            requiresInit: true
        }
    },
    {
        path: '/workers/:id',
        name: 'WorkerDetail',
        component: () => import('../views/workers/WorkerDetailView.vue'),
        props: true,
        meta: {
            title: 'Worker Details',
            requiresInit: true
        }
    },

    // Settings route
    {
        path: '/settings',
        name: 'Settings',
        component: () => import('../views/SettingsView.vue'),
        meta: {
            title: 'Settings',
            requiresInit: true
        }
    },

    // Error pages
    {
        path: '/404',
        name: 'NotFound',
        component: () => import('../views/errors/NotFoundView.vue'),
        meta: {
            title: 'Page Not Found'
        }
    },
    {
        path: '/error',
        name: 'Error',
        component: () => import('../views/errors/ErrorView.vue'),
        meta: {
            title: 'Error Occurred'
        }
    },

    // Catch-all route for any unmatched routes
    {
        path: '/:pathMatch(.*)*',
        redirect: '/404'
    }
];

// Create router instance
const router = createRouter({
    history: createWebHistory(process.env.BASE_URL),
    routes,
    // Scroll to top when navigating to a new route
    scrollBehavior(to, from, savedPosition) {
        if (savedPosition) {
            return savedPosition;
        } else {
            return { top: 0 };
        }
    }
});

// Global navigation guard
router.beforeEach(async (to, from, next) => {
    // Update page title
    document.title = to.meta.title
        ? `${to.meta.title} | Fyer Scheduler`
        : 'Fyer Scheduler';

    // Handle routes that require app initialization
    if (to.meta.requiresInit && !store.state.appReady) {
        try {
            // Initialize app if not already done
            await store.dispatch('initializeApp');
            next();
        } catch (error) {
            console.error('Failed to initialize app:', error);
            next({ name: 'Error', params: { error: 'Failed to initialize application' } });
        }
    } else {
        next();
    }
});

// Handle errors during navigation
router.onError((error) => {
    console.error('Navigation error:', error);
    if (error.name === 'ChunkLoadError') {
        // Handle chunk load errors (when a route component fails to load)
        window.location.reload();
    }
});

export default router;