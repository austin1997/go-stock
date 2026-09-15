<script setup>
import {onMounted, reactive, ref} from 'vue'
import {useRoute} from 'vue-router'
import {NButton, NCard, NForm, NFormItem, NInput, NTabPane, NTabs, useMessage} from 'naive-ui'
import {fetchAuthStatus, login, register} from '../platform/auth.js'

const route = useRoute()
const message = useMessage()
const tab = ref('login')
const allowRegister = ref(false)
const loading = ref(false)

const loginForm = reactive({ username: '', password: '' })
const registerForm = reactive({ username: '', password: '', confirm: '' })

onMounted(async () => {
  try {
    const st = await fetchAuthStatus()
    allowRegister.value = !!st.allowRegister
    if (!st.hasUsers) {
      tab.value = 'register'
    }
  } catch {
    allowRegister.value = false
  }
})

function redirectAfterAuth() {
  const to = typeof route.query.redirect === 'string' && route.query.redirect
    ? route.query.redirect
    : '/home'
  const path = to.startsWith('#') ? to.slice(1) : to
  window.location.hash = path.startsWith('/') ? '#' + path : '#/' + path
  window.location.reload()
}

async function onLogin() {
  if (!loginForm.username || !loginForm.password) {
    message.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    await login(loginForm.username, loginForm.password)
    redirectAfterAuth()
  } catch (e) {
    message.error(e.message || '登录失败')
  } finally {
    loading.value = false
  }
}

async function onRegister() {
  if (registerForm.password !== registerForm.confirm) {
    message.warning('两次输入的密码不一致')
    return
  }
  loading.value = true
  try {
    await register(registerForm.username, registerForm.password)
    redirectAfterAuth()
  } catch (e) {
    message.error(e.message || '注册失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="web-login">
    <n-card title="go-stock 网页版" style="width: 420px; max-width: 92vw">
      <n-tabs v-model:value="tab" type="segment" animated>
        <n-tab-pane name="login" tab="登录">
          <n-form @submit.prevent="onLogin">
            <n-form-item label="用户名">
              <n-input v-model:value="loginForm.username" placeholder="用户名" @keyup.enter="onLogin"/>
            </n-form-item>
            <n-form-item label="密码">
              <n-input v-model:value="loginForm.password" type="password" show-password-on="click"
                       placeholder="密码" @keyup.enter="onLogin"/>
            </n-form-item>
            <n-button type="primary" block :loading="loading" @click="onLogin">登录</n-button>
          </n-form>
        </n-tab-pane>
        <n-tab-pane v-if="allowRegister" name="register" tab="注册">
          <n-form @submit.prevent="onRegister">
            <n-form-item label="用户名">
              <n-input v-model:value="registerForm.username" placeholder="3-32 位字母数字或下划线"/>
            </n-form-item>
            <n-form-item label="密码">
              <n-input v-model:value="registerForm.password" type="password" show-password-on="click"
                       placeholder="至少 6 位"/>
            </n-form-item>
            <n-form-item label="确认密码">
              <n-input v-model:value="registerForm.confirm" type="password" show-password-on="click"
                       placeholder="再次输入密码" @keyup.enter="onRegister"/>
            </n-form-item>
            <n-button type="primary" block :loading="loading" @click="onRegister">创建账号</n-button>
          </n-form>
        </n-tab-pane>
      </n-tabs>
      <p class="hint">每个账号有独立的自选股、AI 配置和工作空间。请勿将服务暴露到公网。</p>
    </n-card>
  </div>
</template>

<style scoped>
.web-login {
  min-height: calc(100vh - 24px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 12px 80px;
}
.hint {
  margin: 16px 0 0;
  font-size: 12px;
  color: #888;
  line-height: 1.5;
}
</style>
