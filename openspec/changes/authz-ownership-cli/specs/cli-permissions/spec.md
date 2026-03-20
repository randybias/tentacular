## ADDED Requirements

### Requirement: permissions get command displays tentacle permissions
The CLI SHALL provide `tntc permissions get <name>` that calls permissions_get and displays owner, group, and mode.

#### Scenario: Get permissions for a tentacle
- **WHEN** the user runs `tntc permissions get my-tentacle`
- **THEN** the CLI SHALL display owner (sub, email, name), group, and mode in a readable format

#### Scenario: Get permissions with JSON output
- **WHEN** the user runs `tntc permissions get my-tentacle --json`
- **THEN** the CLI SHALL output the permissions as JSON

### Requirement: permissions set command modifies tentacle permissions
The CLI SHALL provide `tntc permissions set <name>` with --mode, --group, and --owner flags.

#### Scenario: Set mode
- **WHEN** the user runs `tntc permissions set my-tentacle --mode 0750`
- **THEN** the CLI SHALL call permissions_set with the new mode

### Requirement: chmod convenience command
The CLI SHALL provide `tntc permissions chmod <mode> <name>` as shorthand for `permissions set --mode`.

#### Scenario: Chmod a tentacle
- **WHEN** the user runs `tntc permissions chmod 0755 my-tentacle`
- **THEN** the CLI SHALL call permissions_set with mode=0755

### Requirement: chgrp convenience command
The CLI SHALL provide `tntc permissions chgrp <group> <name>` as shorthand for `permissions set --group`.

#### Scenario: Chgrp a tentacle
- **WHEN** the user runs `tntc permissions chgrp platform-team my-tentacle`
- **THEN** the CLI SHALL call permissions_set with group=platform-team
