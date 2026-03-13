## ADDED Requirements

### Requirement: EmptyDir tmp volume has sizeLimit
The generated Deployment manifest SHALL define the `tmp` volume with `emptyDir.sizeLimit` set to `512Mi`.

#### Scenario: Generated Deployment contains sizeLimit on tmp volume
- **WHEN** `GenerateK8sManifests` is called for any workflow
- **THEN** the Deployment manifest content SHALL contain `sizeLimit: 512Mi` under the tmp emptyDir volume definition

#### Scenario: No unbounded emptyDir
- **WHEN** `GenerateK8sManifests` is called for any workflow
- **THEN** the Deployment manifest SHALL NOT contain `emptyDir: {}` (the old unbounded format)
