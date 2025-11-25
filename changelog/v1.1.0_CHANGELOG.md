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

### [v1.1.0.dev251125] - 2025-11-25

#### 新增

- **[功能]** 新增带宽资源作为KusciaJob调度限制 （由社区贡献者 @ElectricFish7 贡献 ）
- **[功能]** Datamesh 的 Arrow 读写数据时支持 large_utf8 和时间字段类型（alpha版本）
- **[功能]** KusciaAPI crt 支持在 kuscia.yaml 中配置自定义 sans
- **[功能]** Kuscia 新增 KusciaDomainData GC Controller（默认关闭，可通过 kuscia.yaml 配置开启）
- **[功能]** Kuscia envoy 支持粘性会话（即在负载均衡的场景中，将同一客户端的请求持续路由到同一后端实例）

#### 修改

- **[无]**

#### 不兼容变更

- **[无]**

#### 修复

- **[问题修复]** PostgreSQL 作为任务结果数据源存储时，数据结果中存在 NULL 时报错
- **[问题修复]** KusciaAPI DeleteDomainDataAndRaw 的 GRPC 接口 Response 类型错误修复

#### 安全

- **[漏洞]** 升级 Kuscia 依赖的相关组件（k3s、containerd、nodeExporter 等）解决相关漏洞问题

**Kuscia 依赖组件升级如下**：

- kuscia 的相关 kubernetes 依赖升级到 v1.33.5。
- k3s 升级到 v1.33.5+k3s1
- containerd 升级到 1.7.28
- node_exporter 升级到 1.9.1
- kuscia 的 go 版本升级到 1.24.7
- rootlesskit 升级到 v2.3.5
- containernetworking/plugins 升级到 v1.7.1

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

### [v1.1.0.dev251125] - 2025-11-25

#### Added

- **[Feature]** New bandwidth resources added as scheduling constraints for KusciaJob (contributed by community contributors @ElectricFish7).
- **[Feature]** Datamesh supports the `large_utf8` and `timestamp` field types when reading and writing data via Arrow (alpha version).
- **[Feature]** KusciaAPI crt supports configuring custom sans in kuscia.yaml.
- **[Feature]** Kuscia has added the KusciaDomainData GC Controller (disabled by default; can be enabled via kuscia.yaml configuration).
- **[Feature]** Kuscia Envoy supports sticky sessions (i.e., in load-balancing scenarios, consistently routing requests from the same client to the same backend instance).

#### Changed

- **[NA]**

#### Breaking Changed

- **[NA]**

#### Fixes

- **[Bugfix]** Fixed an issue where NULL values in the data results stored as PostgreSQL datasource would cause errors.
- **[Bugfix]** Fixed a type error in the GRPC interface Response of KusciaAPI DeleteDomainDataAndRaw.

#### Security

- [Vulnerability] Upgrade the components dependent on Kuscia (k3s, containerd, nodeExporter, etc.) to address related vulnerability issues.

**Kuscia Dependent Component Upgrade**：

- Upgraded the related kubernetes dependencies of kuscia to v1.33.5.
- Upgraded k3s to v1.33.5+k3s1.
- Upgraded containerd to 1.7.28.
- Upgraded node_exporter to 1.9.1.
- Upgraded the go version of kuscia to 1.24.7.
- Upgraded rootlesskit to v2.3.5.
- Upgraded containernetworking/plugins to v1.7.1

**Note**

- Alpha features are still under development and have not been quality tested, so they are not suitable for use in actual production environments.
