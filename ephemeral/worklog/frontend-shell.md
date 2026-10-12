# Frontend shell

correction: Menubar means the macOS system menubar outside the application window. This slice supplies an in-window tab strip only; native application menus and traffic-light/titlebar integration remain deferred.
decision: Keep the shell additive under web/src/lib/shell. Parents own project inventory, open views, active identity, work and recording; hidden keyed panels retain supplied local content while switching views. Production remotes and native bridges remain outside this slice.
friction: Storybook's standalone iframe did not apply the theme global URL parameter. Use the existing themes decorator's per-story themeOverride for a deterministic dark specimen; verify its computed background before labeling screenshots dark.
