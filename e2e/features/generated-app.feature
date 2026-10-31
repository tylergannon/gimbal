Feature: The Gimbal application works through a browser

  These scenarios run against its development server and standalone production
  binary.

  Scenario: Live run inventory is discovered, reloaded, updated and opened through its graph
    Given I open the controlled project home
    Then the project home offers an empty activity list
    When I open All runs
    Then the project offers an empty Runs view
    When pending and completed run records become available
    Then the live inventory shows the pending question and completed run
    When the foreign pending run completes
    Then its live inventory row completes without stale running state
    When I return to project home
    When a recorded run becomes available
    Then the project home discovers the recorded run
    When I reload the project home
    Then the project home discovers the recorded run
    When I open and reload All runs
    Then the recorded run shows its identity, failure and cost
    Then the long failed summary keeps its run reachable at desktop and phone widths
    When the recorded run summary changes
    Then the live Runs list updates without replacing the focused row
    When I open the recorded fixture run
    Then the recorded transcript is visible
    When I open its workflow map and await the graph API
    Then the graph shows the recorded scope
    When I select that recorded scope
    Then the detail pane describes that scope
    When I return to the transcript and reopen the map
    Then the same graph and selection are retained without a document reload
