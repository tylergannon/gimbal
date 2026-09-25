# Adjacent interface references

Research scope: three deliberately different interfaces that help with run discovery, history, spatial structure, or attention without suggesting that Gimbal should become a workflow authoring tool or a generic analytics dashboard. Image URLs below are public, directly embeddable references; they are not downloaded into the repository.

## 1. Chrome DevTools Performance panel — temporal trace with spatial nesting

Source: [Chrome Performance features reference](https://developer.chrome.com/docs/devtools/performance/reference)

Image: [official flame-chart image](https://developer.chrome.com/static/docs/devtools/performance/reference/image/flame-chart.png)

Observed visual facts (from the linked official image): a dense horizontal trace is divided into stacked rectangular bars; nesting is expressed by vertical depth, and labels sit inside bars. The reference image uses distinct but restrained colors for script/activity categories and a blue selection outline. The page also exposes the related [timeline overview image](https://developer.chrome.com/static/docs/devtools/performance/reference/image/timeline-overview.png), which places compact overview tracks above the detailed trace.

Documented behavior: Chrome says the x-axis is recording time and y-axis is call-stack depth; events above cause events below. The timeline can select a range, zoom into that range, and retain nested zoom breadcrumbs. Long traces can be scrolled, searched, filtered, and reduced through hidden functions/tracks. Call Tree, Bottom-up, and Event Log provide alternate table views while keeping corresponding trace events highlighted. (See source, sections “Navigate the recording,” “Read the flame chart,” and “View activities in a table.”)

Inference / possible borrow: Gimbal could use a compact overview band plus a detail lane to preserve position while inspecting a long run. A selected scope/turn could highlight its corresponding history rows, while breadcrumbs or “back to overview” preserve orientation. The useful idea is temporal focus and linked selection, not performance metrics or a playback scrubber. For repeated tasks, nested bars can show actual instances while the graph remains a separate declared structure.

What not to borrow: a trace is not a workflow editor. Do not add arbitrary drag-to-reorder, editable call stacks, or invented timing/causal arrows. A run history is not a video timeline; reviewing history does not imply replay.

## 2. Blender Node Editor — zoomable spatial canvas

Source: [Blender Manual, Node Editor introduction](https://docs.blender.org/manual/en/4.4/interface/controls/nodes/introduction.html) and [Geometry Node Editor](https://docs.blender.org/manual/en/latest/editors/geometry_node.html)

Image: [official manual node-editor example](https://docs.blender.org/manual/en/latest/_images/interface_controls_nodes_introduction_example.jpg)

Observed visual facts (parent directly inspected the linked image): rectangular nodes with colored headers and sockets sit over an image backdrop; curved links connect a left-to-right network. Node bodies contain detailed controls. This is a spatial workspace rather than a time axis. The busy image background is not a proposed Gimbal treatment.

Documented behavior: Blender describes node editors as editors for node-based workflows; the header exposes View, Select, Add, and Node menus, and the sidebar exposes selected-node properties. Geometry Node Editor documentation describes navigating/editing node groups and switching node-tree context. Blender’s node system supports entering/leaving parent node trees and reroute nodes for organization.

Inference / possible borrow: Gimbal’s generated workflow graph can borrow the canvas’s “overview first, inspect a selected region second” rhythm: pan/zoom, select a group or branch, and show details alongside it. Directional links and visible containment can clarify parallel siblings, loops, and supervisor relationships when color alone is insufficient. A minimap or “fit graph” action would serve the same orientation need.

What not to borrow: this is a creative authoring canvas. Gimbal must not expose Add Node, wiring, rearranging, editing, or pretend every runtime instance is a graph node. Keep workflow definition read-only and explicitly separate from actual run instances; an unobserved declared operation is not “queued” or “skipped.”

## 3. Sentry Issue Stream — attention queue with chronological detail

Source: [Sentry Issue Stream UI enhancements](https://sentry.io/changelog/issue-stream-ui-enhancements/) and [Issue status / triage](https://docs.sentry.io/product/issues/states-triage/)

Image: [official Sentry issue-stream screenshot](https://cslswue7zohm4cat.public.blob.vercel-storage.com/tL4PFCI-image.png)

Additional official image: [Sentry escalating-issues issue stream](https://images.ctfassets.net/em6l9zw4tzag/32SxXhH2llNK4tNGCBS60A/72716d627a486d14d1aa1caa55d282bf/image_4.png)

Observed visual facts (parent directly inspected the linked screenshot): dense rows pair a bold issue label and contextual subline with Last Seen, Age, Trend, Events, Users, Priority, and Assignee columns. Small purple dots precede rows. Tabs are not visible in this crop. Its emphasis is scanability rather than a spatial graph.

Documented behavior: Sentry says rows are clickable across the row, “First Seen” and “Age” are separate columns, and unread indicators identify viewed state. Its triage documentation defines status transitions and a “For Review” subset; escalating issues return to the top when event volume rises. Issue Details then provides an event graph, searchable event range, a chronological activity section, and breadcrumbs leading up to the event.

Inference / possible borrow: Gimbal could have a lightweight attention strip or run-list facet for “needs answer,” active, failed, and disconnected—then let a person open the exact interview or turn. The important pattern is attention as a derived queue with explicit status and unread/viewed state, not a dashboard of every metric. A chronological activity/history pane can preserve context while the queue remains stable.

What not to borrow: Gimbal is local run observation, not an incident-management product. Do not add assignees, escalation policies, issue grouping, alerting, cross-project analytics, or auto-prioritization claims. “Needs answer” should remain visibly tied to a pending interview and its workflow location; it is not a new terminal run status.

## Cross-reference and limitations

These references support three distinct directions: (1) temporal trace + linked detail (Chrome), (2) spatial definition map + drilldown (Blender), and (3) attention queue + chronological context (Sentry). They should be composed around Gimbal's four requested capabilities, not merged into one universal dashboard. The researcher initially had no browser rendering; the parent subsequently opened and visually inspected the primary Chrome, Blender, and Sentry image assets in Chrome. They are published screenshots, not hands-on product tests. Supplemental images were not independently inspected; no images were fabricated or downloaded.
