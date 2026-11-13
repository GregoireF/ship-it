### Added

- One commit per intent: when the work does several things (a rename, formatting, a feature with its tests, the rest), the line-by-line review (`--plain`) proposes one commit for each, a file split hunk by hunk when needed. The last commit leaves the files exactly as they are, `ship undo` takes them all back, and work changed since the review is refused (`SHIP-E0202`) (#34).
