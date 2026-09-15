<script setup>
import {h, onMounted, reactive, ref} from 'vue'
import {NButton, NDataTable, NForm, NFormItem, NInput, NSwitch, useMessage} from 'naive-ui'
import {createUser, listUsers, setUserDisabled} from '../platform/auth.js'

const message = useMessage()
const loading = ref(false)
const users = ref([])
const form = reactive({ username: '', password: '', isAdmin: false })

const columns = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '用户名', key: 'username' },
  {
    title: '管理员',
    key: 'isAdmin',
    render(row) {
      return row.isAdmin ? '是' : '否'
    },
  },
  {
    title: '状态',
    key: 'disabled',
    render(row) {
      return row.disabled ? '已禁用' : '正常'
    },
  },
  {
    title: '操作',
    key: 'actions',
    render(row) {
      return h(NButton, {
        size: 'small',
        onClick: () => onToggle(row),
      }, { default: () => row.disabled ? '启用' : '禁用' })
    },
  },
]

async function refresh() {
  loading.value = true
  try {
    users.value = await listUsers()
  } catch (e) {
    message.error(e.message || '加载用户失败')
  } finally {
    loading.value = false
  }
}

async function onCreate() {
  try {
    await createUser(form.username, form.password, form.isAdmin)
    form.username = ''
    form.password = ''
    form.isAdmin = false
    message.success('已创建用户')
    await refresh()
  } catch (e) {
    message.error(e.message || '创建失败')
  }
}

async function onToggle(row) {
  try {
    await setUserDisabled(row.id, !row.disabled)
    await refresh()
  } catch (e) {
    message.error(e.message || '操作失败')
  }
}

onMounted(refresh)
</script>

<template>
  <div style="padding: 16px 16px 80px; max-width: 900px">
    <h3>用户管理</h3>
    <p style="color:#888;font-size:13px">新用户拥有独立的自选股、设置和 AI 模型配置。禁用后该账号无法登录。</p>
    <n-form inline :show-feedback="false" style="margin: 12px 0 20px">
      <n-form-item label="用户名">
        <n-input v-model:value="form.username" placeholder="用户名" style="width: 140px"/>
      </n-form-item>
      <n-form-item label="密码">
        <n-input v-model:value="form.password" type="password" placeholder="密码" style="width: 140px"/>
      </n-form-item>
      <n-form-item label="管理员">
        <n-switch v-model:value="form.isAdmin"/>
      </n-form-item>
      <n-form-item>
        <n-button type="primary" @click="onCreate">创建用户</n-button>
      </n-form-item>
    </n-form>
    <n-data-table :columns="columns" :data="users" :loading="loading" :bordered="false"/>
  </div>
</template>
