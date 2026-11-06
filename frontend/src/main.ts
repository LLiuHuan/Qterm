import { createApp } from 'vue'
import '@fontsource-variable/geist'
import App from './App.vue'

import './styles/index.css'
import { ensureUpdateChecks } from './features/updates/updateStore'

createApp(App).mount('#app')

ensureUpdateChecks()
