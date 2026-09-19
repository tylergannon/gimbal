Feature: The Gimble application works through a browser

  These scenarios run against its development server and standalone production
  binary.

  Scenario: Project runs can be filtered and opened
    Given I open the project runs
    Then recorded runs show identity, status and time
    When I filter runs by the first recorded status
    Then only runs with that status remain
    When I open the first filtered run
    Then the run workspace shows its identity and observation
    When I select recorded work in the workspace
    Then the detail pane describes that selected work

  Scenario: Navigation and a direct deep link both reach About
    Given I open the project runs
    When I follow the About link
    Then About is visible without a document reload
    When I load the About route directly
    Then About is visible in a new document

  Scenario: Delayed Runs navigation shows its destination before populated data arrives
    Given I open About with recorded runs available
    When I follow Runs while its data is delayed
    Then accessible Runs loading cards remain until data arrives
    When the delayed Runs data arrives
    Then recorded cards replace the loading feedback

  Scenario: Delayed Runs navigation resolves to the existing empty state
    Given I open About with an empty project
    When I follow Runs while its data is delayed
    Then accessible Runs loading cards remain until data arrives
    When the delayed Runs data arrives
    Then the empty project replaces the loading feedback

  Scenario: Background refresh preserves usable Runs cards
    Given I open the controlled project runs
    When a background Runs refresh is delayed
    Then the current Runs card stays visible and usable
    When the background Runs refresh arrives with updated data
    Then the card updates without navigation loading feedback

  Scenario: A long failed summary stays inside its Runs card
    Given I open the controlled project runs
    Then the failed Runs card is contained with a reachable action at desktop and phone widths
