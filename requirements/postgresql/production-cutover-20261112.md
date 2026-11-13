# 47.33 PostgreSQL 生产切换记录

2026-11-12，用户明确要求开始切换 47.33；此前已明确接受切换、试运行和回滚窗口的新增计费丢失。此次实际保留了最终快照的全部记录，没有主动清零历史账务，也没有执行回滚。

## 结果

- 地址：`http://192.168.47.33:53000`，端口和前端不变。
- 当前数据库：本机独立 PostgreSQL 17.11，库／角色 `new_api_ty`，仅监听回环地址 5432。
- 发布目录：`/data/app/new-api-ty/update_20261112_120307/`。
- 新网关 PID：102956。恢复健康前停机约 **52.14 秒**，2026-11-12 12:07 恢复服务。
- 应用打开／闲置连接上限 32/16，生命周期 300 秒；锁等待 1000ms，单语句超时 30000ms。无 Redis、无内存延迟批量记账，退出超时 15 秒。
- 迁移 49 张表、369,537 行，每表计数与内容 SHA256 校验通过。源恢复点 SHA256：`d5f31d820726ac5f387a4145ac9bb1e04b1e7a71cad054ba8ca38536f0f3d4fb`。
- 最终快照含 208 用户、7 渠道、13 API Key、183,954 条日志、181,051 条配额账单；部门、权限、认证、知识库、价格版本和其他配置完整迁移。
- 模型定价和 DS 价格规则没有改变，汇率保持 **7.3**，历史没有重新计价。

旧服务在 TERM 后 30 秒内未全部退出，发布脚本核对可执行路径后终止了剩余旧网关 PID 94510，再生成最终恢复点。在途请求可能中断，按已授权的切换窗口处理；未停止其他服务。旧 SQLite 及恢复点均保留。快照内 6 笔未完成预扣、合计 116,824 quota，原样保留，未擅自重算这些历史状态。

## 上线核验

- 内网访问 `/api/status` 为 200，管理员登录成功。
- 管理员部门配额、用量分析、DS 账单、DS 价格规则接口成功；前端入口和实际 JS 资源哈希核对通过。
- 核对运行进程实际使用 PostgreSQL DSN，日志库与主库统一。
- 上线后通过已配置的 Ucloud DS 渠道完成一笔微量真实请求。使用用户 #194 既有 Key，仅让模型回复 OK，未新增或调整用户、Key、余额、权限或价格。
- 请求 200；输入 39、输出 8、缓存命中 0，原价人民币 0.000071，按 7.3 换算及原有舍入计为 **5 quota**。消费日志 183959 与配额账单均为 5，状态 settled，扣部门配额，钱包不参与。
- 该次检查时 3 个派生周期计数与原始账单一致，差异数 0；数据库连接 6、锁等待 0。此为当时读数，不是持续容量承诺。

## 发布内容与记录

后端 Linux amd64、CGO 关闭，由本地已验证代码构建；生产源码基于 `c78a0711eb24452a23cebaee7532412d48f74349` 加本次变更。此次没有 Git commit/push，不能把该基线提交号当成新二进制的完整版本；准确内容由上传清单和各文件 SHA256 标识。

发布目录包含：`payload.tar.gz`、`uploaded/`、`upload-manifest.json`（在 uploaded 内）、`source-before.tar.gz`、`preflight.json`、`baseline.json`、`migration-report.json`、`migration.log`、`cutover.log`、`verification-report.json`、`post-cutover-api-verification.json`、`old-reservations-check.json`、`UPDATE.txt`。后续文档更新另列 `post-release-documentation-manifest.json`。本地副本在项目父目录 `.deployment/update_20261112_120307/`。

恢复点 `sqlite-rollback/` 包含旧二进制、原 `.env`、完整最终 SQLite 备份及 `manifest.json`。目录及凭据只允许管理员读取，不进入 Git。PG 连接凭据继续位于 `/data/app/postgresql/connection.env`，没有写入源码或报告。

## 需要回滚时

```bash
bash /data/app/new-api-ty/update_20261112_120307/rollback-tools/rollback_to_sqlite.sh \
  /data/app/new-api-ty /data/app/new-api-ty/update_20261112_120307
```

这会停网关，恢复切换前旧程序、原配置及 SQLite，再启动并检查健康；PG 保留供排查。用户接受该窗口新增计费丢失，因此不反向同步试运行账务。先确认窗口内没有未记录的渠道、部门、用户权限等非计费变更；这些不属于允许丢失的范围。源代码不执行破坏性 Git reset，运行版本以恢复的二进制为准。

截至上线核验，自动回滚没有触发。后续正常重启继续使用根目录原有 `start.sh` / `stop.sh`，`.env` 已持久保存 PG 配置。
