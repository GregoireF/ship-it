### Added

- `ship watch`: a checkpoint after each pause in your edits, until Ctrl+C, and none while nothing changes; `ship rewind` goes back to one. `ship gc` keeps them apart from the checkpoints of operations, 48 hours and 200 at most (`ship.watch.pauseSeconds`, `ship.watch.keepHours`, `ship.watch.keepMax`) (#38).
