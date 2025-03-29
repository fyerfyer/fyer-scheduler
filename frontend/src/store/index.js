import { createStore } from 'vuex';
import jobs from './modules/jobs';
import logs from './modules/logs';
import workers from './modules/workers';

export default createStore({
    state: {
        appReady: false
    },
    mutations: {
        SET_APP_READY(state, isReady) {
            state.appReady = isReady;
        }
    },
    actions: {
        initializeApp({ commit }) {
            // Perform any app initialization here
            commit('SET_APP_READY', true);
        }
    },
    modules: {
        jobs,
        logs,
        workers
    }
});