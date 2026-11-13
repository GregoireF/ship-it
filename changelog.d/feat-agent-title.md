### Added

- Coding agents name their own work: `ship_plan` returns the repository's `conventions` (title and branch formats, commit types, scopes, limits), and the skill tells the agent to always propose its title and branch. A refused proposal lists every rule it breaks and proposes a title and a branch that pass, the agent's wording kept when a correction cannot change its meaning (#33).
