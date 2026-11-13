### Added

- `ship.agents`: rules per coding agent working through `ship mcp`, and `"*"` for every agent. `tools` lists the `ship_*` tools it may call (the others are hidden and refused, `SHIP-E0802`); `protectedPaths` lists files it may not deliver (`SHIP-E0803`, a human delivers them). An organisation policy can enforce them (#48).
