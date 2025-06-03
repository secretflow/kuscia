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

### [v0.14.0.dev250218] - 2025-02-18

#### 新增

- **[功能]** 引擎等应用日志（含dataproxy等），可以定期自动回收
- **[功能]** kuscia image命令重构，并新增删除镜像功能
- **[功能]** kuscia containerd通过代理可以访问远程仓库，拉取远程仓库镜像

#### 修改

- **[优化]** 废弃创建domain时填写auth_center。默认使用RSA方式创建Token
- **[优化]** envoy日志轮转优化
- **[优化]** makefile结构优化
- **[优化]** diagnose工具优化，去除对kusciajob和镜像的依赖

#### 重大变更

- **[无]**

#### 修复

- **[问题修复]** 修正runk模式下，提交pod到backend k8s的某些失败情况下无法正确终止
- **[问题修复]** 修复diagnose启动配置，可使用http访问kuscia api

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

### [v0.14.0.dev250218] - 2025-02-18

#### Added

- **[Feature]** Engine and other application logs (secretflow/dataproxy, etc.) can now be automatically recycled on a regular basis.
- **[Feature]** Refactored kuscia image command and added a new feature to delete images.
- **[Feature]** Kuscia containerd can access remote repositories via a proxy to pull images from remote repositories.

#### Changed

- **[Optimization]** Deprecated the option to specify auth_center when creating a domain. Tokens are now created using RSA by default.
- **[Optimization]** Improved log rotation for Envoy.
- **[Optimization]** Optimized the structure of the Makefile.
- **[Optimization]** Enhanced the diagnose tool by removing dependencies on kusciajob and images.

#### Breaking Changed

- **[NA]**

#### Fixes

- **[Bugfix]** Resolved an issue where pods submitted to the backend Kubernetes in runk mode could not terminate properly under certain failure scenarios.
- **[Bugfix]** Fixed the configuration for starting diagnose, allowing Kuscia APIs to be accessed via HTTP.
