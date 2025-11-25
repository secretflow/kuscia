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

### [v1.0.0.dev250825] - 2025-08-25

#### 新增

- **[功能]** 新增 Delete DomainData 物理文件接口 （由社区贡献者 @peter5232 贡献 ）
- **[功能]** Hive 数据源支持（alpha版本 由社区贡献者 @peter5232 贡献 ）
- **[功能]** Kuscia Task 资源及连通性前置检查（由社区贡献者 @MiKKiYang 贡献 ）
- **[功能]** Kuscia images import 支持校验镜像架构是否匹配当前 Kuscia架构（alpha版本 由社区贡献者 @exyb 贡献）
- **[功能]** 支持通过 envoy 日志对 Kuscia task 进行异常分析
- **[功能]** KusciaDeployment 默认加上反亲和性（PodAntiAffinity）配置，多副本时，尽量部署到不同节点
- **[功能]** Kuscia DomainData 支持 PostgreSQL 代理数据源连接参数配置

#### 修改

- **[优化]** Kuscia PostgreSQL 数据源数据代理写入优化
- **[优化]** runp 运行时 kill pod， agent 发送 sigterm 信号量
- **[优化]** 完善 Kuscia 文档

#### 重大变更

- **[无]**

#### 修复

- **[问题修复]** Kuscia 使用 PostgreSQL 作为元数据存储时连接异常问题

**注**：

- Alpha功能还在完善中，未经质量测试，不适合在实际的生产环境中使用。

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

### [v1.0.0.dev250825] - 2025-08-25

#### Added

- **[Feature]** Added Delete DomainData physical file interface (contributed by community contributors @peter5232).
- **[Feature]** Hive data source support (alpha version contributed by community contributors @peter5232).
- **[Feature]** Kuscia Task Resource and Connectivity Pre-Check (contributed by community contributors @MiKKiYang).
- **[Feature]** Kuscia images import support to validate image architecture matching the current Kuscia architecture (alpha version contributed by community contributors @exyb).
- **[Feature]** Support for analyzing Kuscia task exceptions through envoy logs.
- **[Feature]** Added PodAntiAffinity configuration to KusciaDeployment, with a preference to deploy across different nodes when multiple replicas are present.
- **[Feature]** Kuscia DomainData supports PostgreSQL proxy data source connection parameter configuration.

#### Changed

- **[Optimization]** Kuscia PostgreSQL data source write optimization.
- **[Optimization]** When running in runp mode, the agent sends a sigterm signal to kill the pod.
- **[Optimization]** Improved Kuscia documentation.

#### Breaking Changed

- **[NA]**

#### Fixes

- **[Bugfix]** Kuscia connection exception when using PostgreSQL as metadata storage

**Note**

- Alpha features are still under development and have not been quality tested, so they are not suitable for use in actual production environments.
