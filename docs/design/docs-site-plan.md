# The docs site plan

Agreed with Tyler on 2026-09-19. Strategy only: no copy, colors, or type
here. Those come from later research.

## The problem

The site lists mechanics and gives no impression of the product. The live
console, the thing most agent libraries do not have, appears nowhere. There
are three pages, no path inward, and nothing keeps the pages true as the code
changes.

## The poster

A visitor gives a project five seconds. The top of the landing page is a
poster: the name, a category line, a moving capture of the console, four or
five features as short noun phrases, and the install command.

The poster is one deliverable on three surfaces:

| Surface | Mechanism |
| --- | --- |
| Link previews (Slack, Discord, X, LinkedIn, iMessage, Bluesky) | OpenGraph tags. The image is what matters: a static 1200x630 PNG, a still of the console with the name and category line. Every feature page gets its own card. Today the site has no `og:image` and no `twitter:card`. |
| GitHub | The top of the README, which can animate, and the repository's social preview image, uploaded by hand in repository settings. |
| Hacker News | No preview at all: a title and a domain. The title and the first screen after the click do the work. |

## Three depths

- **Five seconds.** The poster, above the fold.
- **Thirty seconds.** Scrolling gives each feature a band: its visual, one
  sentence, a link.
- **Five minutes.** One page per feature. It opens with the visual, then what
  it is, how to use it, and where the reference lives. A reader arrives by
  interest, not in sequence. The quickstart is the one ordered page.

Why workflows are ordinary Go, and what Gimbal refuses to do, lives on the
About page for readers who stay.

## Visuals

Each feature has one signature visual, used in its landing tile, its page,
its link card, and the README. The console capture is the hero. Visuals show
the real product or real code; one concept diagram per hard idea, all in one
style. The mascot moves to the 404 page, the footer, and the favicon.

## Voice

Peer to peer. Claims are specific and checkable. Tradeoffs and limits are
shown. Short sentences, real code, real output.

## Staying true

Every fact has one home, and the site generates from it or links to it.

- **The feature list is one data file.** It drives the landing tiles, the
  nav, the feature page stubs, and the README block.
- **Generated pages are never hand-edited.** The role reference already works
  this way through `rolerefgen`. CLI flags, the workflow list, and the API
  reference follow the same pattern. The site links to Godoc and never
  restates it.
- **Code samples are compiled code**, pulled from `example_test.go` and the
  built-in workflows. A breaking API change fails the docs build.
- **Visuals are Playwright captures from a fixture run**, including the hero
  and the link cards. A UI change refreshes them in the same PR. A feature
  does not reach the site without its visual.
- **Hand-written pages are few, and the definition of done covers them.** A
  change in behavior updates its page in the same PR. CI checks for stale
  generated files.
- **A docs agent catches what slips through.** A scheduled Gimbal workflow
  reads merged commits, finds pages that drifted, and proposes fixes.

## Before posting anywhere

- **Every phrase on the poster has a receipt one click away.** Write down the
  claims, mark each provable now or not, and close the gaps or drop the claim.
- **Choose the domain.** Moving after launch strands every shared link and
  cached card.
- **Quiet test.** Share the link in a few real channels and check that every
  card renders.

## Order

1. Choose the features and the signature visual for each.
2. Build the capture plumbing.
3. Build the poster: landing page, README top, link cards.
4. Write the feature pages, the console first.
5. Generated reference, the docs agent, About.
6. Visual identity research, applied over a structure that is already right.
7. The pre-launch checks above, then post.

## Done

Someone who sees the top of the page for five seconds can say what Gimbal is
and name two things in the box. A merged API change cannot leave the site
wrong without CI or the docs agent noticing.
