# DS 异步报表生产发布记录

2026-11-13，用户在本地测试库验证页面并推送代码后，明确授权发布到 47.33，重点要求生产网关继续使用生产库。此次没有上传本地测试库、测试 `.env`、测试账号或用量数据。

## 已完成的发布

- 生产网关地址仍为 `http://192.168.47.33:53000`。
- 发布代码：`5d288c72fae1fdc3e8f5405bb9195292d9d0c08d`；生产构建版本 `v0.0.0-5d288c72`。
- 发布目录：`/data/app/new-api-ty/update_20261113_171503/`，含上传清单、代码与前端文件哈希、旧程序／旧配置／旧源码备份、生产 PG 一致性备份、安装及验证记录。
- 2026-11-13 17:24:31 恢复健康，切换耗时约 **12.45 秒**；旧网关 PID 102956 正常停止，新网关 PID **115759**。
- 生产 PostgreSQL 未重启，原业务库 **`new_api_ty`** 和原业务角色保留。发布前后 `SQL_DSN` 字符串完全一致；所有旧 `.env` 项保留，仅新增生产报表 reader DSN。
- 生产数据未被替换、导入测试记录或重新扣费；定价 options 的哈希一致，DS 规则和全局汇率仍为 **7.3**。启动后再次检查 `/proc/<pid>/environ`，确认实际运行进程连接生产库。
- 本地测试网关继续连接 `new_api_ty_test_20261113`，本地报表继续连接 `new_api_ty_test_reports_20261113`。此次没有修改这些测试连接。

网关程序、独立 worker 及前端均从已推送代码构建。上传 64 个变更源文件和 234 个前端文件；安装前校验了服务器此前未提交的 PG 修复文件与已留档哈希一致，才同步源码到新提交，避免丢弃未知修改。

## 独立生产报表库及权限

新建 `new_api_ty_reports`，报表明细、每日汇总与导入位置只在这个库中，验证业务库里没有新报表表。

| 用途 | 库 | 角色 | 权限与连接上限 |
| --- | --- | --- | --- |
| 业务网关 | `new_api_ty` | 原 `new_api_ty` | 原配置及 32/16 连接池保持 |
| 后台读取源 | `new_api_ty` | `new_api_bill_source` | 仅 public USAGE、logs SELECT；默认只读；角色上限 2，worker 池 1 |
| 报表写入 | `new_api_ty_reports` | `new_api_bill_writer` | 仅独立报表库拥有者；角色上限 2，worker 池 1 |
| 网关报表读取 | `new_api_ty_reports` | `new_api_bill_reader` | 仅报表表 SELECT；默认只读；角色上限 4，网关池 2 |

三个新增角色均无生产用户、Key、渠道、options 或部门计费表的读写权限；source 例外仅为 logs SELECT。报表库撤销 public CONNECT，按用途显式授权。新增三条精确匹配库、账号及 `127.0.0.1/32` 的 HBA SCRAM 规则，备份旧文件、检查语法并重载；没有扩大 PG 监听地址或开放端口。

凭据分别位于 root 私有的 `report-worker.env`、生产 `app/.env` 和本次发布目录的私有配置文件，不写入源码／本文。

## 后台服务和首次补算

服务名 `deepseek-report-worker.service`，已启用开机启动并运行。默认每批最多 200 条源日志、结束后休息 2 秒；source/target 各一个连接。systemd 使用 DynamicUser、只读系统、CPUQuota 25%、MemoryMax 256MiB、GOMAXPROCS 1 和低优先级。

systemd 的 `/opt/new-api-ty-deepseek-report-worker` 是运行时只读绑定路径，静态校验时用同一个宿主可执行文件验证命令路径；实际安装的 unit 保留隔离挂载路径，并已验证可启动、生成首批报表、保持心跳。初次样本 RSS 约 12.4MiB，6 tasks。

初始源位置约 217,229。**首次历史补算尚需等待**：2026-11-13 17:28:32 已扫描至 34,800，约 16.02%。API 返回 `configured=true/history_complete=false/worker_running=true`，新前端显示进度；不会把未完成的账单显示成完整零金额。此进度只是记录位置近似值，不是最终封账证明。用户可继续调用 AI，首次导入结束后正常查看完整账单。

后台可以单独停止：

```bash
systemctl status deepseek-report-worker.service
systemctl stop deepseek-report-worker.service
systemctl start deepseek-report-worker.service
```

停止 worker 不需要停止业务网关。报表库应随业务库一起纳入备份；源日志还保留时可重建报表，清理源日志后不能丢弃报表历史。

## 发布后核验

- 管理员登录、部门额度、用量分析、DS 报表及定价规则接口均成功，新前端静态资产的 HTTP 内容哈希与发布包一致。
- 使用现有用户 #194 的既有 Key 发送 **一次**极小真实 Ucloud DS 请求，没有创建或修改渠道、Key、权限或额度规则：HTTP 200，输入 39、输出 22、缓存 0；金额 CNY `0.000254`，FX 7.3，扣额 17 quota，部门账单 settled 且金额一致。
- 17:24:31～17:28:32 观察到 HTTP 200 共 70 次、404 共 5 次，没有新的 ERR／数据库异常；这包含网页和 API 请求，不把 HTTP 200 数量冒充全部模型生成成功次数。
- 17:28:32 生产库连接数 8、锁等待 0、用量队列积压 0。真实调用验证及后台补算均未改变业务库连接目标。

以上是实际样本，不是无限并发、无限数据量或上游可用性的保证。同一 PG 实例仍共享底层硬件；保持后台限速，大并发压测继续使用本地隔离 PG。

完整证据保存在本次发布目录：`preflight.json`、`reporting-ready.json`、`verification-report.json`、`post-release-api-and-billing-check.json`、`post-restart-observation.json`。父目录本地 `.deployment/active-production-update.json` 和 `.deployment/active-pg-production-update.json` 已指向本次生产发布；原迁库指针在当前留档中保留。

## 本次回退方式

已备份旧二进制、旧 `.env`、源码和前端；另有生产一致性 dump 留档，**普通代码回退不恢复这个 dump**。

```bash
python3 /data/app/new-api-ty/update_20261113_171503/server_ds_report_release.py \
  /data/app/new-api-ty/update_20261113_171503 rollback
```

此回退先停后台，再恢复发布前的 PG 网关程序、配置、源码和前端，继续连接当前 `new_api_ty`。**不切换 SQLite、不恢复旧数据库、不清空或覆盖上线期间的新账单／余额／配置。** 之前迁库时的 SQLite 回退脚本不适用于本次报表发布。新建报表库及只读角色可保留待排查，不因代码回退删除数据。

本次发布没有触发业务回退；首次账单补算由后台继续运行。
