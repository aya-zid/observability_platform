# Agent

This folder contains the install/uninstall scripts for the host agent.

## What this agent does

The agent is installed on a client host machine and runs as a local service.

It collects:

- Metrics from system/services automatically
- Logs from common service log paths automatically


## Scripts

- `install.sh`: installs and configures the agent service
- `uninstall.sh`: removes the agent service and related files

## Notes

- Metrics and logs are mostly auto-collected after install.
- Traces require app instrumentation/export config (minimal app changes).
