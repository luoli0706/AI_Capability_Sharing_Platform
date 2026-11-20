# AI 能力共享平台（ASCP）

> 面向研发团队的多 Agent 协同开发基础设施，对 **MCP 服务、Skills 技能、Agent 实例、文件式 Markdown 知识库** 四类核心 AI 资产进行统一托管、版本管理、域隔离治理与可审计调用。

---

## 核心特性

- **组织中心与业务域隔离**
  - 支持创建组织与组织成员管理，组织内所有成员自动共享组织公有域访问与调用权限。
  - 业务域具备唯一域 ID（`DOM-XXXXXXXX`）与 `public / private` 可见性控制：公开域可在域广场展示与名称搜索，私密域仅支持通过唯一域 ID 精确查找；仅域创建者及其指定的管理员可管理域与成员。
- **四类 AI 资产统一托管**
  - **MCP 服务**：支持官方 MCP Registry（`registry.modelcontextprotocol.io`）检索导入、6 种主流 JSON 格式（`mcpServers` / `servers` / `server.json` / 注册表包裹格式）解析、快速配置模板、健康检查与 `mcpServers` 客户端配置一键复制。
  - **Skills 技能**：兼容 `skills.sh` 规范，支持 `skills.sh` URL 一键导入、`SKILL.md` / `.zip` 技能包上传解析以及 `.zip` / `SKILL.md` 标准化下载。
  - **Agent 实例**：支持纯 Markdown、Frontmatter Markdown、Dify YAML DSL、OpenAI / 通用 JSON 以及 ModelScope / Agentverse URL 导入，支持关联域内 MCP、Skills 与知识库，并可导出 `.agent.md` / `.agent.json`。
  - **文件式 Markdown 知识库**：支持多级文件夹与 `.md` 批量/文件夹上传、XSS 安全的在线 Markdown 渲染与源码预览、关键词检索，以及单文件/文件夹递归/整库打包下载（下载前弹出二次确认并展示预定下载体积）。
- **审批中心与调用审计**
  - 统一审批中心支持 **资产发布审批**（域所有者/管理员审批上线或驳回退回草稿）与 **跨域权限申请**（两级审批流）。
  - 支持 `JWT` 与 `X-API-Key` 双模式鉴权调用代理，全链路记录调用审计日志（`asset_invoke_log`）。

---

## 技术栈

| 层级 | 技术选型 |
| :--- | :--- |
| **后端** | Go + Echo v4 + GORM + MySQL 8.0 + Zap + Viper + JWT |
| **前端** | Vue 3 (Composition API) + TypeScript + Vite + Pinia + Element Plus + ECharts |
| **网关** | Nginx（反向代理、多实例负载均衡、限流与 SSE 流式透传） |
| **容器化** | Docker + Docker Compose |

---

## 目录结构

```text
AI_Capability_Sharing_Platform/
├── docs/                       # 需求、系统设计方案与 P0-P2 开发方案
├── docker/                     # Dockerfile 与 docker-compose.dev.yml
├── src/
│   ├── backend/                # Go Echo 后端服务
│   ├── frontend/               # Vue 3 + TypeScript 前端应用
│   └── nginx/                  # Nginx 开发/生产配置模板
├── LICENSE                     # Apache License 2.0
└── README.md
```

---

## 本地快速启动

### 1. 数据库准备
确保本地已启动 MySQL 8.0，并参考 `src/backend/.env.example` 配置 `src/backend/.env`（或直接修改 `src/backend/config/config.yaml`）。

### 2. 启动后端
```bash
cd src/backend
go run ./cmd/server
```
后端默认监听 `:8080`，启动时自动执行 GORM `AutoMigrate` 与角色种子初始化。

### 3. 启动前端
```bash
cd src/frontend
npm install
npm run dev
```
前端默认监听 `:5173`。

### 4. 通过 Nginx 访问（推荐）
将 [src/nginx/ASCP_nginx.dev.conf.example](src/nginx/ASCP_nginx.dev.conf.example) 挂载或配置至本地 Nginx 后重载，访问 `http://localhost` 即可。

### 5. Docker Compose 一键启动（生产式容器编排）
```bash
cd docker
docker compose -f docker-compose.dev.yml up -d --build
```

---

## 相关文档

- [系统设计方案](docs/系统设计方案.md)
- [P0–P2 分级开发方案](docs/P0-P2开发方案.md)

---

## 开源协议

本项目基于 [Apache License 2.0](LICENSE) 开源。
