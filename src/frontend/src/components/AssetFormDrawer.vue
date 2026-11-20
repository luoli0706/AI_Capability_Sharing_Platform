<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { createAsset, getAssetDetail, updateAsset } from '@/api/asset'
import type {
  AgentConfig,
  AssetType,
  AssetVisibility,
  CreateAssetPayload,
  DomainItem,
  KbConfig,
  McpConfig,
  SkillConfig
} from '@/types/api'

const props = defineProps<{
  modelValue: boolean
  editAssetId: number | null
  domains: DomainItem[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val: boolean) => emit('update:modelValue', val)
})

const isEdit = computed(() => Boolean(props.editAssetId))
const formRef = ref<FormInstance>()
const loadingDetail = ref(false)
const submitting = ref(false)
const tagInput = ref('')

interface AssetFormState {
  asset_code: string
  asset_type: AssetType
  name: string
  summary: string
  tags: string[]
  domain_id: number | undefined
  visibility: AssetVisibility
  mcp_config: McpConfig
  skill_config: SkillConfig
  agent_config: AgentConfig
  kb_config: KbConfig
}

function createInitialForm(): AssetFormState {
  return {
    asset_code: '',
    asset_type: 'mcp',
    name: '',
    summary: '',
    tags: [],
    domain_id: props.domains[0]?.id,
    visibility: 'domain',
    mcp_config: {
      transport_type: 'sse',
      endpoint_url: '',
      auth_type: 'none'
    },
    skill_config: {
      skill_type: 'prompt',
      content: '',
      runtime_env: 'python3.11'
    },
    agent_config: {
      model_provider: 'openai',
      model_name: 'gpt-4o-mini',
      system_prompt: '',
      temperature: 0.7,
      max_tokens: 2048
    },
    kb_config: {
      chunk_size: 500,
      chunk_overlap: 50,
      embedding_model: 'text-embedding-3-small'
    }
  }
}

const form = reactive<AssetFormState>(createInitialForm())

const rules: FormRules = {
  name: [{ required: true, message: '请输入资产名称', trigger: 'blur' }],
  asset_code: [
    { required: true, message: '请输入资产编码', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_-]+$/, message: '仅支持字母、数字、下划线和连字符', trigger: 'blur' }
  ],
  asset_type: [{ required: true, message: '请选择资产类型', trigger: 'change' }],
  domain_id: [{ required: true, message: '请选择所属业务域', trigger: 'change' }],
  visibility: [{ required: true, message: '请选择可见范围', trigger: 'change' }],
  'mcp_config.endpoint_url': [
    {
      validator: (_rule, val, cb) => {
        if (form.asset_type === 'mcp' && !val?.trim()) {
          cb(new Error('请输入 MCP 服务端点 URL'))
          return
        }
        cb()
      },
      trigger: 'blur'
    }
  ],
  'skill_config.content': [
    {
      validator: (_rule, val, cb) => {
        if (form.asset_type === 'skill' && !val?.trim()) {
          cb(new Error('请输入技能内容'))
          return
        }
        cb()
      },
      trigger: 'blur'
    }
  ],
  'agent_config.model_name': [
    {
      validator: (_rule, val, cb) => {
        if (form.asset_type === 'agent' && !val?.trim()) {
          cb(new Error('请输入模型名称'))
          return
        }
        cb()
      },
      trigger: 'blur'
    }
  ]
}

function resetForm() {
  Object.assign(form, createInitialForm())
  tagInput.value = ''
  formRef.value?.clearValidate()
}

watch(
  () => [props.modelValue, props.editAssetId] as const,
  async ([isOpen, editId]) => {
    if (!isOpen) return
    resetForm()
    if (!editId) return

    loadingDetail.value = true
    try {
      const detail = await getAssetDetail(editId)
      form.asset_code = detail.asset_code
      form.asset_type = detail.asset_type
      form.name = detail.name
      form.summary = detail.summary || ''
      form.tags = Array.isArray(detail.tags) ? [...detail.tags] : []
      form.domain_id = detail.domain_id
      form.visibility = detail.visibility || 'domain'
      if (detail.mcp_config) {
        form.mcp_config = { ...form.mcp_config, ...detail.mcp_config }
      }
      if (detail.skill_config) {
        form.skill_config = { ...form.skill_config, ...detail.skill_config }
      }
      if (detail.agent_config) {
        form.agent_config = { ...form.agent_config, ...detail.agent_config }
      }
      if (detail.kb_config) {
        form.kb_config = { ...form.kb_config, ...detail.kb_config }
      }
    } catch {
      visible.value = false
    } finally {
      loadingDetail.value = false
    }
  }
)

function addTag() {
  const val = tagInput.value.trim()
  if (!val) return
  if (!form.tags.includes(val)) {
    form.tags.push(val)
  }
  tagInput.value = ''
}

function removeTag(tag: string) {
  form.tags = form.tags.filter((t) => t !== tag)
}

async function handleSubmit() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid || !form.domain_id) return

  const basePayload = {
    asset_code: form.asset_code.trim(),
    asset_type: form.asset_type,
    name: form.name.trim(),
    summary: form.summary.trim(),
    tags: form.tags,
    domain_id: form.domain_id,
    visibility: form.visibility
  }

  const payload: CreateAssetPayload = { ...basePayload }
  if (form.asset_type === 'mcp') {
    payload.mcp_config = { ...form.mcp_config, endpoint_url: form.mcp_config.endpoint_url.trim() }
  } else if (form.asset_type === 'skill') {
    payload.skill_config = { ...form.skill_config }
  } else if (form.asset_type === 'agent') {
    payload.agent_config = { ...form.agent_config }
  } else if (form.asset_type === 'kb') {
    payload.kb_config = { ...form.kb_config }
  }

  submitting.value = true
  try {
    if (isEdit.value && props.editAssetId) {
      await updateAsset(props.editAssetId, payload)
      ElMessage.success('资产更新成功')
    } else {
      await createAsset(payload)
      ElMessage.success('资产创建成功')
    }
    visible.value = false
    emit('saved')
  } catch {
    // Error shown by interceptor
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <el-drawer
    v-model="visible"
    :title="isEdit ? '编辑 AI 资产' : '新建 AI 资产'"
    size="640px"
    destroy-on-close
  >
    <div v-if="loadingDetail" class="drawer-loading">
      <el-skeleton :rows="8" animated />
    </div>

    <el-form
      v-else
      ref="formRef"
      :model="form"
      :rules="rules"
      label-position="top"
      class="asset-form"
    >
      <div class="form-section-title">基础信息</div>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="资产名称" prop="name">
            <el-input v-model="form.name" placeholder="例如：订单查询 MCP 服务" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="资产唯一编码" prop="asset_code">
            <el-input
              v-model="form.asset_code"
              :disabled="isEdit"
              placeholder="例如：order-query-mcp"
              class="mono-text"
            />
          </el-form-item>
        </el-col>
      </el-row>

      <el-row :gutter="16">
        <el-col :span="8">
          <el-form-item label="资产类型" prop="asset_type">
            <el-select v-model="form.asset_type" :disabled="isEdit" style="width: 100%">
              <el-option label="MCP 服务" value="mcp" />
              <el-option label="Skills 技能" value="skill" />
              <el-option label="Agent 实例" value="agent" />
              <el-option label="私有知识库" value="kb" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="所属业务域" prop="domain_id">
            <el-select v-model="form.domain_id" placeholder="选择业务域" style="width: 100%">
              <el-option
                v-for="d in domains"
                :key="d.id"
                :label="d.domain_name"
                :value="d.id"
              />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="可见范围" prop="visibility">
            <el-select v-model="form.visibility" style="width: 100%">
              <el-option label="业务域内可见" value="domain" />
              <el-option label="全平台公开" value="public" />
              <el-option label="仅自己可见" value="private" />
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item label="能力简介" prop="summary">
        <el-input
          v-model="form.summary"
          type="textarea"
          :rows="2"
          placeholder="简要说明该 AI 能力的功能、适用场景与输入输出特点"
        />
      </el-form-item>

      <el-form-item label="标签">
        <div class="tags-editor">
          <el-tag
            v-for="tag in form.tags"
            :key="tag"
            closable
            @close="removeTag(tag)"
          >
            {{ tag }}
          </el-tag>
          <el-input
            v-model="tagInput"
            size="small"
            placeholder="输入标签后按回车添加"
            style="width: 180px"
            @keyup.enter.prevent="addTag"
          />
        </div>
      </el-form-item>

      <el-divider />

      <!-- MCP Config -->
      <template v-if="form.asset_type === 'mcp'">
        <div class="form-section-title">MCP 服务配置</div>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="传输协议 (Transport)" prop="mcp_config.transport_type">
              <el-select v-model="form.mcp_config.transport_type" style="width: 100%">
                <el-option label="SSE" value="sse" />
                <el-option label="Streamable HTTP" value="streamable_http" />
                <el-option label="Stdio" value="stdio" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="鉴权方式" prop="mcp_config.auth_type">
              <el-select v-model="form.mcp_config.auth_type" style="width: 100%">
                <el-option label="无鉴权 (none)" value="none" />
                <el-option label="Bearer Token" value="bearer" />
                <el-option label="自定义 Header" value="header" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="服务端点 URL" prop="mcp_config.endpoint_url">
          <el-input
            v-model="form.mcp_config.endpoint_url"
            placeholder="https://mcp.example.com/sse"
            class="mono-text"
          />
        </el-form-item>
      </template>

      <!-- Skill Config -->
      <template v-else-if="form.asset_type === 'skill'">
        <div class="form-section-title">Skills 技能配置</div>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="技能类型" prop="skill_config.skill_type">
              <el-select v-model="form.skill_config.skill_type" style="width: 100%">
                <el-option label="Prompt 模板" value="prompt" />
                <el-option label="脚本 (Script)" value="script" />
                <el-option label="工作流 (Workflow)" value="workflow" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="运行环境" prop="skill_config.runtime_env">
              <el-input
                v-model="form.skill_config.runtime_env"
                placeholder="例如：python3.11 / nodejs20"
                class="mono-text"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="技能内容 / Prompt 定义" prop="skill_config.content">
          <el-input
            v-model="form.skill_config.content"
            type="textarea"
            :rows="6"
            placeholder="请输入 Prompt 模板、Python/JS 脚本或工作流定义..."
            class="mono-text"
          />
        </el-form-item>
      </template>

      <!-- Agent Config -->
      <template v-else-if="form.asset_type === 'agent'">
        <div class="form-section-title">Agent 实例配置</div>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="模型厂商" prop="agent_config.model_provider">
              <el-input
                v-model="form.agent_config.model_provider"
                placeholder="例如：openai / deepseek / qwen"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="模型名称" prop="agent_config.model_name">
              <el-input
                v-model="form.agent_config.model_name"
                placeholder="例如：gpt-4o-mini / deepseek-chat"
                class="mono-text"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="Temperature (0 ~ 2)">
              <el-input-number
                v-model="form.agent_config.temperature"
                :min="0"
                :max="2"
                :step="0.1"
                :precision="2"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Max Tokens">
              <el-input-number
                v-model="form.agent_config.max_tokens"
                :min="128"
                :max="128000"
                :step="256"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="System Prompt">
          <el-input
            v-model="form.agent_config.system_prompt"
            type="textarea"
            :rows="5"
            placeholder="设定 Agent 的角色定位、行为约束与输出规范..."
          />
        </el-form-item>
      </template>

      <!-- KB Config -->
      <template v-else-if="form.asset_type === 'kb'">
        <div class="form-section-title">私有知识库配置</div>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="切片大小 (Chunk Size)">
              <el-input-number
                v-model="form.kb_config.chunk_size"
                :min="100"
                :max="4000"
                :step="50"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="切片重叠 (Chunk Overlap)">
              <el-input-number
                v-model="form.kb_config.chunk_overlap"
                :min="0"
                :max="500"
                :step="10"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="Embedding 模型">
          <el-input
            v-model="form.kb_config.embedding_model"
            placeholder="例如：text-embedding-3-small / bge-large-zh"
            class="mono-text"
          />
        </el-form-item>
      </template>
    </el-form>

    <template #footer>
      <div class="drawer-footer">
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ isEdit ? '保存修改' : '确认创建' }}
        </el-button>
      </div>
    </template>
  </el-drawer>
</template>

<style scoped>
.drawer-loading {
  padding: 12px 0;
}

.form-section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ascp-text-primary);
  margin-bottom: 14px;
  padding-left: 8px;
  border-left: 3px solid var(--ascp-primary);
}

.tags-editor {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
