# Embedded instruction library

The binary embeds 32 task overviews and 85 detailed adapted references from Piebald revision `9b3512f`. Search returns metadata only; reading an ID loads one module without network access. Overview modules link to related detailed IDs.

`source-selection.json` records all 870 source files, including selection decisions and exclusion reasons. A selected source always has a readable `source-*` module. Host schemas, proprietary publishing, unsupported scheduling and unresolved runtime expressions are excluded. Portable principles from some excluded sources are retained in the task overviews.

The upstream MIT license is preserved in `LICENSE`. Source URLs are stored in catalog metadata. Updates ship with the binary through the normal update path.
