import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { applyDesignTokens } from '@mall/design-tokens'
import App from './App.vue'
import router from './router'
import './styles.css'

applyDesignTokens()
const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(VueQueryPlugin)
app.use(ElementPlus)
app.mount('#app')
