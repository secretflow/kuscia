# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Types of changes

`Added ` for new features.
`Changed` for changes in existing functionality.
`Deprecated` for soon-to-be removed features.
`Removed` for now removed features.
`Fixed` for any bug fixes.
`Security` in case of vulnerabilities.
`Breaking Changed` Breaking for backward-incompatible changes that require user intervention.

## [v0.8.0.dev240331] - 2024-03-31

### Added

- [Alpha] Kuscia Job now has new interface capabilities for retrying, pausing, and canceling jobs.
- [Alpha] One-way Networking: Support for collaboration where only one party needs to expose a public port, eliminating
  the need for the other party to also do so. Documentation to follow.
- [Documentation] How to use the Kuscia API to run a SecretFlow Serving.
- [Documentation] Introduction to Kuscia listening ports.
- [Documentation] Added documentation explaining DataMesh interface.
- [Scripts] Added kuscia.sh and init_example_data.sh scripts.

### Changed

- Modified the default task state in the job query results from Kuscia API.
- Optimized processing flows for KusciaJob and KusciaTask.
- Added isTLS field to the kuscia API for creating DomainRoute interface.
- Kuscia API has added a Suspended state enumeration to kusciaJob.
- Revised container launch deployment steps in point-to-point and centralized deployment documentation.
- Modified the pre-validation logic of the master address when initiating lite mode.

### Breaking Changes

- Deprecated:start_standalone.sh,deploy.sh. Recommended use kuscia.sh

### Fixed

- Fixed an occasional concurrent read-write map issue causing panics in the DomainRoute module.
- Fixed an issue with image repository names in the Agent module.
- Strengthened integration tests for improved stability.
- Fixed an issue where changes to the Configmap in Runk mode did not take effect for Serving.
- Fixed an error when creating DomainDataSource with Kusica API.
- Fixed a potential issue with abnormal startup when the protocol is set to TLS in the Kuscia configuration file.

## [0.7.0.dev240229] - 2024-02-29

### Added

- add the documents of datasource api
- add kusciaapi errorcode doc
- add kuscia init command generating kuscia config

### Changed

- update domain register and handshake error
- report events when Pod failed to start in RunK mode

### Breaking Changed

- Change the mounting directory for logs and data of Kuscia deploying with Docker
    - {{ROOT}}/kuscia-{{DEPLOY_MODE}}-{{DOMAIN_ID}}-data -> {{ROOT}}/{{DOMAIN_CONTAINER_NAME}}/data.
    - {{ROOT}}/kuscia-{{DEPLOY_MODE}}-{{DOMAIN_ID}}-logs -> {{ROOT}}/{{DOMAIN_CONTAINER_NAME}}/logs.
    - {{ROOT}}/kuscia-{{DEPLOY_MODE}}-{{DOMAIN_ID}}-certs was deleted.

### Fixed

- fix some unit test case
- fix the issue of inconsistent states among multiple parties in KusciaDeployment
- fix the issue of ClusterDefine only having unilateral information under KusciaDeployment in P2P scenarios
- fix kusciaapi grpc role check
- Upgrade certain dependency packages to fix security vulnerabilities in pkg.

## [0.6.0.dev240131] - 2024-01-31

### Added

- Upgrade interconnection protocol from kuscia-alpha to kuscia-beta to support interconnection between Kuscia-Master and
  Kuscia-Autonomy.
- Kuscia monitor, Kuscia exposes a set of metric data, which can be used as data sources for collection by external
  monitoring tools (such as Prometheus).
- The Kuscia API added a job approve interface ，allowing participants to review jobs .
- Add some pre-check before kuscia running, such as health check of the connection of database.
- Add parameter validation to the kuscia api.
- The create job interface of kuscia API added the attribute 'customed-fields' .
- Support configuring the application's image ID in AppImage to prevent domain's application image from being tampered
  with.
- Added the curl command example for requesting the kuscia API.
- polish the agent runtime docs.

### Changed

- Changed some kuscia-crds （KusciaJob，KusciaTask，KusciaDeployment）from cluster to namespace (cross-domain).

### Fixed

- Correct some inaccurate descriptions in the document.

## [0.6.0.dev240115] - 2024-01-15

### Added

- Add network error troubleshooting document.
- Add steps for pre creating data tables in the process of deploying kusica on K8s.

### Changed

- The token from lite to master supports rotation.

### Fixed

- When deploying using deploy.sh, no kuscia API client certificate was generated.

## [0.5.0b0] - 2024-1-8

### Added

- Support deploying kuscia on K8s.
- Support running algorithm images based on runp and runk modes.
- Support configuring Path prefix in domain public URL addresses.

### Changed

- Optimize deployment configuration and add configuration documentation.
- Optimize error information of task and error logs of kuscia.

### Fixed

- When there is a duplicate node error, the node will not exit but will try again.
- Change ClusterDomainRoute status to be unready when dest domain is unreachable.

## [0.5.0.dev231225] - 2023-12-25

### Added

- Add document of Kuscia overview.

### Changed

- Move pod scheduling phase to the task pending phase.

## [0.5.0.dev231215] - 2023-12-15

### Added

- Add document for deploying Kuscia on k8s.

### Changed

- Optimize log output.

## [0.5.0.dev231205] - 2023-12-5

### Changed

- Optimize Kuscia deployment configuration and add configuration documentation.
- Optimize error messages due to scheduling failures.

## [v0.5.0.dev231201] - 2023-12-01

### Fixed

- When there is a duplicate node error, the node will not exit but will try again.
- Change ClusterDomainRoute status to be unready when dest domain is unreachable.

## [v0.5.0.dev231122] - 2023-11-22

### Added

- Support register secretflow psi image.

## [0.4.0b0] - 2023-11-9

### Added

- Add KusciaDeployment operator.
- Support non MTLS network communication in P2P networking mode.

## [0.3.0b0] - 2023-9-7

### Added

- Support the deployment of new lite domain in centralized clusters.
- Support non MTLS network communication in centralized networking mode.
- Supports the deployment of an autonomy domain across machines.
- Add Integration Test.

## [0.2.0b2] - 2023-7-18

### Fixed

- Correct datamesh service name for p2p

## [0.2.0b1] - 2023-7-7

### Fixed

- Fix document typo.
- Fix errors when installing secretpad using non-root user.
- Fix the issue of token failure after restarting Kuscia.

## [0.2.0b0] - 2023-7-6

### Added

- Kuscia init release.
