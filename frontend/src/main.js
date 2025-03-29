import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import store from './store'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import '@/assets/styles/main.scss'
import ErrorHandler from '@/utils/ErrorHandler'

// Initialize error handler
ErrorHandler.setupApp()

// Make store available globally for error handling
window.store = store

const app = createApp(App)

app.use(router)
app.use(store)
app.use(ElementPlus)

app.mount('#app')