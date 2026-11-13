# PostgreSQL 独立部署与后续迁移准备

确认及部署日期：2026-11-12。

现网已于 **2026-11-12 切换至 PostgreSQL 17.11**，端口仍为 53000。详见 [生产切换记录](production-cutover-20261112.md)。发布前的 [本地验证记录](local-rehearsal.md) 和 [脱敏测试结果](local-validation-20261112.json) 保留供复核；下文是最初独立安装阶段的历史记录。

2026-11-13 已另建独立测试业务库／报表库并连接本地网关，详见 [远程测试副本与本地连接](remote-test-copy-20261113.md)。这次没有更新或重启生产网关。

## 本次范围与结果

- 按用户要求，在 192.168.47.33 安装独立 PostgreSQL，当前版本 `17.11 (Ubuntu 17.11-1.pgdg22.04+2)`。
- 使用 PGDG 官方 Ubuntu 软件源（jammy-pgdg），验证签名密钥指纹 `B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8`；保留 Ubuntu 原软件源和 CUDA 配置，未执行系统整体升级或重启其他服务。
- 仅安装数据库，不迁移 SQLite，不修改网关的 `.env`、`SQL_DSN`、程序或数据库内容。网关 PID 94510 保持不变，53000 健康检查通过，运行进程的 `SQL_DSN` 仍为空。
- PostgreSQL 的安装本身不会消除当前网关的 SQLite 锁竞争；切库、全量数据迁移、计费对账、并发验证与回滚方案属于下一阶段，尚未执行。

## 部署位置

| 项目 | 配置 |
| --- | --- |
| 服务目录 | `/data/app/postgresql/` |
| 数据目录 | `/data/app/postgresql/data/17/main` |
| 配置目录 | `/etc/postgresql/17/main` |
| 专用配置 | `/etc/postgresql/17/main/conf.d/new-api-ty.conf` |
| 日志目录 | `/data/app/postgresql/logs/` |
| systemd 服务 | `postgresql@17-main`，已启动并启用开机启动 |
| 监听 | `127.0.0.1:5432`、`[::1]:5432`；未开放内网或公网监听 |
| 应用数据库／角色 | `new_api_ty`／`new_api_ty`，空库、非超级用户 |
| 连接凭据 | `/data/app/postgresql/connection.env`，root 所有、0600；密码不写入 Git |
| 部署记录 | `/data/app/postgresql/setup_20261112_103930/` |
| 初始备份 | `backups/initial-empty.dump`、`backups/initial-globals.sql`，仅 root 可读 |

账号使用独立随机密码、SCRAM-SHA-256 验证，不复用网关管理员或 SSH 密码。UTF-8、北京时间、数据校验和、fsync、同步提交及完整页写入已启用。初始资源配置为 shared_buffers 256MB、work_mem 8MB、max_connections 100，未按整台共享服务器的内存规模放大数据库占用。

## 验收

- 实际应用账号通过 TCP 密码认证，中文和精确小数写入／读取一致，测试事务回滚后 public 下应用表数量为 0。
- 错误密码连接失败，应用角色无超级用户、建库、创建角色或复制权限。
- 服务重启后可重新连接，开机启动状态为 enabled；5432 实际仅绑定回环地址。
- 初始逻辑备份可由 pg_restore 读取，运行报告见服务目录 `DEPLOYMENT.json`。
- 网关进程、配置哈希保持不变，未停止网关、未清零或重算任何用户用量和账单。

## 管理命令

```bash
systemctl status postgresql@17-main
systemctl restart postgresql@17-main
systemctl stop postgresql@17-main
systemctl start postgresql@17-main
runuser -u postgres -- psql
```

在完成迁移演练前，不要将连接文件里的 SQL_DSN 写入或导入网关启动环境，以免误切到空库。当前连接文件未放入网关目录。

官方依据：[Ubuntu 安装说明](https://www.postgresql.org/download/linux/ubuntu/)、[版本支持政策](https://www.postgresql.org/support/versioning/)。
