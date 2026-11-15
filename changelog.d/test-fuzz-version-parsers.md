### Fixed

- The warning about open pull requests that change the same lines keeps every hunk of a file a diff lists twice, and `ship doctor` no longer reads a git version with a negative part; both found by new fuzz tests (#132, thanks @chiragjangiir).
