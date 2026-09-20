import {createRouter, createWebHashHistory} from 'vue-router'

import stockView from '../components/stock.vue'
import settingsView from '../components/settings.vue'
import aboutView from "../components/about.vue";
import fundView from '../components/fund.vue'
import marketView from '../components/market.vue'
import agentChat from "../components/agent-chat.vue"
import research from "../components/researchIndex.vue";
import cronTaskManager from "../components/cron-task-manager.vue"
import mcpServerManager from "../components/mcp-server-manager.vue"
import klineAnalysis from "../components/kline-analysis.vue"
import aiConfigManager from "../components/ai-config-manager.vue"
import userProfile from "../components/user-profile.vue"
import homeView from "../components/Home.vue";
import dailyReview from "../components/DailyReview.vue"
import morningStrategy from "../components/MorningStrategy.vue"
import webLogin from "../components/web-login.vue"
import webUsers from "../components/web-users.vue"
import {fetchMe, isWebMode} from "../platform/auth.js"

const routes = [
    { path: '/', redirect: '/home'},
    { path: '/home', component: homeView,name: 'home'},
    { path: '/stock', component: stockView,name: 'stock'},
    { path: '/fund', component: fundView,name: 'fund' },
    { path: '/settings', component: settingsView,name: 'settings' },
    { path: '/about', component: aboutView,name: 'about' },
    { path: '/market', component: marketView,name: 'market' },
    { path: '/agent', component: agentChat,name: 'agent' },
    { path: '/research', component: research,name: 'research' },
    { path: '/cron-tasks', component: cronTaskManager,name: 'cronTasks' },
    { path: '/mcp-servers', component: mcpServerManager,name: 'mcpServers' },
    { path: '/kline-analysis', component: klineAnalysis,name: 'klineAnalysis' },
    { path: '/ai-configs', component: aiConfigManager,name: 'aiConfigs' },
    { path: '/user-profile', component: userProfile,name: 'userProfile' },
    { path: '/daily-review', component: dailyReview,name: 'dailyReview' },
    { path: '/morning-strategy', component: morningStrategy,name: 'morningStrategy' },
    { path: '/login', component: webLogin, name: 'login' },
    { path: '/users', component: webUsers, name: 'users' },
]

const router = createRouter({
    history: createWebHashHistory(),
    routes,
})

let cachedUser = null

export function getCachedUser() {
    return cachedUser
}

export function setCachedUser(user) {
    cachedUser = user
}

router.beforeEach(async (to) => {
    if (!isWebMode()) {
        if (to.name === 'login' || to.name === 'users') {
            return { name: 'home' }
        }
        return true
    }
    if (to.name === 'login') {
        const me = await fetchMe().catch(() => null)
        cachedUser = me
        if (me) {
            return { path: '/' }
        }
        return true
    }
    const me = await fetchMe().catch(() => null)
    cachedUser = me
    if (!me) {
        return { name: 'login', query: { redirect: to.fullPath } }
    }
    if (to.name === 'users' && !me.isAdmin) {
        return { name: 'home' }
    }
    return true
})

export default router
