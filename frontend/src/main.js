import { createApp } from 'vue'
import 'femtocrank/style.css'
import './style.css'
import App from './App.vue'
import router from './router'
import { initCompactMode } from './composables/compactMode'

initCompactMode()
createApp(App).use(router).mount('#app')

