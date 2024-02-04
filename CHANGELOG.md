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

## [0.6.0.dev240131] - 2023-01-31
### Added
- Upgrade interconnection protocol from kuscia-alpha to kuscia-beta to support interconnection between Kuscia-Master and Kuscia-Autonomy.
- Kuscia monitor, Kuscia exposes a set of metric data, which can be used as data sources for collection by external monitoring tools (such as Prometheus).
- The Kuscia API  added a  job approve interface ，allowing participants to review jobs .
- Add some pre-check before kuscia running, such as health check of the connection of database.
- Add parameter validation to the kuscia api.
- The create job interface of kuscia API added the attribute 'customed-fields' .
- Support configuring the application's image ID in AppImage to prevent domain's application image from being tampered with.
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
