## 更新日志

本项目的所有显著变更都将记录在此文档中。

变更记录的格式遵循 [保持变更日志](https://keepachangelog.com/zh-CN/1.0.0/) 约定，
同时本项目遵守 [语义化版本控制](https://semver.org/lang/zh-CN/spec/v2.0.0.html) 规范。

### 变更类型

- `新增`：引入新功能。
- `修改`：对现有功能的改进或调整。
- `废弃`：计划在未来移除的功能。
- `移除`：已从项目中移除的功能。
- `修复`：错误或漏洞的修复。
- `安全`：涉及安全漏洞的修复或更新。
- `重大变更`：引入了不兼容的更改，可能需要用户做出相应调整。

### [v0.11.0.dev240630] - 2024-06-30

#### 新增

- **[功能]** 支持 odps 数据源读写
- **[功能]** 支持 DataProxy-Java 部署
- **[功能]** Kuscia-Seving 支持设置 Service 名称
- **[功能]** 通过 Proot 实现 RunP 运行时，解耦 App 镜像与 Kuscia 镜像 支持 App 镜像动态更新

#### 修改

- **[依赖]** 升级 golang 版本至 1.22
- **[文档]** 补充了网关 Path Rewrite 说明
- **[文档]** 更新优化了 KusciaAPI 文档描述
- **[文档]** 更新了 Kuscia 监控端口的暴露文档说明
- **[文档]** 更新了内置 Domain 的说明
- **[文档]** 更新了常见问题的文档路径
- **[功能]** KusciaAPI 创建任务支持参与方不包含自己的任务
- **[功能]** 权限前置检查
- **[功能]** 增强 KusciaDeployment 控制器在节点重启时的处理逻辑

#### 重大变更

- **[无]**

#### 修复

- **[功能]** 修复 P2P 场景下 Job 校验失败或创建 Task 失败时，状态不同步问题
- **[功能]** 修复了挂载宿主机数据路径可能因为权限导致的问题

#### 安全

- **[漏洞]** 修复了部分安全漏洞

---

## Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

### Types of changes

- `Added` for new features.
- `Changed` for changes in existing functionality.
- `Deprecated` for soon-to-be removed features.
- `Removed` for now removed features.
- `Fixed` for any bug fixes.
- `Security` in case of vulnerabilities.
- `Breaking Changed` Breaking for backward-incompatible changes that require user intervention.

### [v0.11.0.dev240630] - 2024-06-30

#### Added

- [Feature] Support for odps data source read and write
- [Feature] Support DataProxy-Java deployment
- [Feature] Kuscia-Serving supports setting service name
- [Feature] Decoupling of app image and Kuscia image support for dynamic updating of app image through Proot implementation of RunP runtime

#### Changed

- [Dependency] Upgrade golang version to 1.22
- [Documentation] Added Gateway Path Rewrite explanation
- [Documentation] Updated and optimized KusciaAPI documentation description
- [Documentation] Updated documentation on exposing Kuscia monitoring ports
- [Documentation] Updated description for built-in domain
- [Documentation] Updated documentation path for common issues
- [Feature] KusciaAPI task creation supports participants not including oneself
- [Feature] Pre-check of permissions
- [Feature] Enhanced KusciaDeployment controller handling logic on node restart

#### Breaking Changed

- [NA]

#### Fixed

- [Feature] Fixed synchronization issue when job verification fails or task creation fails in P2P scenario
- [Feature] Fixed issue with host data path mounting due to permission

#### Security

- [Vulnerability] Fixed some security vulnerabilities
