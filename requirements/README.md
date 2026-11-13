# TIANYAN Demo 需求管理

本目录用于持续记录 TIANYAN 内部 Demo 的需求、范围、验收状态和技术评估。

## 永久约束

- 任何需求的新增、删除、修改、范围调整和验收结论，都必须在代码变更的同一批次同步更新本目录中的对应文档。
- 需求文档是后续实现和验收的依据；代码与文档不一致时，必须先补齐文档并明确差异。
- 隐藏功能默认只移除 UI 入口，保留路由、接口、源码和数据；只有需求明确要求删除时才执行清理。
- 环境固定隔离（2026-11-13 用户确认）：本地网关连接测试库 `new_api_ty_test_20261113`，本地账单连接测试报表库 `new_api_ty_test_reports_20261113`；47.33 网关连接生产库 `new_api_ty`。测试数据不得回写生产，推送代码不代表授权发布生产。详见 `postgresql/remote-test-copy-20261113.md`。

## 当前文档

- `postgresql/README.md`：PostgreSQL 独立部署、本地并发／账务验证、47.33 生产切换及允许丢弃试运行计费的回滚方案。

- `page-scrolling/README.md`：公共页面滚动修复，确保长页面底部内容可达并保留聊天自身滚动。

- `deepseek-bill-demo/README.md`：DeepSeek Flash 专用原厂峰谷计费、规则版本、共用部门配额及历史回算，管理员可查全员、普通用户仅可查本人。

- `deepseek-bill-demo/async-reports.md`：独立进程／独立数据库生成 DS 账单，查询不扫描调用日志，包含延迟提示、游标分页及 AI 调用隔离验证。

- `deepseek-bill-demo/production-rollout-20261113.md`：DS 异步报表生产发布，确认 47.33 保持生产库、独立报表库、限速补算、真实调用对账及保留 PG 数据的回退方法。

- `sqlite-concurrency/README.md`：SQLite 并发配额写入、结算及用量汇总锁冲突修复。

- `chat-attachments/README.md`：聊天知识库和 Skill 收起、临时文本与图片附件、剪贴板截图及禁止落盘。

- `department-models/README.md`：部门模型授权、默认禁止规则与部门策略入口。

- `usage-analysis/README.md`：管理员独立用量分析、部门/人员/模型统计与本地模型商业采购支出估算。

- `quota/README.md`：企业部门周期配额、钱包兜底、账单及余额调整审计。

- `users/README.md`：管理员维护平级部门、用户归属和部门筛选。

- `chat/README.md`：Chat 功能、导航入口收口及实现状态。
- `auth/ldap-ad-assessment.md`：保留现有用户体系并接入 LDAP/AD 的可行性评估。
- `knowledge-base/README.md`：知识库目录扫描、文件授权、Chat 检索以及树形交互需求。
- `skill/README.md`：大模型生成受控指令并实时更新本地展示页的 Skill Demo。
