# Environment UI decisions

- Session setup keeps branch/base/isolation/model choices per environment inside the retained NewSession component. Selecting an environment only reads setup; the first-message form remains the admission boundary.
- Machines is a web pane in native Settings alongside Recording. Same-origin native handlers open Settings, SSH Terminal connections and the owning session in the main workspace; address and route entries give explicit terminal instructions.
- Configuration editing captures its original byte revision. A live refresh cannot advance that revision under unsaved text. Explicit reload uses a fresh remote form instance, because SvelteKit caches keyed form fields across a same-key remount.
- Opaque configuration fields stay in the Go store. The frontend parses only the display fields needed for connection details, while the configuration editor edits the actual active file.
- Machine removal names retained sessions and waits for their explicitly selected, confirmed cleanup. A rejected cleanup leaves the removal disabled and its error visible.
- Generator friction: commands require RequestEvent even when they have no route identity; duplicate Harness names require consumers to use Setup's indexed harness type rather than an unstable generated collision alias.
- Native validation exposed an integration boundary missed by component mocks: instance project routing rejected the new global environment remotes from Settings because their module was absent from the shared-remotes list. The same URLs worked with an admitted project Referer. Global machine operations and the explicit-project cleanup command now pass that gate; project-sensitive controls keep their existing gate. The HTTP regression covers both Settings routes and a native caller without Referer.
- Live-query errors may be serialized App.Error objects rather than Error instances. Render their message explicitly instead of coercing the object to text.
