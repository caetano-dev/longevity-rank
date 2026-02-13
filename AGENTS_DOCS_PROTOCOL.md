# AGENT DIRECTIVE: DOCUMENTATION SYNC PROTOCOL

**CRITICAL INSTRUCTION:** Code changes are fundamentally incomplete until the documentation reflects the new system state. You must execute this protocol immediately after writing or modifying any code.

## 1. Documentation Targets
You must evaluate and update the following files if your code alters their described behavior:
* `SPEC.md`: Update if you modify data models (e.g., `types.go`), architectural flow, or integration points.
* `README.md`: Update if you change CLI commands, environment variables, setup instructions, or resolve items on the `TODO` list.
* `data/vendor_rules.json`: Ensure new vendor configurations or global rule schemas are documented if the parser structure changes.

## 2. Execution Steps
1.  **Analyze the Delta:** Identify exactly what your code changed (e.g., added a new regex, fixed a pagination bug, changed a struct).
2.  **Scan Docs:** Grep `SPEC.md` and `README.md` for outdated references to the old logic.
3.  **Purge Stale Logic:** Delete descriptions of bugs you just fixed or workarounds that are no longer necessary. 
4.  **Update TODOs:** If your code fulfills a requirement listed under "Missing Data", "TODOs", or "Known Bugs", you must physically remove that item from the list.
5.  **Inject Truth:** Write concise, technically accurate descriptions of the new state. Do not describe what the code *will* do; describe what it *does*.

## 3. Strict Constraints
* **No Drift:** Do not allow the `Product` or `Analysis` struct definitions in `SPEC.md` to fall out of sync with `internal/models/types.go`.
* **No Hallucinations:** Never document a feature you intend to build next. Only document code that is actively committed and functioning.
* **No Fluff:** Write documentation updates in blunt, direct technical language. Skip marketing copy or filler words.